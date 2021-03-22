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
	"github.com/gathering/gondulapi/receiver"
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
	FamilyID      *string `column:"family_id" json:"family_id"`         // Required, unique with local ID
	LocalID       *string `column:"local_id" json:"local_id"`           // Required, unique with family ID
	Sequence      *int    `column:"sequence" json:"sequence,omitempty"` // For sorting
	Name          *string `column:"name" json:"name,omitempty"`
	Content       *string `column:"content" json:"content,omitempty"`
	ContentFormat *string `column:"content_format" json:"content_format,omitempty"` // E.g. "plaintext" or "markdown"
}

// Documents is a list of documents.
type Documents []*Document

func init() {
	receiver.AddHandler("/document-families/", func() interface{} { return &DocumentFamilies{} })
	receiver.AddHandler("/document-family/", func() interface{} { return &DocumentFamily{} })
	receiver.AddHandler("/documents/", func() interface{} { return &Documents{} })
	receiver.AddHandler("/document/", func() interface{} { return &Document{} })
}

// Get gets multiple families.
func (families *DocumentFamilies) Get(request *gondulapi.Request) error {
	if request.Element != "" {
		return gondulapi.Errorf(400, "element not allowed")
	}

	var queryBuilder strings.Builder
	nextQueryArgID := 1
	var queryArgs []interface{}
	queryBuilder.WriteString("SELECT id,name FROM document_families")
	if request.Limit > 0 {
		queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%v", nextQueryArgID))
		nextQueryArgID++
		queryArgs = append(queryArgs, request.Limit)
	}

	rows, err := db.DB.Query(queryBuilder.String(), queryArgs...)
	if err != nil {
		return gondulapi.Errorf(500, "failed to query database")
	}
	defer func() {
		rows.Close()
	}()

	for rows.Next() {
		var family DocumentFamily
		err = rows.Scan(&family.ID, &family.Name)
		if err != nil {
			return gondulapi.Errorf(500, "failed to scan entity from the database")
		}
		*families = append(*families, &family)
	}

	return nil
}

// Get gets a single family.
func (family *DocumentFamily) Get(request *gondulapi.Request) error {
	if request.Element == "" {
		return gondulapi.Errorf(400, "ID required")
	}

	rows, err := db.DB.Query("SELECT id,name FROM document_families WHERE id = $1", request.Element)
	if err != nil {
		return gondulapi.Errorf(500, "failed to query database")
	}
	defer func() {
		rows.Close()
	}()

	if !rows.Next() {
		return gondulapi.Errorf(404, "not found")
	}

	err = rows.Scan(&family.ID, &family.Name)
	if err != nil {
		return gondulapi.Errorf(500, "failed to parse data from database")
	}

	return nil
}

// Post creates a new family.
func (family *DocumentFamily) Post(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if exists, err := family.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(409, "duplicate ID")
	}
	return family.create()
}

// Put creates or updates a family.
func (family *DocumentFamily) Put(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if request.Element == "" {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "ID required")
	}
	if *family.ID != request.Element {
		return gondulapi.WriteReport{Failed: 1}, fmt.Errorf("mismatch between URL and JSON IDs")
	}
	return family.createOrUpdate()
}

// Delete deletes a family.
func (family *DocumentFamily) Delete(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if request.Element == "" {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "ID required")
	}
	exists, err := family.exists()
	if err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(404, "not found")
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
	if request.Element != "" {
		return gondulapi.Errorf(400, "element not allowed")
	}

	var queryBuilder strings.Builder
	nextQueryArgID := 1
	var queryArgs []interface{}
	_, brief := request.Args["brief"]
	if brief {
		queryBuilder.WriteString("SELECT family_id,local_id,name,sequence FROM documents")
	} else {
		queryBuilder.WriteString("SELECT family_id,local_id,name,sequence,content,content_format FROM documents")
	}
	if familyID, ok := request.Args["family_id"]; ok && len(familyID) > 0 && len(familyID[0]) > 0 {
		queryBuilder.WriteString(fmt.Sprintf(" WHERE family_id = $%v", nextQueryArgID))
		nextQueryArgID++
		queryArgs = append(queryArgs, familyID[0])
	}
	if request.Limit > 0 {
		queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%v", nextQueryArgID))
		nextQueryArgID++
		queryArgs = append(queryArgs, request.Limit)
	}

	rows, err := db.DB.Query(queryBuilder.String(), queryArgs...)
	if err != nil {
		return gondulapi.Errorf(500, "failed to query database")
	}
	defer func() {
		rows.Close()
	}()

	for rows.Next() {
		var document Document
		if brief {
			err = rows.Scan(&document.FamilyID, &document.LocalID, &document.Name, &document.Sequence)
		} else {
			err = rows.Scan(&document.FamilyID, &document.LocalID, &document.Name, &document.Sequence, &document.Content, &document.ContentFormat)
		}
		if err != nil {
			return gondulapi.Errorf(500, "failed to scan entity from the database")
		}
		*documents = append(*documents, &document)
	}

	return nil
}

