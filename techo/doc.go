/*
Tech:Online backend
Copyright 2020, Kristian Lyngstøl <kly@kly.no>

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

package techo

import (
	"fmt"
	"strings"

	"github.com/gathering/gondulapi"
	"github.com/gathering/gondulapi/db"
	"github.com/gathering/gondulapi/helper"
	"github.com/gathering/gondulapi/receiver"
	"github.com/google/uuid"
)

// DocumentFamily is a category of documents.
type DocumentFamily struct {
	ID   *string `column:"id" json:"id"` // Required, unique
	Name *string `column:"name" json:"name"`
}

// DocumentFamilies is a list of families.
type DocumentFamilies []*DocumentFamily

// Document is a document.
type Document struct {
	ID            *uuid.UUID `column:"id" json:"id"`               // Required, unique
	FamilyID      *string    `column:"family_id" json:"family_id"` // Required
	Shortname     *string    `column:"shortname" json:"shortname"` // Required, unique with family ID
	Name          *string    `column:"name" json:"name,omitempty"`
	Content       *string    `column:"content" json:"content,omitempty"`
	ContentFormat *string    `column:"content_format" json:"content_format,omitempty"` // E.g. "plaintext" or "markdown"
	Sequence      *int       `column:"sequence" json:"sequence,omitempty"`             // For sorting
}

// Documents is a list of documents.
type Documents []*Document

func init() {
	receiver.AddHandler("/document-families/", "", func() interface{} { return &DocumentFamilies{} })
	receiver.AddHandler("/document-family/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &DocumentFamily{} })
	receiver.AddHandler("/documents/", "", func() interface{} { return &Documents{} })
	receiver.AddHandler("/document/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Document{} })
}

// Get gets multiple families.
func (families *DocumentFamilies) Get(request *gondulapi.Request) error {
	var queryBuilder strings.Builder
	nextQueryArgID := 1
	var queryArgs []interface{}
	queryBuilder.WriteString("SELECT id,name FROM document_families")
	if request.ListLimit > 0 {
		queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%v", nextQueryArgID))
		nextQueryArgID++
		queryArgs = append(queryArgs, request.ListLimit)
	}

	rows, err := db.DB.Query(queryBuilder.String(), queryArgs...)
	if err != nil {
		return gondulapi.Error{Code: 500, Message: "failed to query database"}
	}
	defer func() {
		rows.Close()
	}()

	for rows.Next() {
		var family DocumentFamily
		err = rows.Scan(&family.ID, &family.Name)
		if err != nil {
			return gondulapi.Error{Code: 500, Message: "failed to scan entity from the database"}
		}
		*families = append(*families, &family)
	}

	return nil
}

// Get gets a single family.
func (family *DocumentFamily) Get(request *gondulapi.Request) error {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	rows, err := db.DB.Query("SELECT id,name FROM document_families WHERE id = $1", id)
	if err != nil {
		return gondulapi.Error{Code: 500, Message: "failed to query database"}
	}
	defer func() {
		rows.Close()
	}()

	if !rows.Next() {
		return gondulapi.Error{Code: 404, Message: "not found"}
	}

	err = rows.Scan(&family.ID, &family.Name)
	if err != nil {
		return gondulapi.Error{Code: 500, Message: "failed to parse data from database"}
	}

	return nil
}

// Post creates a new family.
func (family *DocumentFamily) Post(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if exists, err := family.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 409, Message: "duplicate ID"}
	}
	return family.create()
}

// Put creates or updates a family.
func (family *DocumentFamily) Put(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	if *family.ID != id {
		return gondulapi.WriteReport{Failed: 1}, fmt.Errorf("mismatch between URL and JSON IDs")
	}

	return family.createOrUpdate()
}

// Delete deletes a family.
func (family *DocumentFamily) Delete(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	family.ID = &id
	exists, err := family.exists()
	if err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 404, Message: "not found"}
	}

	return db.Delete("document_families", "id", family.ID)
}

func (family *DocumentFamily) createOrUpdate() (gondulapi.WriteReport, error) {
	exists, err := family.exists()
	if err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	if exists {
		return family.update()
	}
	return family.create()
}

func (family *DocumentFamily) create() (gondulapi.WriteReport, error) {
	return db.Insert("document_families", family)
}

func (family *DocumentFamily) update() (gondulapi.WriteReport, error) {
	return db.Update("document_families", family, "id", "=", family.ID)
}

func (family *DocumentFamily) exists() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM document_families WHERE id = $1", family.ID)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}

// Get gets multiple documents.
func (documents *Documents) Get(request *gondulapi.Request) error {
	var queryBuilder strings.Builder
	nextQueryArgID := 1
	var queryArgs []interface{}
	if request.ListBrief {
		queryBuilder.WriteString("SELECT id,family_id,shortname,name,sequence FROM documents WHERE family_id LIKE $1 AND shortname LIKE $2")
	} else {
		queryBuilder.WriteString("SELECT id,family_id,shortname,name,sequence,content,content_format FROM documents WHERE family_id LIKE $1 AND shortname LIKE $2")
	}
	familyIDArg := "%"
	if familyID, ok := request.QueryArgs["family"]; ok {
		familyIDArg = familyID
	}
	nextQueryArgID++
	queryArgs = append(queryArgs, familyIDArg)
	shortnameArg := "%"
	if shortname, ok := request.QueryArgs["shortname"]; ok {
		shortnameArg = shortname
	}
	nextQueryArgID++
	queryArgs = append(queryArgs, shortnameArg)
	if request.ListLimit > 0 {
		queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%v", nextQueryArgID))
		nextQueryArgID++
		queryArgs = append(queryArgs, request.ListLimit)
	}

	rows, err := db.DB.Query(queryBuilder.String(), queryArgs...)
	if err != nil {
		return gondulapi.Error{Code: 500, Message: "failed to query database"}
	}
	defer func() {
		rows.Close()
	}()

	for rows.Next() {
		var document Document
		if request.ListBrief {
			err = rows.Scan(&document.ID, &document.FamilyID, &document.Shortname, &document.Name, &document.Sequence)
		} else {
			err = rows.Scan(&document.ID, &document.FamilyID, &document.Shortname, &document.Name, &document.Sequence, &document.Content, &document.ContentFormat)
		}
		if err != nil {
			return gondulapi.Error{Code: 500, Message: "failed to scan entity from the database"}
		}
		*documents = append(*documents, &document)
	}

	return nil
}

// Get gets a single document.
func (document *Document) Get(request *gondulapi.Request) error {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	rows, err := db.DB.Query("SELECT id,family_id,shortname,name,sequence,content,content_format FROM documents WHERE id = $1", id)
	if err != nil {
		return gondulapi.Error{Code: 500, Message: "failed to query database"}
	}
	defer func() {
		rows.Close()
	}()

	if !rows.Next() {
		return gondulapi.Error{Code: 404, Message: "not found"}
	}

	err = rows.Scan(&document.ID, &document.FamilyID, &document.Shortname, &document.Name, &document.Sequence, &document.Content, &document.ContentFormat)
	if err != nil {
		return gondulapi.Error{Code: 500, Message: "failed to parse data from database"}
	}

	return nil
}

// Post creates a new document.
func (document *Document) Post(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if helper.IsEmpty(document.FamilyID) || helper.IsEmpty(document.Shortname) {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "family and shortname required"}
	}

	if ok, err := document.checkUniqueFields(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if !ok {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 409, Message: "combination of family and shortname already exists"}
	}

	if document.ID == nil {
		newID := uuid.New()
		document.ID = &newID
	}
	if exists, err := document.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 409, Message: "duplicate ID"}
	}

	return document.create()
}

// Put creates or updates a document.
func (document *Document) Put(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	if (*document.ID).String() != id {
		return gondulapi.WriteReport{Failed: 1}, fmt.Errorf("mismatch between URL and JSON IDs")
	}

	if ok, err := document.checkUniqueFields(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if !ok {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 409, Message: "combination of family and shortname already exists"}
	}

	return document.createOrUpdate()
}

// Delete deletes a document.
func (document *Document) Delete(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "missing ID"}
	}
	uuid, uuidError := uuid.Parse(id)
	if uuidError != nil {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "invalid ID"}
	}

	document.ID = &uuid
	exists, err := document.exists()
	if err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 404, Message: "not found"}
	}
	return db.Delete("documents", "id", "=", document.FamilyID)
}

func (document *Document) createOrUpdate() (gondulapi.WriteReport, error) {
	exists, err := document.exists()
	if err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	if exists {
		return document.update()
	}
	return document.create()
}

func (document *Document) create() (gondulapi.WriteReport, error) {
	return db.Insert("documents", document)
}

func (document *Document) update() (gondulapi.WriteReport, error) {
	return db.Update("documents", document, "id", "=", document.ID)
}

func (document *Document) exists() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM documents WHERE id = $1", document.ID)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}

func (document *Document) checkUniqueFields() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM documents WHERE family_id = $1 AND shortname = $2", document.FamilyID, document.Shortname)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return !hasNext, nil
}
