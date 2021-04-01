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

// Timeslot is a participation object used both for registration (without time and station), planning (with time) and station binding (station with this timeslot).
type Timeslot struct {
	ID        *uuid.UUID `column:"id" json:"id"`                 // Generated, required, unique
	UserToken string     `column:"user_token" json:"user_token"` // Required, secret
	TrackID   string     `column:"track" json:"track"`           // Required
	BeginTime *time.Time `column:"begin_time" json:"begin_time"` // Empty upon registration, used strictly for manual purposes
	EndTime   *time.Time `column:"end_time" json:"end_time"`     // Empty upon registration, used strictly for manual purposes
}

// TimeslotForAdmins is a timeslot, accessible only by admins.
type TimeslotForAdmins Timeslot

// Timeslots is a list of timeslots.
type Timeslots []*Timeslot

// TimeslotsForAdmins is a list of timeslots, accessible only by admins.
type TimeslotsForAdmins []*TimeslotForAdmins

// TimeslotAssignStationRequest is for finding and binding a station to the timeslot.
type TimeslotAssignStationRequest struct{}

// TimeslotFinishRequest is for requesting a timeslot to finish.
type TimeslotFinishRequest struct{}

func init() {
	receiver.AddHandler("/admin/timeslots/", "^$", func() interface{} { return &TimeslotsForAdmins{} })
	receiver.AddHandler("/timeslots/", "^$", func() interface{} { return &Timeslots{} })
	receiver.AddHandler("/admin/timeslot/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &TimeslotForAdmins{} })
	receiver.AddHandler("/timeslot/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Timeslot{} })
	receiver.AddHandler("/admin/timeslot/", "^(?P<id>[^/]+)/assign-station/$", func() interface{} { return &TimeslotAssignStationRequest{} })
	receiver.AddHandler("/admin/timeslot/", "^(?P<id>[^/]+)/finish/$", func() interface{} { return &TimeslotFinishRequest{} })
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
	if _, ok := request.QueryArgs["no-time"]; ok {
		whereArgs = append(whereArgs, "begin_time", "IS", nil)
	}
	if _, ok := request.QueryArgs["not-ended"]; ok {
		whereArgs = append(whereArgs, "end_time", ">=", now)
	}

	selectErr := db.SelectMany(timeslots, "timeslots", whereArgs...)
	if selectErr != nil {
		return gondulapi.Result{Error: selectErr}
	}

	// Check for every one if a station is assigned to it (expensive)
	_, assignedStation := request.QueryArgs["assigned-station"]
	_, notAssignedStation := request.QueryArgs["not-assigned-station"]
	if (assignedStation || notAssignedStation) && assignedStation != notAssignedStation {
		oldTimeslots := *timeslots
		*timeslots = make(TimeslotsForAdmins, 0)
		for _, timeslot := range oldTimeslots {
			exist, err := timeslot.stationsExistWithThis()
			if err != nil {
				return gondulapi.Result{Error: err}
			}
			if (exist && assignedStation) || (!exist && notAssignedStation) {
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
func (timeslot *TimeslotForAdmins) Get(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
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

// Post creates a new timeslot.
func (timeslot *TimeslotForAdmins) Post(request *gondulapi.Request) gondulapi.Result {
	if timeslot.ID == nil {
		newID := uuid.New()
		timeslot.ID = &newID
	}
	if result := timeslot.validate(); result.HasErrorOrCode() {
		return result
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
func (timeslot *TimeslotForAdmins) Put(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}

	if timeslot.ID != nil && (*timeslot.ID).String() != id {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := timeslot.validate(); result.HasErrorOrCode() {
		return result
	}

	return timeslot.createOrUpdate()
}

// Delete deletes a timeslot.
func (timeslot *TimeslotForAdmins) Delete(request *gondulapi.Request) gondulapi.Result {
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists || rawID == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}
	id, uuidError := uuid.Parse(rawID)
	if uuidError != nil {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid ID"}
	}

	timeslot.ID = &id
	exists, existsErr := timeslot.exists()
	if existsErr != nil {
		return gondulapi.Result{Failed: 1, Error: existsErr}
	}
	if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Delete("timeslots", "id", "=", timeslot.ID)
	result.Error = err
	return result
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

	// Proxy.
	timeslotForAdmins := TimeslotForAdmins(*timeslot)
	result := timeslotForAdmins.Get(request)
	if result.HasErrorOrCode() {
		return result
	}

	// Validate token.
	if timeslotForAdmins.UserToken != userToken {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid token"}
	}

	*timeslot = Timeslot(timeslotForAdmins)
	return result
}

// Post creates a new timeslot.
func (timeslot *Timeslot) Post(request *gondulapi.Request) gondulapi.Result {
	// Limit access to certain fields
	timeslot.BeginTime = nil
	timeslot.EndTime = nil

	// Proxy, no user token validation.
	timeslotForAdmins := TimeslotForAdmins(*timeslot)
	result := timeslotForAdmins.Post(request)
	*timeslot = Timeslot(timeslotForAdmins)
	return result
}

// Put updates a timeslot.
// func (timeslot *Timeslot) Put(request *gondulapi.Request) gondulapi.Result {
// 	id, idExists := request.PathArgs["id"]
// 	if !idExists || id == "" {
// 		return gondulapi.Result{Code: 400, Message: "missing ID"}
// 	}

// 	// Require user token.
// 	userToken, userTokenOk := request.QueryArgs["user-token"]
// 	if !userTokenOk {
// 		return gondulapi.Result{Code: 400, Message: "missing user token"}
// 	}
// 	if timeslot.UserToken != userToken {
// 		return gondulapi.Result{Failed: 1, Code: 400, Message: "incorrect user token"}
// 	}

// 	// Validate
// 	if timeslot.ID != nil && (*timeslot.ID).String() != id {
// 		return gondulapi.Result{Failed: 1, Code: 400, Message: "mismatch between URL and JSON IDs"}
// 	}
// 	timeslotForAdmins := TimeslotForAdmins(*timeslot)
// 	if result := timeslotForAdmins.validate(); result.HasErrorOrCode() {
// 		return result
// 	}

// 	// Get existing timeslot to check if exists and to compare token
// 	var existingTimeslot TimeslotForAdmins
// 	found, err := db.Select(&existingTimeslot, "timeslots", "id", "=", id)
// 	if err != nil {
// 		return gondulapi.Result{Error: err}
// 	}
// 	if !found {
// 		return gondulapi.Result{Code: 404, Message: "not found"}
// 	}

// 	// Verify user token (before proxy).
// 	if existingTimeslot.UserToken != userToken {
// 		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid token"}
// 	}

// 	return timeslotForAdmins.createOrUpdate()
// }

// Delete deletes a timeslot.
// func (timeslot *Timeslot) Delete(request *gondulapi.Request) gondulapi.Result {
// 	id, idExists := request.PathArgs["id"]
// 	if !idExists || id == "" {
// 		return gondulapi.Result{Code: 400, Message: "missing ID"}
// 	}

// 	// Require user token.
// 	userToken, userTokenOk := request.QueryArgs["user-token"]
// 	if !userTokenOk {
// 		return gondulapi.Result{Code: 400, Message: "missing user token"}
// 	}
// 	if timeslot.UserToken != userToken {
// 		return gondulapi.Result{Failed: 1, Code: 400, Message: "incorrect user token"}
// 	}

// 	// Get existing timeslot to check if exists and to compare token
// 	var existingTimeslot TimeslotForAdmins
// 	found, err := db.Select(&existingTimeslot, "timeslots", "id", "=", id)
// 	if err != nil {
// 		return gondulapi.Result{Error: err}
// 	}
// 	if !found {
// 		return gondulapi.Result{Code: 404, Message: "not found"}
// 	}

// 	// Verify user token (before proxy).
// 	if existingTimeslot.UserToken != userToken {
// 		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid token"}
// 	}

// 	result, err := db.Delete("timeslots", "id", "=", timeslot.ID)
// 	result.Error = err
// 	return result
// }

func (timeslot *TimeslotForAdmins) create() gondulapi.Result {
	if exists, err := timeslot.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate"}
	}

	result, err := db.Insert("timeslots", timeslot)
	result.Error = err
	return result
}

func (timeslot *TimeslotForAdmins) createOrUpdate() gondulapi.Result {
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

func (timeslot *TimeslotForAdmins) exists() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM timeslots WHERE id = $1", timeslot.ID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (timeslot *TimeslotForAdmins) existsWithTrack(trackID string) (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM timeslots WHERE id = $1 AND track = $2", timeslot.ID, trackID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (timeslot *TimeslotForAdmins) stationsExistWithThis() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE track = $1 AND timeslot = $2", timeslot.TrackID, timeslot.ID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (timeslot *TimeslotForAdmins) validate() gondulapi.Result {
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

	// Check if the user has a timeslot for the current track which hasn't ended yet.
	if has, err := timeslot.hasCurrentTimeslot(); err != nil {
		return gondulapi.Result{Error: err}
	} else if has {
		return gondulapi.Result{Code: 409, Message: "user currently has timeslot for this track"}
	}

	return gondulapi.Result{}
}

func (timeslot *TimeslotForAdmins) hasCurrentTimeslot() (bool, error) {
	now := time.Now()
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM timeslots WHERE id != $1 AND track = $2 AND user_token = $3 AND (end_time IS NULL OR end_time >= $4)", timeslot.ID, timeslot.TrackID, timeslot.UserToken, now)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

// Post attempts to find an available station to bind to the timeslot.
func (assignStationRequest *TimeslotAssignStationRequest) Post(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}

	// Get the things
	var timeslot TimeslotForAdmins
	timeslotFound, timeslotErr := db.Select(&timeslot, "timeslots", "id", "=", id)
	if timeslotErr != nil {
		return gondulapi.Result{Error: timeslotErr}
	}
	if !timeslotFound {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}
	var track Track
	trackFound, trackErr := db.Select(&track, "tracks", "id", "=", timeslot.TrackID)
	if trackErr != nil {
		return gondulapi.Result{Error: trackErr}
	}
	if !trackFound {
		return gondulapi.Result{Code: 404, Message: "track not found"}
	}

	var station *Station

	// Get all available station
	var stations Stations
	stationsErr := db.SelectMany(&stations, "stations",
		"track", "=", timeslot.TrackID,
		"status", "=", StationStatusActive,
		"timeslot", "=", "",
	)
	if stationsErr != nil {
		return gondulapi.Result{Error: stationsErr}
	}
	if len(stations) > 0 {
		station = stations[0]
	}

	// If server and no available, try to allocate one
	if track.Type == trackTypeServer && station == nil {
		// Check limit (with friendly 404s instead of 400s)
		trackConfig, trackConfigOk := gondulapi.Config.ServerTracks[track.ID]
		if !trackConfigOk || trackConfig.BaseURL == "" {
			return gondulapi.Result{Code: 404, Message: "no available stations and track not configured for dynamic stations"}
		}
		maxStations := trackConfig.MaxInstances
		if maxStations > 0 {
			currentRow := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE track = $1 AND status != $2", track.ID, StationStatusTerminated)
			var count int
			currentRowErr := currentRow.Scan(&count)
			if currentRowErr != nil {
				return gondulapi.Result{Error: currentRowErr}
			}
			if count+1 > maxStations {
				return gondulapi.Result{Code: 404, Message: "no available stations and limit for dynamic stations reached"}
			}
		}

		// Allocate one
		station = &Station{}
		if result := station.Provision(track.ID); result.HasErrorOrCode() {
			return result
		}
	}

	// Check if an available station was found or created
	if station == nil {
		return gondulapi.Result{Code: 404, Message: "no available stations"}
	}

	// Take station and save
	station.TimeslotID = timeslot.ID.String()
	if result := station.createOrUpdate(); result.HasErrorOrCode() {
		return result
	}

	// Update begin and end times and save
	beginTime := time.Now()
	timeslot.BeginTime = &beginTime
	endTime := time.Now().AddDate(1000, 0, 0) // +1000 years
	timeslot.EndTime = &endTime
	if result := timeslot.createOrUpdate(); result.HasErrorOrCode() {
		return result
	}

	return gondulapi.Result{Code: 303, Location: fmt.Sprintf("%v/station/%v/", gondulapi.Config.SitePrefix, station.ID)}
}

// Post finishes a timeslot.
func (finishRequest *TimeslotFinishRequest) Post(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}

	// Get the things
	var timeslot TimeslotForAdmins
	timeslotFound, timeslotErr := db.Select(&timeslot, "timeslots", "id", "=", id)
	if timeslotErr != nil {
		return gondulapi.Result{Error: timeslotErr}
	}
	if !timeslotFound {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}
	var track Track
	trackFound, trackErr := db.Select(&track, "tracks", "id", "=", timeslot.TrackID)
	if trackErr != nil {
		return gondulapi.Result{Error: trackErr}
	}
	if !trackFound {
		return gondulapi.Result{Code: 404, Message: "track not found"}
	}
	var station Station
	stationFound, stationErr := db.Select(&station, "stations", "timeslot", "=", id)
	if stationErr != nil {
		return gondulapi.Result{Error: stationErr}
	}
	if !stationFound {
		return gondulapi.Result{Code: 400, Message: "no station assigned to this timeslot"}
	}

	// Check stuff
	if station.TrackID != track.ID {
		return gondulapi.Result{Code: 400, Message: "mismatch between timeslot track and assigned station track (this shouldn't happen)"}
	}

	// Update end time
	now := time.Now()
	timeslot.EndTime = &now
	if timeslot.BeginTime == nil || timeslot.BeginTime.After(*timeslot.EndTime) {
		timeslot.BeginTime = &now
	}

	// Handle station according to track type
	station.TimeslotID = ""
	if track.Type == trackTypeNet {
		station.Status = StationStatusDirty
	} else if track.Type == trackTypeServer {
		if result := station.Terminate(); result.HasErrorOrCode() {
			return result
		}
	} else {
		return gondulapi.Result{Code: 400, Message: "unknown track type (this shouldn't happen)"}
	}

	// Save timeslot and station
	if result := timeslot.createOrUpdate(); result.HasErrorOrCode() {
		return result
	}
	if result := station.createOrUpdate(); result.HasErrorOrCode() {
		return result
	}

	return gondulapi.Result{}
}
