/*
Tech:Online backend
Copyright 2020, Kristian Lyngstøl <kly@kly.no>
Copyright 2021, Håvard Ose Nordstrand <hon@hon.one>

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
	"time"

	"github.com/gathering/gondulapi"
	"github.com/gathering/gondulapi/db"
	"github.com/gathering/gondulapi/receiver"
	"github.com/google/uuid"
)

// DocumentFamily is a category of documents.
type DocumentFamily struct {
	ID   string `column:"id" json:"id"` // Required, unique
	Name string `column:"name" json:"name"`
}

// DocumentFamilies is a list of families.
type DocumentFamilies []*DocumentFamily

// Document is a document.
type Document struct {
	ID            *uuid.UUID `column:"id" json:"id"`               // Generated, required, unique
	FamilyID      string     `column:"family" json:"family"`       // Required
	Shortname     string     `column:"shortname" json:"shortname"` // Required, unique with family ID
	Name          string     `column:"name" json:"name"`
	Content       string     `column:"content" json:"content"`
	ContentFormat string     `column:"content_format" json:"content_format"` // E.g. "plaintext" or "markdown"
	Sequence      *int       `column:"sequence" json:"sequence"`             // For sorting
	LastChange    *time.Time `column:"last_change" json:"last_change"`
}

// Documents is a list of documents.
type Documents []*Document

func init() {
	receiver.AddHandler("/document-families/", "^$", func() interface{} { return &DocumentFamilies{} })
	receiver.AddHandler("/document-family/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &DocumentFamily{} })
	receiver.AddHandler("/documents/", "^$", func() interface{} { return &Documents{} })
	receiver.AddHandler("/document/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Document{} })
}

// Get gets multiple families.
func (families *DocumentFamilies) Get(request *gondulapi.Request) gondulapi.Result {
	// TODO order by sequence
	selectErr := db.SelectMany(families, "document_families")
	if selectErr != nil {
		return gondulapi.Result{Error: selectErr}
	}

	return gondulapi.Result{}
}

// Get gets a single family.
func (family *DocumentFamily) Get(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	}

	found, err := db.Select(family, "document_families", "id", "=", id)
	if err != nil {
		return gondulapi.Result{Error: err}
	}
	if !found {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}

	return gondulapi.Result{}
}

// Post creates a new family.
func (family *DocumentFamily) Post(request *gondulapi.Request) gondulapi.Result {
	if family.ID == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}

	if exists, err := family.exists(); err != nil {
		return gondulapi.Result{Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate ID"}
	}

	result := family.create()
	result.Code = 201
	result.Location = fmt.Sprintf("%v/document-family/%v", gondulapi.Config.SitePrefix, family.ID)
	return result
}

// Put updates a family.
func (family *DocumentFamily) Put(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}

	if family.ID != id {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "mismatch between URL and JSON IDs"}
	}

	exists, existsErr := family.exists()
	if existsErr != nil {
		return gondulapi.Result{Failed: 1, Error: existsErr}
	}
	if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	return family.update()
}

// Delete deletes a family.
func (family *DocumentFamily) Delete(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}

	family.ID = id
	exists, err := family.exists()
	if err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	}
	if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Delete("document_families", "id", family.ID)
	result.Error = err
	return result
}

func (family *DocumentFamily) create() gondulapi.Result {
	if exists, err := family.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate"}
	}

	result, err := db.Insert("document_families", family)
	result.Error = err
	return result
}

func (family *DocumentFamily) update() gondulapi.Result {
	if exists, err := family.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Update("document_families", family, "id", "=", family.ID)
	result.Error = err
	return result
}

func (family *DocumentFamily) exists() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM document_families WHERE id = $1", family.ID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

// Get gets multiple documents.
func (documents *Documents) Get(request *gondulapi.Request) gondulapi.Result {
	var whereArgs []interface{}
	if shortname, ok := request.QueryArgs["shortname"]; ok {
		whereArgs = append(whereArgs, "shortname", "=", shortname)
	}
	if familyID, ok := request.QueryArgs["family"]; ok {
		whereArgs = append(whereArgs, "family", "=", familyID)
	}

	selectErr := db.SelectMany(documents, "documents", whereArgs...)
	if selectErr != nil {
		return gondulapi.Result{Error: selectErr}
	}

	return gondulapi.Result{}
}

// Get gets a single document.
func (document *Document) Get(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	}

	found, err := db.Select(document, "documents", "id", "=", id)
	if err != nil {
		return gondulapi.Result{Error: err}
	}
	if !found {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}

	return gondulapi.Result{}
}

// Post creates a new document.
func (document *Document) Post(request *gondulapi.Request) gondulapi.Result {
	if document.ID == nil {
		newID := uuid.New()
		document.ID = &newID
	}
	now := time.Now()
	document.LastChange = &now
	if result := document.validate(true); result.HasErrorOrCode() {
		return result
	}

	result := document.create()
	result.Code = 201
	result.Location = fmt.Sprintf("%v/document/%v", gondulapi.Config.SitePrefix, document.ID)
	return result
}

// Put updates a document.
func (document *Document) Put(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}

	if document.ID != nil && (*document.ID).String() != id {
		return gondulapi.Result{Failed: 1, Message: "mismatch between URL and JSON IDs"}
	}

	now := time.Now()
	document.LastChange = &now

	if result := document.validate(false); result.HasErrorOrCode() {
		return result
	}

	return document.update()
}

// Delete deletes a document.
func (document *Document) Delete(request *gondulapi.Request) gondulapi.Result {
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists || rawID == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}
	id, uuidError := uuid.Parse(rawID)
	if uuidError != nil {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid ID"}
	}

	document.ID = &id
	exists, err := document.exists()
	if err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	}
	if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Delete("documents", "id", "=", document.ID)
	result.Error = err
	return result
}

func (document *Document) create() gondulapi.Result {
	if exists, err := document.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate"}
	}

	result, err := db.Insert("documents", document)
	result.Error = err
	return result
}

func (document *Document) update() gondulapi.Result {
	if exists, err := document.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Update("documents", document, "id", "=", document.ID)
	result.Error = err
	return result
}

func (document *Document) exists() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM documents WHERE id = $1", document.ID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (document *Document) validate(new bool) gondulapi.Result {
	switch {
	case document.ID == nil:
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	case document.FamilyID == "":
		return gondulapi.Result{Code: 400, Message: "missing family ID"}
	case document.Shortname == "":
		return gondulapi.Result{Code: 400, Message: "missing shortname"}
	case document.LastChange == nil:
		return gondulapi.Result{Code: 400, Message: "missing last update time"}
	}

	// Check if existence is as expected
	if exists, err := document.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if new && exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate ID"}
	} else if !new && !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	if exists, err := document.existsFamilyShortname(); err != nil {
		return gondulapi.Result{Error: err}
	} else if exists {
		return gondulapi.Result{Code: 409, Message: "combination of family and shortname already exists"}
	}

	family := DocumentFamily{ID: document.FamilyID}
	if exists, err := family.exists(); err != nil {
		return gondulapi.Result{Error: err}
	} else if !exists {
		return gondulapi.Result{Code: 400, Message: "referenced family does not exist"}
	}

	return gondulapi.Result{}
}

func (document *Document) existsFamilyShortname() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM documents WHERE id != $1 AND family = $2 AND shortname = $3", document.ID, document.FamilyID, document.Shortname)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}
