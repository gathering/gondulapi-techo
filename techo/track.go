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
	ID                string    `column:"id" json:"id"`                                   // Generated, required, unique
	Type              TrackType `column:"type" json:"type"`                               // Required
	StationPermanent  *bool     `column:"station_permanent" json:"station_permanent"`     // Required, if it should be reused or if the URLs to create and destroy should be used
	StationCreateURL  string    `column:"station_create_url" json:"station_create_url"`   // URL to create new station
	StationDestroyURL string    `column:"station_destroy_url" json:"station_destroy_url"` // URL to destroy existing station
	StationCountMax   int       `column:"station_count_max" json:"station_count_max"`     // Max number of stations to create automatically
}

// Tracks is a list of tracks.
type Tracks []*Track

func init() {
	receiver.AddHandler("/tracks/", "^$", func() interface{} { return &Tracks{} })
	receiver.AddHandler("/track/", "^(?:(?P<id>[^/]+)/)?", func() interface{} { return &Track{} })
}

// Get gets multiple tracks.
func (tracks *Tracks) Get(request *gondulapi.Request) error {
	var whereArgs []interface{}
	if trackType, ok := request.QueryArgs["type"]; ok {
		whereArgs = append(whereArgs, "type", "=", trackType)
	}

	selectErr := db.SelectMany(tracks, "tracks", whereArgs...)
	if selectErr != nil {
		return gondulapi.Error{Code: 500, Message: "failed to query database"}
	}

	return nil
}

// Get gets a single station.
func (track *Track) Get(request *gondulapi.Request) error {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	found, err := db.Select(track, "tracks", "id", "=", id)
	if err != nil {
		return err
	}
	if !found {
		return gondulapi.Error{Code: 404, Message: "not found"}
	}

	return nil
}

// Post creates a new station.
func (track *Track) Post(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if err := track.validate(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}

	if exists, err := track.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 409, Message: "duplicate ID"}
	}

	return track.create()
}

// Put updates a station.
func (track *Track) Put(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	if track.ID != id {
		return gondulapi.WriteReport{Failed: 1}, fmt.Errorf("mismatch between URL and JSON IDs")
	}
	if err := track.validate(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}

	return track.update()
}

// Delete deletes a station.
func (track *Track) Delete(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	track.ID = id
	exists, err := track.exists()
	if err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 404, Message: "not found"}
	}
	return db.Delete("tracks", "id", "=", track.ID)
}

func (track *Track) create() (gondulapi.WriteReport, error) {
	if exists, err := track.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 409, Message: "duplicate"}
	}

	return db.Insert("tracks", track)
}

func (track *Track) update() (gondulapi.WriteReport, error) {
	if exists, err := track.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 404, Message: "not found"}
	}

	return db.Update("tracks", track, "id", "=", track.ID)
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

func (track *Track) validate() error {
	switch {
	case track.ID == "":
		return gondulapi.Error{Code: 400, Message: "missing ID"}
	case !track.validateType():
		return gondulapi.Error{Code: 400, Message: "missing or invalid type"}
	case track.StationPermanent == nil:
		return gondulapi.Error{Code: 400, Message: "missing station permanent status"}
	case track.StationCountMax < 0:
		return gondulapi.Error{Code: 400, Message: "negative station max count"}
	default:
		return nil
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