// Get gets a single document.
func (document *Document) Get(request *gondulapi.Request) error {
	if request.Element == "" {
		return gondulapi.Errorf(400, "family and local IDs required")
	}
	var familyID string
	var localID string
	spaceElement := strings.Replace(request.Element, "/", " ", -1)
	if _, err := fmt.Sscanf(spaceElement, "%s %s", &familyID, &localID); err != nil {
		return gondulapi.Errorf(400, "family and local IDs required")
	} else if familyID == "" || localID == "" {
		return gondulapi.Errorf(400, "family and local IDs required")
	}

	rows, err := db.DB.Query("SELECT family_id,local_id,name,sequence,content,content_format FROM documents WHERE family_id = $1 AND local_id = $2", familyID, localID)
	if err != nil {
		return gondulapi.Errorf(500, "failed to query database")
	}
	defer func() {
		rows.Close()
	}()

	if !rows.Next() {
		return gondulapi.Errorf(404, "not found")
	}

	err = rows.Scan(&document.FamilyID, &document.LocalID, &document.Name, &document.Sequence, &document.Content, &document.ContentFormat)
	if err != nil {
		return gondulapi.Errorf(500, "failed to parse data from database")
	}

	return nil
}

// Post creates a new document.
func (document *Document) Post(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if document.FamilyID == nil || *document.FamilyID == "" || document.LocalID == nil || *document.LocalID == "" {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "family and local IDs required")
	}

	if exists, err := document.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(409, "duplicate ID")
	}

	family := DocumentFamily{ID: document.FamilyID}
	if exists, err := family.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "family does not exist")
	}

	return document.create()
}

// Put creates or updates a document.
func (document *Document) Put(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if request.Element == "" {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "family and local IDs required")
	}
	var familyID string
	var localID string
	spaceElement := strings.Replace(request.Element, "/", " ", -1)
	if _, err := fmt.Sscanf(spaceElement, "%s %s", &familyID, &localID); err != nil {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "family and local IDs required")
	} else if familyID == "" || localID == "" {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "family and local IDs required")
	}

	if document.FamilyID == nil || *document.FamilyID == "" || document.LocalID == nil || *document.LocalID == "" {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "family and local IDs required")
	}
	if *document.FamilyID != familyID || *document.LocalID != localID {
		return gondulapi.WriteReport{Failed: 1}, fmt.Errorf("mismatch between URL and JSON IDs")
	}

	family := DocumentFamily{ID: document.FamilyID}
	if exists, err := family.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "family does not exist")
	}

	return document.createOrUpdate()
}

// Delete deletes a document.
func (document *Document) Delete(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if request.Element == "" {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "family and local IDs required")
	}
	var familyID string
	var localID string
	spaceElement := strings.Replace(request.Element, "/", " ", -1)
	if _, err := fmt.Sscanf(spaceElement, "%s %s", &familyID, &localID); err != nil {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "family and local IDs required")
	} else if familyID == "" || localID == "" {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "family and local IDs required")
	}

	document.FamilyID = &familyID
	document.LocalID = &localID
	exists, err := document.exists()
	if err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(404, "not found")
	}
	return db.Delete("documents", "family_id", "=", document.FamilyID, "local_id", "=", document.LocalID)
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
	return db.Update("documents", document, "family_id", "=", document.FamilyID, "local_id", "=", document.LocalID)
}

func (document *Document) exists() (bool, error) {
	rows, err := db.DB.Query("SELECT family_id,local_id FROM documents WHERE family_id = $1 AND local_id = $2", document.FamilyID, document.LocalID)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}
