/*
Tech:Online Backend
Copyright 2020, Kristian Lyngstøl <kly@kly.no>
Copyright 2021-2022, Håvard Ose Nordstrand <hon@hon.one>

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
*/

/*
Package db provides convenience-functions for mapping Go-types to a
database. It does not address a few well-known issues with database
code and is not particularly fast, but is intended to make 90% of the
database-work trivial, while not attempting to solve the last 10% at
all.

The convenience-functions work well if you are not regularly handling
updates to the same content in parallel, and you do not depend on
extreme performance for SELECT. If you are unsure if this is the case,
I'm willing to bet five kilograms of bananas that it's not. You'd know
if it was.

You can always just use the db/DB handle directly, which is provided
intentionally.

db tries to automatically map a type to a row, both on insert/update and
select. It does this using introspection and the official database/sql
package's interfaces for scanning results and packing data types. So if
your data types implement sql.Scanner and sql/driver.Value, you can use
them directly with 0 extra boiler-plate.

To use it, you need a struct datatype with at least some exported
fields that map to a database table. If your field names don't match
the column name, you can tag the struct fields with
`column:"alternatename"`. If you wish to have this package ignore the
field entirely (e.g.: it's exported, but doesn't exist at all in the
database), tag it with `column:"-"`.
*/
package db

import (
	"fmt"
	"reflect"
	"unicode"

	log "github.com/sirupsen/logrus"
)

type keyvals struct {
	keys    []string // name
	keyidx  []int    // mapping from our index to struct-index (in case of skipping)
	values  []interface{}
	newvals []interface{}
}

func enumerate(haystacks map[string]bool, populate bool, d interface{}) (keyvals, error) {
	v := reflect.ValueOf(d)
	v = reflect.Indirect(v)
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	v = reflect.Indirect(v)

	st := v.Type()
	kvs := keyvals{}
	if st.Kind() != reflect.Struct {
		return kvs, newError("Got the wrong data type. Got %s / %T.", st.Kind(), d)
	}

	kvs.keys = make([]string, 0)
	kvs.values = make([]interface{}, 0)

	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)
		value := v.Field(i)
		if !unicode.IsUpper(rune(field.Name[0])) {
			continue
		}
		col := field.Name
		if ncol, ok := field.Tag.Lookup("column"); ok {
			col = ncol
		}
		if haystacks[col] || col == "-" {
			continue
		}

		if field.Type.Kind() == reflect.Ptr && value.IsNil() {
			if !populate {
				continue
			}
			value = reflect.New(field.Type.Elem())
		} else {
			value = reflect.Indirect(value)
		}
		kvs.keys = append(kvs.keys, col)
		kvs.values = append(kvs.values, value.Interface())
		kvs.newvals = append(kvs.newvals, reflect.New(value.Type()).Interface())
		kvs.keyidx = append(kvs.keyidx, i)
	}
	return kvs, nil
}

// Update attempts to update the object in the database, using the provided
// string and matching the haystack with the needle. It skips fields that
// are nil-pointers.
func Update(table string, d interface{}, searcher ...interface{}) Result {
	report := Result{}
	search, err := buildSearch(searcher...)
	if err != nil {
		report.Failed++
		report.Error = err
		return report
	}
	haystacks := make(map[string]bool, 0)
	for _, item := range search {
		haystacks[item.Haystack] = true
	}
	kvs, err := enumerate(haystacks, false, d)
	if err != nil {
		report.Failed++
		report.Error = newErrorWithCause("Update(): enumerate() failed", err)
		return report
	}
	lead := fmt.Sprintf("UPDATE %s SET ", table)
	comma := ""
	last := 0
	for idx := range kvs.keys {
		lead = fmt.Sprintf("%s%s%s = $%d", lead, comma, kvs.keys[idx], idx+1)
		comma = ", "
		last = idx
	}
	strsearch, searcharr := buildWhere(last+1, search)
	lead = fmt.Sprintf("%s%s", lead, strsearch)
	kvs.values = append(kvs.values, searcharr...)
	res, err := DB.Exec(lead, kvs.values...)
	log.WithField("query", lead).Trace("Update()")
	if err != nil {
		report.Failed++
		report.Error = newErrorWithCause("Update(): EXEC failed", err)
		return report
	}
	rowsaf, _ := res.RowsAffected()
	report.Ok++
	report.Affected += int(rowsaf)
	return report
}

// Insert adds the object to the table specified. It only provides the
// non-nil-pointer objects as fields, so it is up to the caller and the
// database schema to enforce default values. It also does not check
// if an object already exists, so it will happily make duplicates -
// your database schema should prevent that, and calling code should
// check if that is not the desired behavior.
func Insert(table string, d interface{}) Result {
	report := Result{}
	haystacks := make(map[string]bool, 0)
	kvs, err := enumerate(haystacks, false, d)
	if err != nil {
		report.Failed++
		report.Error = newErrorWithCause("Insert(): Enumerate failed", err)
		return report
	}
	lead := fmt.Sprintf("INSERT INTO %s (", table)
	middle := ""
	comma := ""
	for idx := range kvs.keys {
		lead = fmt.Sprintf("%s%s%s ", lead, comma, kvs.keys[idx])
		middle = fmt.Sprintf("%s%s$%d ", middle, comma, idx+1)
		comma = ", "
	}
	lead = fmt.Sprintf("%s) VALUES(%s)", lead, middle)
	res, err := DB.Exec(lead, kvs.values...)
	log.WithField("query", lead).Trace("Insert()")
	if err != nil {
		report.Error = newErrorWithCause("Insert(): EXEC failed", err)
		return report
	}
	rowsaf, _ := res.RowsAffected()
	report.Ok++
	report.Affected += int(rowsaf)
	return report
}

// Upsert makes database-people cringe by first checking if an element
// exists, if it does, it is updated. If it doesn't, it is inserted. This
// is NOT a transaction-safe implementation, which means: use at your own
// peril. The biggest risks are:
//
// 1. If the element is created by a third party during Upsert, the update
// will fail because we will issue an INSERT instead of UPDATE. This will
// generate an error, so can be handled by the frontend.
//
// 2. If an element is deleted by a third party during Upsert, Upsert will
// still attempt an UPDATE, which will fail silently (for now). This can be
// handled by a front-end doing a double-check, or by just assuming it
// doesn't happen often enough to be worth fixing.
func Upsert(table string, d interface{}, searcher ...interface{}) Result {
	existsResult := Exists(table, searcher...)
	if existsResult.Error != nil {
		return existsResult
	}
	if existsResult.IsSuccess() {
		return Update(table, d, searcher...)
	}
	return Insert(table, d)
}

// Delete will delete the element, and will also delete duplicates.
func Delete(table string, searcher ...interface{}) Result {
	report := Result{}
	search, err := buildSearch(searcher...)
	if err != nil {
		report.Failed++
		report.Error = err
		return report
	}
	strsearch, searcharr := buildWhere(0, search)
	q := fmt.Sprintf("DELETE FROM %s%s", table, strsearch)
	res, err := DB.Exec(q, searcharr...)
	log.WithField("query", q).Trace("Delete()")
	if err != nil {
		report.Failed++
		report.Error = newErrorWithCause("Delete(): Query failed", err)
		return report
	}
	rowsaf, _ := res.RowsAffected()
	report.Ok++
	report.Affected += int(rowsaf)
	return report
}
