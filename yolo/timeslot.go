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
	"time"

	"github.com/gathering/tech-online-backend/config"
	"github.com/gathering/tech-online-backend/db"
	"github.com/gathering/tech-online-backend/rest"
	"github.com/google/uuid"
)

// Timeslot is a participation object used both for registration (without time and station), planning (with time) and station binding (station with this timeslot).
type Timeslot struct {
	ID        *uuid.UUID `column:"id" json:"id"`                 // Generated, required, unique
	UserID    *uuid.UUID `column:"user_id" json:"user_id"`       // Required
	TrackID   string     `column:"track" json:"track"`           // Required
	BeginTime *time.Time `column:"begin_time" json:"begin_time"` // Empty upon registration, used strictly for manual purposes
	EndTime   *time.Time `column:"end_time" json:"end_time"`     // Empty upon registration, used strictly for manual purposes
	Notes     string     `column:"notes" json:"notes"`           // Optional
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
	// rest.AddHandler("/admin/timeslots/", "^$", func() interface{} { return &TimeslotsForAdmins{} })
	rest.AddHandler("/timeslots/", "^$", func() interface{} { return &Timeslots{} })
	// rest.AddHandler("/admin/timeslot/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &TimeslotForAdmins{} })
	rest.AddHandler("/timeslot/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Timeslot{} })
	// rest.AddHandler("/admin/timeslot/", "^(?P<id>[^/]+)/assign-station/$", func() interface{} { return &TimeslotAssignStationRequest{} })
	// rest.AddHandler("/admin/timeslot/", "^(?P<id>[^/]+)/finish/$", func() interface{} { return &TimeslotFinishRequest{} })
}

// Get gets multiple timeslots.
func (timeslots *TimeslotsForAdmins) Get(request *rest.Request) rest.Result {
	now := time.Now()
	var whereArgs []interface{}
	if userID, ok := request.QueryArgs["user-id"]; ok {
		whereArgs = append(whereArgs, "user_id", "=", userID)
	}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}

	dbResult := db.SelectMany(timeslots, "timeslots", whereArgs...)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}

	// Post-fetch filtering (expensive and easy to do outside SQL but hard to do with DB layer)
	_, notEnded := request.QueryArgs["not-ended"]
	_, assignedStation := request.QueryArgs["assigned-station"]
	_, notAssignedStation := request.QueryArgs["not-assigned-station"]
	if notEnded || assignedStation || notAssignedStation {
		oldTimeslots := *timeslots
		*timeslots = make(TimeslotsForAdmins, 0)
		for _, timeslot := range oldTimeslots {
			stationsExist, err := timeslot.stationsExistWithThis()
			if err != nil {
				return rest.Result{Code: 500, Error: err}
			}
			if assignedStation && !stationsExist {
				continue
			}
			if notAssignedStation && stationsExist {
				continue
			}
			if notEnded && timeslot.EndTime != nil && timeslot.EndTime.Before(now) {
				continue
			}
			*timeslots = append(*timeslots, timeslot)
		}
	}

	return rest.Result{}
}

// Get gets multiple timeslots.
func (timeslots *Timeslots) Get(request *rest.Request) rest.Result {
	var whereArgs []interface{}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}

	// Require user ID.
	userID, userIDOk := request.QueryArgs["user-id"]
	if userIDOk {
		whereArgs = append(whereArgs, "user_id", "=", userID)
	} else {
		return rest.Result{Code: 400, Message: "missing user ID"}
	}

	dbResult := db.SelectMany(timeslots, "timeslots", whereArgs...)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

// Get gets a single timeslot.
func (timeslot *TimeslotForAdmins) Get(request *rest.Request) rest.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	dbResult := db.Select(timeslot, "timeslots", "id", "=", id)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	if !dbResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "not found"}
	}
	return rest.Result{}
}

// Post creates a new timeslot.
func (timeslot *TimeslotForAdmins) Post(request *rest.Request) rest.Result {
	if timeslot.ID == nil {
		newID := uuid.New()
		timeslot.ID = &newID
	}
	if result := timeslot.validate(); !result.IsOk() {
		return result
	}

	result := timeslot.create()
	if !result.IsOk() {
		return result
	}

	result.Code = 201
	result.Location = fmt.Sprintf("%v/timeslot/%v/", config.Config.SitePrefix, timeslot.ID)
	return result
}

