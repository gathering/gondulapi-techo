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
	"time"

	"github.com/gathering/gondulapi"
	"github.com/gathering/gondulapi/db"
	"github.com/gathering/gondulapi/receiver"
	"github.com/google/uuid"
)

// Timeslot is a duration a station is booked to a participant.
type Timeslot struct {
	ID               *uuid.UUID `column:"id" json:"id"`                               // Generated, required, unique
	UserID           *uuid.UUID `column:"user_id" json:"user"`                        // Required ("user" is problematic for DB)
	TrackID          string     `column:"track" json:"track"`                         // Required
	StationShortname string     `column:"station_shortname" json:"station_shortname"` // May empty until station assigned
	BeginTime        *time.Time `column:"begin_time" json:"begin_time"`               // TODO
	EndTime          *time.Time `column:"end_time" json:"end_time"`                   // TODO
}

// Timeslots is a list of timeslots.
type Timeslots []*Timeslot

func init() {
	receiver.AddHandler("/timeslots/", "^$", func() interface{} { return &Timeslots{} })
	receiver.AddHandler("/timeslot/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Timeslot{} })
}

// Get gets multiple tracks.
func (timeslots *Timeslots) Get(request *gondulapi.Request) gondulapi.Result {
	var whereArgs []interface{}
	if userID, ok := request.QueryArgs["user"]; ok {
		whereArgs = append(whereArgs, "user_id", "=", userID)
	}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}
	if stationShortname, ok := request.QueryArgs["station_shortname"]; ok {
		whereArgs = append(whereArgs, "station_shortname", "=", stationShortname)
	}
	// TODO time filtering

	selectErr := db.SelectMany(timeslots, "timeslots", whereArgs...)
	if selectErr != nil {
		return gondulapi.Result{Code: 500, Message: "failed to query database"}
	}

	return gondulapi.Result{}
}

// Get gets a single station.
func (timeslot *Timeslot) Get(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	}

	found, err := db.Select(timeslot, "timeslots", "id", "=", id)
	if err != nil {
		return gondulapi.Result{Error: err}
	}
	if !found {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}

	return gondulapi.Result{}
}

// Post creates a new station.
func (timeslot *Timeslot) Post(request *gondulapi.Request) gondulapi.Result {
	if timeslot.ID == nil {
		newID := uuid.New()
		timeslot.ID = &newID
	}

	if result := timeslot.validate(); result.HasErrorOrCode() {
		return result
	}

	if exists, err := timeslot.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate ID"}
	}

	return timeslot.create()
}

// Put updates a station.
func (timeslot *Timeslot) Put(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}

	if (*timeslot.ID).String() != id {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := timeslot.validate(); result.HasErrorOrCode() {
		return result
	}

	return timeslot.update()
}

// Delete deletes a station.
func (timeslot *Timeslot) Delete(request *gondulapi.Request) gondulapi.Result {
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}
	id, uuidError := uuid.Parse(rawID)
	if uuidError != nil {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid ID"}
	}

	timeslot.ID = &id
	exists, err := timeslot.exists()
	if err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	}
	if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Delete("timeslots", "id", "=", timeslot.ID)
	result.Error = err
	return result
}

func (timeslot *Timeslot) create() gondulapi.Result {
	if exists, err := timeslot.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate"}
	}

	result, err := db.Insert("timeslots", timeslot)
	result.Error = err
	return result
}

func (timeslot *Timeslot) update() gondulapi.Result {
	if exists, err := timeslot.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Update("timeslots", timeslot, "id", "=", timeslot.ID)
	result.Error = err
	return result
}

func (timeslot *Timeslot) exists() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM timeslots WHERE id = $1", timeslot.ID)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}

func (timeslot *Timeslot) validate() gondulapi.Result {
	switch {
	case timeslot.ID == nil:
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	case timeslot.UserID == nil:
		return gondulapi.Result{Code: 400, Message: "missing user ID"}
	case timeslot.TrackID == "":
		return gondulapi.Result{Code: 400, Message: "missing track ID"}
	}

	// TODO validate time etc.

	user := User{ID: timeslot.UserID}
	if exists, err := user.exists(); err != nil {
		return gondulapi.Result{Error: err}
	} else if !exists {
		return gondulapi.Result{Code: 400, Message: "referenced user does not exist"}
	}
	track := Track{ID: timeslot.TrackID}
	if exists, err := track.exists(); err != nil {
		return gondulapi.Result{Error: err}
	} else if !exists {
		return gondulapi.Result{Code: 400, Message: "referenced track does not exist"}
	}
	if timeslot.StationShortname != "" {
		station := Station{TrackID: timeslot.TrackID, Shortname: timeslot.StationShortname}
		if exists, err := station.existsShortname(); err != nil {
			return gondulapi.Result{Error: err}
		} else if !exists {
			return gondulapi.Result{Code: 400, Message: "referenced station does not exist"}
		}
	}

	// TODO currently limited to single timeslot per user and track
	if ok, err := timeslot.checkAlreadyHasTimeslot(); err != nil {
		return gondulapi.Result{Error: err}
	} else if !ok {
		return gondulapi.Result{Code: 409, Message: "user currently has timeslot for this track"}
	}

	return gondulapi.Result{}
}

func (timeslot *Timeslot) checkAlreadyHasTimeslot() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM timeslots WHERE id != $1 AND user_id = $2 AND track = $3", timeslot.ID, timeslot.UserID, timeslot.TrackID)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return !hasNext, nil
}
