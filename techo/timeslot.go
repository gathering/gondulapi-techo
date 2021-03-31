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

// Timeslot is a duration a station is booked to a participant.
type Timeslot struct {
	ID               *uuid.UUID `column:"id" json:"id"`                               // Generated, required, unique
	UserToken        string     `column:"user_token" json:"user_token"`               // Required, secret
	TrackID          string     `column:"track" json:"track"`                         // Required
	StationShortname string     `column:"station_shortname" json:"station_shortname"` // May be empty until assigned
	BeginTime        *time.Time `column:"begin_time" json:"begin_time"`               // May be empty until assigned
	EndTime          *time.Time `column:"end_time" json:"end_time"`                   // May be empty until assigned
}

// Timeslots is a list of timeslots.
type Timeslots []*Timeslot

// TimeslotsForAdmins is a list of timeslots, accessible only by admins.
type TimeslotsForAdmins Timeslots

func init() {
	receiver.AddHandler("/admin/timeslots/", "^$", func() interface{} { return &TimeslotsForAdmins{} })
	receiver.AddHandler("/timeslots/", "^$", func() interface{} { return &Timeslots{} })
	receiver.AddHandler("/timeslot/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Timeslot{} })
}

// Get gets multiple timeslots.
func (timeslots *TimeslotsForAdmins) Get(request *gondulapi.Request) gondulapi.Result {
	now := time.Now()
	var whereArgs []interface{}
	if userID, ok := request.QueryArgs["user-token"]; ok {
		whereArgs = append(whereArgs, "user_token", "=", userID)
	}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}
	if stationShortname, ok := request.QueryArgs["station-shortname"]; ok {
		whereArgs = append(whereArgs, "station_shortname", "=", stationShortname)
	}
	state, stateOK := request.QueryArgs["state"]
	if stateOK && state == "active" {
		whereArgs = append(whereArgs, "begin_time", "<=", now, "end_time", ">=", now)
	}

	selectErr := db.SelectMany(timeslots, "timeslots", whereArgs...)
	if selectErr != nil {
		return gondulapi.Result{Error: selectErr}
	}

	// Continue state filtering (hard to do earlier due to query builder using AND)
	if state == "unassigned" {
		oldTimeslots := *timeslots
		*timeslots = make(TimeslotsForAdmins, 0)
		for _, timeslot := range oldTimeslots {
			if timeslot.BeginTime == nil || timeslot.EndTime == nil {
				*timeslots = append(*timeslots, timeslot)
			}
		}
	}

	return gondulapi.Result{}
}

// Get gets multiple timeslots.
func (timeslots *Timeslots) Get(request *gondulapi.Request) gondulapi.Result {
	var whereArgs []interface{}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}

	// Require user token.
	userToken, userTokenOk := request.QueryArgs["user-token"]
	if userTokenOk {
		whereArgs = append(whereArgs, "user_token", "=", userToken)
	} else {
		return gondulapi.Result{Code: 400, Message: "missing user token"}
	}

	selectErr := db.SelectMany(timeslots, "timeslots", whereArgs...)
	if selectErr != nil {
		return gondulapi.Result{Error: selectErr}
	}

	return gondulapi.Result{}
}

// Get gets a single timeslot.
func (timeslot *Timeslot) Get(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	}

	// Require user token.
	userToken, userTokenOk := request.QueryArgs["user-token"]
	if !userTokenOk {
		return gondulapi.Result{Code: 400, Message: "missing user token"}
	}

	// Find.
	found, err := db.Select(timeslot, "timeslots", "id", "=", id)
	if err != nil {
		return gondulapi.Result{Error: err}
	}
	if !found {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}

	// Validate token.
	if timeslot.UserToken != userToken {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid token"}
	}

	return gondulapi.Result{}
}

// Post creates a new timeslot.
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

	result := timeslot.create()
	if result.HasErrorOrCode() {
		return result
	}

	result.Code = 201
	result.Location = fmt.Sprintf("%v/timeslot/%v/", gondulapi.Config.SitePrefix, timeslot.ID)
	return result
}