// Put updates a timeslot.
func (timeslot *TimeslotForAdmins) Put(request *rest.Request) rest.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	if timeslot.ID != nil && (*timeslot.ID).String() != id {
		return rest.Result{Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := timeslot.validate(); !result.IsOk() {
		return result
	}

	return timeslot.createOrUpdate()
}

// Delete deletes a timeslot.
func (timeslot *TimeslotForAdmins) Delete(request *rest.Request) rest.Result {
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists || rawID == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}
	id, uuidError := uuid.Parse(rawID)
	if uuidError != nil {
		return rest.Result{Code: 400, Message: "invalid ID"}
	}

	timeslot.ID = &id
	exists, existsErr := timeslot.exists()
	if existsErr != nil {
		return rest.Result{Code: 500, Error: existsErr}
	}
	if !exists {
		return rest.Result{Code: 404, Message: "not found"}
	}

	dbResult := db.Delete("timeslots", "id", "=", timeslot.ID)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

// Get gets a single timeslot.
func (timeslot *Timeslot) Get(request *rest.Request) rest.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Require user ID.
	userStrID, userStrIDOk := request.QueryArgs["user-id"]
	if !userStrIDOk {
		return rest.Result{Code: 400, Message: "missing user ID"}
	}
	userID, userIDParseErr := uuid.Parse(userStrID)
	if userIDParseErr != nil {
		return rest.Result{Code: 400, Message: "invalid user ID"}
	}

	// Proxy.
	timeslotForAdmins := TimeslotForAdmins(*timeslot)
	result := timeslotForAdmins.Get(request)
	if !result.IsOk() {
		return result
	}

	// Validate ID.
	if *timeslotForAdmins.UserID != userID {
		return rest.Result{Code: 400, Message: "invalid ID"}
	}

	*timeslot = Timeslot(timeslotForAdmins)
	return result
}

// Post creates a new timeslot.
func (timeslot *Timeslot) Post(request *rest.Request) rest.Result {
	// Limit access to certain fields
	timeslot.BeginTime = nil
	timeslot.EndTime = nil

	// Proxy, no user ID validation.
	timeslotForAdmins := TimeslotForAdmins(*timeslot)
	result := timeslotForAdmins.Post(request)
	*timeslot = Timeslot(timeslotForAdmins)
	return result
}

