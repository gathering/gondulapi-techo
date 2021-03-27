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
	"github.com/google/uuid"
)

/*
 * TODO:
 * - Don't show credentials to participants before they're time slot is active.
 */

// StationStatus is the station status.
type StationStatus string

const (
	stationStatusPreparing   StationStatus = "preparing"
	stationStatusActive      StationStatus = "active"
	stationStatusDirty       StationStatus = "dirty"
	stationStatusTerminated  StationStatus = "terminated"
	stationStatusMaintenance StationStatus = "maintenance"
)

// Station is station.
type Station struct {
	ID          *uuid.UUID    `column:"id" json:"id"`                   // Generated, required, unique
	TrackID     string        `column:"track" json:"track"`             // Required
	Shortname   string        `column:"shortname" json:"shortname"`     // Required
	Status      StationStatus `column:"status" json:"status"`           // Required
	Credentials string        `column:"credentials" json:"credentials"` // Host, port, password, etc. (typically hidden)
	Notes       string        `column:"notes" json:"notes"`             // Misc. notes
}

// Stations is a list of stations.
type Stations []*Station

func init() {
	receiver.AddHandler("/stations/", "^$", func() interface{} { return &Stations{} })
	receiver.AddHandler("/station/", "^(?:(?P<id>[^/]+)/)?", func() interface{} { return &Station{} })
}

// Get gets multiple stations.
func (stations *Stations) Get(request *gondulapi.Request) error {
	var whereArgs []interface{}
	if shortname, ok := request.QueryArgs["shortname"]; ok {
		whereArgs = append(whereArgs, "shortname", "=", shortname)
	}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}
	if status, ok := request.QueryArgs["status"]; ok {
		whereArgs = append(whereArgs, "status", "=", status)
	}

	selectErr := db.SelectMany(stations, "stations", whereArgs...)
	if selectErr != nil {
		return gondulapi.Error{Code: 500, Message: "failed to query database"}
	}

	return nil
}

// Get gets a single station.
func (station *Station) Get(request *gondulapi.Request) error {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	found, err := db.Select(station, "stations", "id", "=", id)
	if err != nil {
		return err
	}
	if !found {
		return gondulapi.Error{Code: 404, Message: "not found"}
	}

	return nil
}

// Post creates a new station.
func (station *Station) Post(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if exists, err := station.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 409, Message: "duplicate ID"}
	}

	if station.ID == nil {
		newID := uuid.New()
		station.ID = &newID
	}
	if err := station.validate(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}

	return station.create()
}

// Put updates a station.
func (station *Station) Put(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "missing ID"}
	}
	id, uuidErr := uuid.Parse(rawID)
	if uuidErr != nil {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "invalid ID"}
	}

	if *station.ID != id {
		return gondulapi.WriteReport{Failed: 1}, fmt.Errorf("mismatch between URL and JSON IDs")
	}
	if err := station.validate(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	return station.update()
}

// Delete deletes a station.
func (station *Station) Delete(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "missing ID"}
	}
	id, uuidErr := uuid.Parse(rawID)
	if uuidErr != nil {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "invalid ID"}
	}

	station.ID = &id
	exists, err := station.exists()
	if err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 404, Message: "not found"}
	}
	return db.Delete("stations", "id", "=", station.ID)
}

func (station *Station) create() (gondulapi.WriteReport, error) {
	if exists, err := station.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 409, Message: "duplicate"}
	}

	return db.Insert("stations", station)
}

func (station *Station) update() (gondulapi.WriteReport, error) {
	if exists, err := station.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 404, Message: "not found"}
	}

	return db.Update("stations", station, "id", "=", station.ID)
}

func (station *Station) exists() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM stations WHERE id = $1", station.ID)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}

func (station *Station) existsShortname() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM stations WHERE track = $1 AND shortname = $2", station.TrackID, station.Shortname)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}

func (station *Station) validate() error {
	switch {
	case station.ID == nil:
		return gondulapi.Error{Code: 400, Message: "missing ID"}
	case station.TrackID == "":
		return gondulapi.Error{Code: 400, Message: "missing track ID"}
	case !station.validateStatus():
		return gondulapi.Error{Code: 400, Message: "missing or invalid status"}
	}

	if ok, err := station.checkUniqueFields(); err != nil {
		return err
	} else if !ok {
		return gondulapi.Error{Code: 409, Message: "combination of track and shortname already exists"}
	}

	track := Track{ID: station.TrackID}
	if exists, err := track.exists(); err != nil {
		return err
	} else if !exists {
		return gondulapi.Error{Code: 400, Message: "referenced track does not exist"}
	}

	return nil
}

func (station *Station) validateStatus() bool {
	switch station.Status {
	case stationStatusPreparing:
		fallthrough
	case stationStatusActive:
		fallthrough
	case stationStatusDirty:
		fallthrough
	case stationStatusTerminated:
		fallthrough
	case stationStatusMaintenance:
		return true
	default:
		return false
	}
}

func (station *Station) checkUniqueFields() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM stations WHERE id != $1 AND track = $2 AND shortname = $3", station.ID, station.TrackID, station.Shortname)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return !hasNext, nil
}
