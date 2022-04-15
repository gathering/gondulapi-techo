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

package yolo

import (
	"fmt"

	"github.com/gathering/tech-online-backend/config"
	"github.com/gathering/tech-online-backend/db"
	"github.com/gathering/tech-online-backend/rest"
)

// TrackType is track type.
type TrackType string

const (
	trackTypeNet    TrackType = "net"
	trackTypeServer TrackType = "server"
)

// Track is a track.
type Track struct {
	ID   string    `column:"id" json:"id"`     // Generated, required, unique
	Type TrackType `column:"type" json:"type"` // Required
	Name string    `column:"name" json:"name"` // Required
}

// Tracks is a list of tracks.
type Tracks []*Track

func init() {
	rest.AddHandler("/tracks/", "^$", func() interface{} { return &Tracks{} })
	rest.AddHandler("/track/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Track{} })
}

// Get gets multiple tracks.
func (tracks *Tracks) Get(request *rest.Request) rest.Result {
	// Check params, prep filtering
	var whereArgs []interface{}
	if trackType, ok := request.QueryArgs["type"]; ok {
		whereArgs = append(whereArgs, "type", "=", trackType)
	}

	// Get
	dbResult := db.SelectMany(tracks, "tracks", whereArgs...)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

// Get gets a single track.
func (track *Track) Get(request *rest.Request) rest.Result {
	// Check params
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Get
	dbResult := db.Select(track, "tracks", "id", "=", id)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	if !dbResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "not found"}
	}
	return rest.Result{}
}

// Post creates a new track.
func (track *Track) Post(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.Result{Code: 403, Message: "Permission denied"}
	}

	// Validate
	if result := track.validate(); !result.IsOk() {
		return result
	}

	// Check if duplicate
	if exists, err := track.exists(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if exists {
		return rest.Result{Code: 409, Message: "duplicate ID"}
	}

	// Create and redirect
	result := track.create()
	if !result.IsOk() {
		return result
	}
	result.Code = 201
	result.Location = fmt.Sprintf("%v/track/%v/", config.Config.SitePrefix, track.ID)
	return result
}

// Put updates a track.
func (track *Track) Put(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.Result{Code: 403, Message: "Permission denied"}
	}

	// Check params
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Validate
	if track.ID != id {
		return rest.Result{Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := track.validate(); !result.IsOk() {
		return result
	}

	// Create or update
	return track.createOrUpdate()
}

// Delete deletes a track.
func (track *Track) Delete(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.Result{Code: 403, Message: "Permission denied"}
	}

	// Check params
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Check if it exists
	track.ID = id
	exists, err := track.exists()
	if err != nil {
		return rest.Result{Code: 500, Error: err}
	}
	if !exists {
		return rest.Result{Code: 404, Message: "not found"}
	}

	// Delete
	dbResult := db.Delete("tracks", "id", "=", track.ID)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (track *Track) create() rest.Result {
	if exists, err := track.exists(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if exists {
		return rest.Result{Code: 409, Message: "duplicate"}
	}

	dbResult := db.Insert("tracks", track)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (track *Track) createOrUpdate() rest.Result {
	exists, existsErr := track.exists()
	if existsErr != nil {
		return rest.Result{Code: 500, Error: existsErr}
	}

	var dbResult db.Result
	if exists {
		dbResult = db.Update("tracks", track, "id", "=", track.ID)
	} else {
		dbResult = db.Insert("tracks", track)
	}
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (track *Track) exists() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM tracks WHERE id = $1", track.ID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (track *Track) validate() rest.Result {
	switch {
	case track.ID == "":
		return rest.Result{Code: 400, Message: "missing ID"}
	case !track.validateType():
		return rest.Result{Code: 400, Message: "missing or invalid type"}
	}

	return rest.Result{}
}

func (track *Track) validateType() bool {
	switch track.Type {
	case trackTypeNet:
		fallthrough
	case trackTypeServer:
		return true
	default:
		return false
	}
}