func (timeslot *TimeslotForAdmins) create() rest.Result {
	if exists, err := timeslot.exists(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if exists {
		return rest.Result{Code: 409, Message: "duplicate"}
	}

	dbResult := db.Insert("timeslots", timeslot)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (timeslot *TimeslotForAdmins) createOrUpdate() rest.Result {
	exists, existsErr := timeslot.exists()
	if existsErr != nil {
		return rest.Result{Code: 500, Error: existsErr}
	}

	var dbResult db.Result
	if exists {
		dbResult = db.Update("timeslots", timeslot, "id", "=", timeslot.ID)
	} else {
		dbResult = db.Insert("timeslots", timeslot)
	}
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
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

func (timeslot *TimeslotForAdmins) validate() rest.Result {
	switch {
	case timeslot.ID == nil:
		return rest.Result{Code: 400, Message: "missing ID"}
	case timeslot.UserID == nil:
		return rest.Result{Code: 400, Message: "missing user ID"}
	case timeslot.TrackID == "":
		return rest.Result{Code: 400, Message: "missing track ID"}
	case (timeslot.BeginTime == nil) != (timeslot.EndTime == nil):
		return rest.Result{Code: 400, Message: "only begin or end time set"}
	case timeslot.BeginTime != nil && timeslot.EndTime != nil && timeslot.EndTime.Before(*timeslot.BeginTime):
		return rest.Result{Code: 400, Message: "cannot end before it begins"}
	}

	user := rest.User{ID: timeslot.UserID}
	if exists, err := user.ExistsWithID(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if !exists {
		return rest.Result{Code: 400, Message: "referenced user does not exist"}
	}
	track := Track{ID: timeslot.TrackID}
	if exists, err := track.exists(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if !exists {
		return rest.Result{Code: 400, Message: "referenced track does not exist"}
	}

	// Check if the user has a timeslot for the current track which hasn't ended yet.
	if has, err := timeslot.hasCurrentTimeslot(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if has {
		return rest.Result{Code: 409, Message: "user currently has timeslot for this track"}
	}

	return rest.Result{}
}

func (timeslot *TimeslotForAdmins) hasCurrentTimeslot() (bool, error) {
	now := time.Now()
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM timeslots WHERE id != $1 AND track = $2 AND user_id = $3 AND (end_time IS NULL OR end_time >= $4)", timeslot.ID, timeslot.TrackID, timeslot.UserID, now)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

// Post attempts to find an available station to bind to the timeslot.
func (assignStationRequest *TimeslotAssignStationRequest) Post(request *rest.Request) rest.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Get the things
	var timeslot TimeslotForAdmins
	timeslotDBResult := db.Select(&timeslot, "timeslots", "id", "=", id)
	if timeslotDBResult.IsFailed() {
		return rest.Result{Code: 500, Error: timeslotDBResult.Error}
	}
	if !timeslotDBResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "not found"}
	}
	var track Track
	trackDBResult := db.Select(&track, "tracks", "id", "=", timeslot.TrackID)
	if trackDBResult.IsFailed() {
		return rest.Result{Code: 500, Error: trackDBResult.Error}
	}
	if !trackDBResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "track not found"}
	}

	var station *Station

	// Get all available station
	var stations Stations
	stationsDBResult := db.SelectMany(&stations, "stations",
		"track", "=", timeslot.TrackID,
		"status", "=", StationStatusActive,
		"timeslot", "=", "",
	)
	if stationsDBResult.IsFailed() {
		return rest.Result{Code: 500, Error: stationsDBResult.Error}
	}
	if len(stations) > 0 {
		station = stations[0]
	}

	// If server and no available, try to allocate one
	if track.Type == trackTypeServer && station == nil {
		// Check limit (with friendly 404s instead of 400s)
		trackConfig, trackConfigOk := config.Config.ServerTracks[track.ID]
		if !trackConfigOk || trackConfig.BaseURL == "" {
			return rest.Result{Code: 404, Message: "no available stations and track not configured for dynamic stations"}
		}
		maxStations := trackConfig.MaxInstances
		if maxStations > 0 {
			currentRow := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE track = $1 AND status != $2", track.ID, StationStatusTerminated)
			var count int
			currentRowErr := currentRow.Scan(&count)
			if currentRowErr != nil {
				return rest.Result{Code: 500, Error: currentRowErr}
			}
			if count+1 > maxStations {
				return rest.Result{Code: 404, Message: "no available stations and limit for dynamic stations reached"}
			}
		}

		// Allocate one
		station = &Station{}
		if result := station.Provision(track.ID); !result.IsOk() {
			return result
		}
	}

	// Check if an available station was found or created
	if station == nil {
		return rest.Result{Code: 404, Message: "no available stations"}
	}

	// Take station and save
	station.TimeslotID = timeslot.ID.String()
	station.Status = StationStatusActive
	if result := station.createOrUpdate(); !result.IsOk() {
		return result
	}

	// Update begin and end times and save
	beginTime := time.Now()
	timeslot.BeginTime = &beginTime
	endTime := time.Now().AddDate(1000, 0, 0) // +1000 years
	timeslot.EndTime = &endTime
	if result := timeslot.createOrUpdate(); !result.IsOk() {
		return result
	}

	return rest.Result{Code: 303, Location: fmt.Sprintf("%v/station/%v/", config.Config.SitePrefix, station.ID)}
}

// Post finishes a timeslot.
func (finishRequest *TimeslotFinishRequest) Post(request *rest.Request) rest.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Get the things
	var timeslot TimeslotForAdmins
	timeslotDBResult := db.Select(&timeslot, "timeslots", "id", "=", id)
	if timeslotDBResult.IsFailed() {
		return rest.Result{Code: 500, Error: timeslotDBResult.Error}
	}
	if !timeslotDBResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "not found"}
	}
	var track Track
	trackDBResult := db.Select(&track, "tracks", "id", "=", timeslot.TrackID)
	if trackDBResult.IsFailed() {
		return rest.Result{Code: 500, Error: trackDBResult.Error}
	}
	if !trackDBResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "track not found"}
	}
	var station Station
	stationDBResult := db.Select(&station, "stations", "timeslot", "=", id)
	if stationDBResult.IsFailed() {
		return rest.Result{Code: 500, Error: stationDBResult.Error}
	}
	if !stationDBResult.IsSuccess() {
		return rest.Result{Code: 400, Message: "no station assigned to this timeslot"}
	}

	// Check stuff
	if station.TrackID != track.ID {
		return rest.Result{Code: 400, Message: "inconsistency between timeslot track and assigned station track (contact support)"}
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
		if result := station.Terminate(); !result.IsOk() {
			return result
		}
	} else {
		return rest.Result{Code: 400, Message: "unknown track type (contact support)"}
	}

	// Save timeslot and station
	if result := timeslot.createOrUpdate(); !result.IsOk() {
		return result
	}
	if result := station.createOrUpdate(); !result.IsOk() {
		return result
	}

	return rest.Result{}
}