// Put updates a timeslot.
func (timeslot *Timeslot) Put(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}

	// Require user token.
	userToken, userTokenOk := request.QueryArgs["user-token"]
	if !userTokenOk {
		return gondulapi.Result{Code: 400, Message: "missing user token"}
	}
	if timeslot.UserToken != userToken {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "incorrect user token"}
	}

	// Validate
	if timeslot.ID != nil && (*timeslot.ID).String() != id {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := timeslot.validate(); result.HasErrorOrCode() {
		return result
	}

	// Check if it exists (regardless of token).
	exists, existsErr := timeslot.exists()
	if existsErr != nil {
		return gondulapi.Result{Failed: 1, Error: existsErr}
	}
	if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	// Validate token.
	existsWithToken, existsWithTokenErr := timeslot.existsWithToken()
	if existsWithTokenErr != nil {
		return gondulapi.Result{Failed: 1, Error: existsWithTokenErr}
	}
	if !existsWithToken {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid token"}
	}

	return timeslot.createOrUpdate()
}

// Delete deletes a timeslot.
func (timeslot *Timeslot) Delete(request *gondulapi.Request) gondulapi.Result {
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists || rawID == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}
	id, uuidError := uuid.Parse(rawID)
	if uuidError != nil {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid ID"}
	}

	// Require user token.
	userToken, userTokenOk := request.QueryArgs["user-token"]
	if !userTokenOk {
		return gondulapi.Result{Code: 400, Message: "missing user token"}
	}

	// Check if it exists (regardless of token).
	timeslot.ID = &id
	exists, existsErr := timeslot.exists()
	if existsErr != nil {
		return gondulapi.Result{Failed: 1, Error: existsErr}
	}
	if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	// Validate token.
	timeslot.ID = &id
	timeslot.UserToken = userToken
	existsWithToken, existsWithTokenErr := timeslot.existsWithToken()
	if existsWithTokenErr != nil {
		return gondulapi.Result{Failed: 1, Error: existsWithTokenErr}
	}
	if !existsWithToken {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid token"}
	}

	// Delete
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

func (timeslot *Timeslot) createOrUpdate() gondulapi.Result {
	exists, existsErr := timeslot.exists()
	if existsErr != nil {
		return gondulapi.Result{Failed: 1, Error: existsErr}
	}

	if exists {
		result, err := db.Update("timeslots", timeslot, "id", "=", timeslot.ID)
		result.Error = err
		return result
	}

	result, err := db.Insert("timeslots", timeslot)
	result.Error = err
	return result
}

func (timeslot *Timeslot) exists() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM timeslots WHERE id = $1", timeslot.ID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (timeslot *Timeslot) existsWithToken() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM timeslots WHERE id = $1 AND user_token = $2", timeslot.ID, timeslot.UserToken)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (timeslot *Timeslot) validate() gondulapi.Result {
	switch {
	case timeslot.ID == nil:
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	case timeslot.UserToken == "":
		return gondulapi.Result{Code: 400, Message: "missing user token"}
	case timeslot.TrackID == "":
		return gondulapi.Result{Code: 400, Message: "missing track ID"}
	case (timeslot.BeginTime == nil) != (timeslot.EndTime == nil):
		return gondulapi.Result{Code: 400, Message: "only begin or end time set"}
	case timeslot.BeginTime != nil && timeslot.EndTime != nil && timeslot.EndTime.Before(*timeslot.BeginTime):
		return gondulapi.Result{Code: 400, Message: "cannot end before it begins"}
	}

	user := User{Token: timeslot.UserToken}
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
	// TODO should maybe allow signing up again if finished with previous ones
	if has, err := timeslot.hasTimeslot(); err != nil {
		return gondulapi.Result{Error: err}
	} else if has {
		return gondulapi.Result{Code: 409, Message: "user currently has timeslot for this track"}
	}

	return gondulapi.Result{}
}

func (timeslot *Timeslot) hasTimeslot() (bool, error) {
	// TODO allow more timeslots per user and track
	rows, err := db.DB.Query("SELECT id FROM timeslots WHERE id != $1 AND user_token = $2 AND track = $3", timeslot.ID, timeslot.UserToken, timeslot.TrackID)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}
