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

package db_test

import (
	"testing"

	"github.com/gathering/tech-online-backend/db"
	"github.com/gathering/tech-online-backend/helper"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetLevel(log.TraceLevel)
}

// system is not completely random. Having Ignored in the middle is
// important to properly test ignored fields and the accounting, which has
// been buggy in the past.
type system struct {
	Sysname   string
	Ip        *string
	Ignored   *string `column:"-"`
	Vlan      *int
	Placement *string
}

func TestSelectMany(t *testing.T) {
	systems := make([]system, 0)
	result := db.SelectMany(1, "1", "things", &systems)
	helper.CheckNotEqual(t, result.Error, nil)
	err := db.Connect()
	helper.CheckEqual(t, err, nil)
	result = db.SelectMany(1, "1", "things", &systems)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckNotEqual(t, len(systems), 0)
	t.Logf("Passed base test, got %d items back", len(systems))

	indirect := make([]*system, 0)
	result = db.SelectMany(1, "1", "things", &indirect)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckNotEqual(t, len(indirect), 0)

	result = db.SelectMany(1, "2", "things", &systems)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, len(systems), 0)

	result = db.SelectMany(1, "1", "asfasf", &systems)
	helper.CheckNotEqual(t, result.Error, nil)

	result = db.SelectMany(1, "1", "things", nil)
	helper.CheckNotEqual(t, result.Error, nil)

	result = db.SelectMany(1, "1", "things", systems)
	helper.CheckNotEqual(t, result.Error, nil)

	aSystem := system{}
	result = db.SelectMany(1, "1", "things", &aSystem)
	helper.CheckNotEqual(t, result.Error, nil)
	db.DB.Close()
	db.DB = nil
}

func TestSelect(t *testing.T) {
	item := system{}
	result := db.Select(1, "1", "things", &item)
	helper.CheckNotEqual(t, result.Error, nil)

	err := db.Connect()
	helper.CheckEqual(t, err, nil)

	result = db.Select(1, "1", "things", &item)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckNotEqual(t, result.IsSuccess(), false)

	result = db.Select(1, "1", "things", item)
	helper.CheckNotEqual(t, result.Error, nil)
	helper.CheckNotEqual(t, result.IsSuccess(), true)

	result = db.Select(1, "sysnax", "things", &item)
	helper.CheckNotEqual(t, result.Error, nil)
	helper.CheckNotEqual(t, result.IsSuccess(), true)

	result = db.Select("e1-3", "sysname", "things", &item)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, result.IsSuccess(), true)
	helper.CheckEqual(t, item.Sysname, "e1-3")
	helper.CheckEqual(t, *item.Vlan, 1)
	db.DB.Close()
	db.DB = nil
}

func TestUpdate(t *testing.T) {
	item := system{}
	err := db.Connect()
	helper.CheckEqual(t, err, nil)

	result := db.Select("e1-3", "sysname", "things", &item)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, result.IsSuccess(), true)
	helper.CheckEqual(t, *item.Vlan, 1)

	*item.Vlan = 42
	result = db.Update("e1-3", "sysname", "things", &item)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, result.Affected, 1)
	helper.CheckEqual(t, result.Ok, 1)
	helper.CheckEqual(t, result.Failed, 0)

	*item.Vlan = 0
	result = db.Select("e1-3", "sysname", "things", &item)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, result.IsSuccess(), true)
	helper.CheckEqual(t, *item.Vlan, 42)

	*item.Vlan = 1
	result = db.Update("e1-3", "sysname", "things", &item)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, result.Affected, 1)
	helper.CheckEqual(t, result.Ok, 1)
	helper.CheckEqual(t, result.Failed, 0)

	*item.Vlan = 0
	result = db.Select("e1-3", "sysname", "things", &item)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, result.IsSuccess(), true)
	helper.CheckEqual(t, *item.Vlan, 1)
	db.DB.Close()
	db.DB = nil
}

func TestInsert(t *testing.T) {
	item := system{}
	err := db.Connect()
	helper.CheckEqual(t, err, nil)

	result := db.Select("kjeks", "sysname", "things", &item)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, result.IsSuccess(), false)

	item.Sysname = "kjeks"
	vlan := 42
	item.Vlan = &vlan
	newip := "192.168.2.1"
	item.Ip = &newip
	result = db.Insert("things", &item)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, result.Affected, 1)
	helper.CheckEqual(t, result.Ok, 1)

	result = db.Select("kjeks", "sysname", "things", &item)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, result.IsSuccess(), true)

	result = db.Delete("kjeks", "sysname", "things")
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, result.Affected, 1)
	helper.CheckEqual(t, result.Ok, 1)
	helper.CheckEqual(t, result.Failed, 0)

	result = db.Upsert("kjeks", "sysname", "things", &item)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, *item.Vlan, 42)
	helper.CheckEqual(t, result.Affected, 1)
	helper.CheckEqual(t, result.Ok, 1)
	helper.CheckEqual(t, result.Failed, 0)

	*item.Vlan = 8128
	result = db.Upsert("kjeks", "sysname", "things", &item)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, result.Affected, 1)
	helper.CheckEqual(t, result.Ok, 1)
	helper.CheckEqual(t, result.Failed, 0)

	systems := make([]system, 0)
	result = db.SelectMany("kjeks", "sysname", "things", &systems)
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, len(systems), 1)
	helper.CheckEqual(t, *systems[0].Vlan, 8128)

	result = db.Delete("kjeks", "sysname", "things")
	helper.CheckEqual(t, result.Error, nil)
	helper.CheckEqual(t, result.Affected, 1)
	helper.CheckEqual(t, result.Ok, 1)
	helper.CheckEqual(t, result.Failed, 0)
	db.DB.Close()
	db.DB = nil
}
