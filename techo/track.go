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
	"github.com/gathering/gondulapi"
	"github.com/gathering/gondulapi/db"
	"github.com/gathering/gondulapi/receiver"
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
}

// Tracks is a list of tracks.
type Tracks []*Track

func init() {
	receiver.AddHandler("/tracks/", "^$", func() interface{} { return &Tracks{} })
	receiver.AddHandler("/track/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Track{} })
}

// Get gets multiple tracks.
func (tracks *Tracks) Get(request *gondulapi.Request) gondulapi.Result {
	var whereArgs []interface{}
	if trackType, ok := request.QueryArgs["type"]; ok {
		whereArgs = append(whereArgs, "type", "=", trackType)
	}

	selectErr := db.SelectMany(tracks, "tracks", whereArgs...)
	if selectErr != nil {
		return gondulapi.Result{Error: selectErr}
	}

	return gondulapi.Result{}
}

// Get gets a single station.
func (track *Track) Get(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	}

	found, err := db.Select(track, "tracks", "id", "=", id)
	if err != nil {
		return gondulapi.Result{Error: err}
	}
	if !found {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}

	return gondulapi.Result{}
}

// Post creates a new station.
func (track *Track) Post(request *gondulapi.Request) gondulapi.Result {
	if result := track.validate(); result.HasErrorOrCode() {
		return result
	}

	if exists, err := track.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate ID"}
	}

	return track.create()
}

// Put updates a station.
func (track *Track) Put(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}

	if track.ID != id {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := track.validate(); result.HasErrorOrCode() {
		return result
	}

	return track.update()
}

// Delete deletes a station.
func (track *Track) Delete(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}

	track.ID = id
	exists, err := track.exists()
	if err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	}
	if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Delete("tracks", "id", "=", track.ID)
	result.Error = err
	return result
}

func (track *Track) create() gondulapi.Result {
	if exists, err := track.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate"}
	}

	result, err := db.Insert("tracks", track)
	result.Error = err
	return result
}

func (track *Track) update() gondulapi.Result {
	if exists, err := track.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Update("tracks", track, "id", "=", track.ID)
	result.Error = err
	return result
}

func (track *Track) exists() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM tracks WHERE id = $1", track.ID)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}

func (track *Track) validate() gondulapi.Result {
	switch {
	case track.ID == "":
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	case !track.validateType():
		return gondulapi.Result{Code: 400, Message: "missing or invalid type"}
	default:
		return gondulapi.Result{}
	}
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
