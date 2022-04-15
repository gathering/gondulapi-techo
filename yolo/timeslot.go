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

// Timeslots is a list of timeslots.
type Timeslots []*Timeslot

// TimeslotBeginRequest is for finding and binding a station to the timeslot.
type TimeslotBeginRequest struct{}

// TimeslotEndRequest is for requesting a timeslot to finish.
type TimeslotEndRequest struct{}

func init() {
	rest.AddHandler("/timeslots/", "^$", func() interface{} { return &Timeslots{} })
	rest.AddHandler("/timeslot/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Timeslot{} })
	rest.AddHandler("/timeslot/", "^(?P<id>[^/]+)/begin/$", func() interface{} { return &TimeslotBeginRequest{} })
	rest.AddHandler("/timeslot/", "^(?P<id>[^/]+)/end/$", func() interface{} { return &TimeslotEndRequest{} })
}

// Get gets multiple timeslots.
func (timeslots *Timeslots) Get(request *rest.Request) rest.Result {
	// Check params and prep filtering
	now := time.Now()
	var whereArgs []interface{}
	if userID, ok := request.QueryArgs["user-id"]; ok {
		whereArgs = append(whereArgs, "user_id", "=", userID)
	}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}

	// Find
	dbResult := db.SelectMany(timeslots, "timeslots", whereArgs...)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}

	// If not operator/admin, hide all non-self-assigned
	if request.AccessToken.GetRole() != rest.RoleOperator && request.AccessToken.GetRole() != rest.RoleAdmin {
		oldTimeslots := *timeslots
		*timeslots = make(Timeslots, 0)
		requestUserID := request.AccessToken.OwnerUserID
		if requestUserID == nil {
			// No access, just leave now
			return rest.Result{}
		}
		for _, timeslot := range oldTimeslots {
			if timeslot.UserID == requestUserID {
				*timeslots = append(*timeslots, timeslot)
			}
		}
	}

	// Post-fetch filtering (easy but expensive to do here, hard to do with current DB layer)
	_, notEnded := request.QueryArgs["not-ended"]
	_, assignedStation := request.QueryArgs["assigned-station"]
	_, notAssignedStation := request.QueryArgs["not-assigned-station"]
	if notEnded || assignedStation || notAssignedStation {
		oldTimeslots := *timeslots
		*timeslots = make(Timeslots, 0)
		for _, timeslot := range oldTimeslots {
			// TODO optimize
			stationsExist, err := timeslot.isActiveWithStation()
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

// Get gets a single timeslot.
func (timeslot *Timeslot) Get(request *rest.Request) rest.Result {
	// Check params
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Get
	dbResult := db.Select(timeslot, "timeslots", "id", "=", id)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	if !dbResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "not found"}
	}

	// Only show if operator/admin or if self-assigned
	if request.AccessToken.GetRole() != rest.RoleOperator && request.AccessToken.GetRole() != rest.RoleAdmin {
		if request.AccessToken.OwnerUserID != timeslot.UserID {
			return rest.Result{Code: 403, Message: "Permission denied"}
		}
	}

	return rest.Result{}
}

// Post creates a new timeslot.
func (timeslot *Timeslot) Post(request *rest.Request) rest.Result {
	// Check params
	if timeslot.ID == nil {
		newID := uuid.New()
		timeslot.ID = &newID
	}

	// Validate
	if result := timeslot.validate(); !result.IsOk() {
		return result
	}

	// Only allow if operator/admin or if self-assigned
	if request.AccessToken.GetRole() != rest.RoleOperator && request.AccessToken.GetRole() != rest.RoleAdmin {
		if request.AccessToken.OwnerUserID == timeslot.UserID {
			// Limit access to certain fields if self-assigned and not operator/admin
			timeslot.BeginTime = nil
			timeslot.EndTime = nil
		} else {
			return rest.Result{Code: 403, Message: "Permission denied"}
		}
	}

	// Create and redirect
	result := timeslot.create()
	if !result.IsOk() {
		return result
	}
	result.Code = 201
	result.Location = fmt.Sprintf("%v/timeslot/%v/", config.Config.SitePrefix, timeslot.ID)
	return result
}

// Put updates a timeslot.
func (timeslot *Timeslot) Put(request *rest.Request) rest.Result {
	// Check perms, only operators/admins may change existing ones
	if request.AccessToken.GetRole() != rest.RoleOperator && request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.Result{Code: 403, Message: "Permission denied"}
	}

	// Check params
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Validate
	if timeslot.ID != nil && (*timeslot.ID).String() != id {
		return rest.Result{Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := timeslot.validate(); !result.IsOk() {
		return result
	}

	// Update or create
	return timeslot.createOrUpdate()
}

// Delete deletes a timeslot.
func (timeslot *Timeslot) Delete(request *rest.Request) rest.Result {
	// Check perms, only operators/admins may change existing ones
	if request.AccessToken.GetRole() != rest.RoleOperator && request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.Result{Code: 403, Message: "Permission denied"}
	}

	// Check params
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists || rawID == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}
	id, uuidError := uuid.Parse(rawID)
	if uuidError != nil {
		return rest.Result{Code: 400, Message: "invalid ID"}
	}

	// Check if it exists
	timeslot.ID = &id
	exists, existsErr := timeslot.exists()
	if existsErr != nil {
		return rest.Result{Code: 500, Error: existsErr}
	}
	if !exists {
		return rest.Result{Code: 404, Message: "not found"}
	}

	// Delete it
	dbResult := db.Delete("timeslots", "id", "=", timeslot.ID)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (timeslot *Timeslot) create() rest.Result {
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

func (timeslot *Timeslot) createOrUpdate() rest.Result {
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

func (timeslot *Timeslot) exists() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM timeslots WHERE id = $1", timeslot.ID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (timeslot *Timeslot) existsWithTrack(trackID string) (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM timeslots WHERE id = $1 AND track = $2", timeslot.ID, trackID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (timeslot *Timeslot) isActiveWithStation() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE track = $1 AND timeslot = $2", timeslot.TrackID, timeslot.ID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (timeslot *Timeslot) validate() rest.Result {
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

	// Check if the user has a timeslot for the current track which hasn't ended yet
	if has, err := timeslot.userHasAnotherUnfinishedTimeslot(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if has {
		return rest.Result{Code: 409, Message: "user currently has timeslot for this track"}
	}

	return rest.Result{}
}

// Check if the user has another non-ended timeslot for the current track.
func (timeslot *Timeslot) userHasAnotherUnfinishedTimeslot() (bool, error) {
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
// It allows users to automatically get assigned to a "ready" net-track station,
// or a server-track station if below the soft limit.
func (beginRequest *TimeslotBeginRequest) Post(request *rest.Request) rest.Result {
	// Check params
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "Missing ID"}
	}

	// Get timeslot and track
	var timeslot Timeslot
	timeslotDBResult := db.Select(&timeslot, "timeslots", "id", "=", id)
	if timeslotDBResult.IsFailed() {
		return rest.Result{Code: 500, Error: timeslotDBResult.Error}
	}
	if !timeslotDBResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "Not found"}
	}
	var track Track
	trackDBResult := db.Select(&track, "tracks", "id", "=", timeslot.TrackID)
	if trackDBResult.IsFailed() {
		return rest.Result{Code: 500, Error: trackDBResult.Error}
	}
	if !trackDBResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "Track not found"}
	}

	// Check perms
	if request.AccessToken.GetRole() != rest.RoleOperator && request.AccessToken.GetRole() != rest.RoleAdmin && request.AccessToken.OwnerUserID != timeslot.UserID {
		return rest.Result{Code: 403, Message: "Permission denied"}
	}

	// Find all ready/available stations
	var unboundStations Stations
	unboundStationsDBResult := db.SelectMany(&unboundStations, "stations",
		"track", "=", timeslot.TrackID,
		"timeslot", "=", "",
	)
	if unboundStationsDBResult.IsFailed() {
		return rest.Result{Code: 500, Error: unboundStationsDBResult.Error}
	}
	var choosableStations Stations
	for _, station := range unboundStations {
		if station.Status == StationStatusReady {
			choosableStations = append(choosableStations, station)
		} else if station.Status == StationStatusAvailable && (request.AccessToken.GetRole() == rest.RoleOperator || request.AccessToken.GetRole() == rest.RoleAdmin) {
			choosableStations = append(choosableStations, station)
		}
	}

	// Pick a station if any ready/available
	var chosenStation *Station
	if len(choosableStations) > 0 {
		// TODO allow choosing using query param
		chosenStation = choosableStations[0]
	}

	// If server and no available, try to allocate one
	if track.Type == trackTypeServer && chosenStation == nil {
		// Check if dynamic provisioning enabled
		trackConfig, trackConfigOk := config.Config.ServerTracks[track.ID]
		if !trackConfigOk || trackConfig.BaseURL == "" {
			return rest.Result{Code: 404, Message: "no available stations and track not configured for dynamic stations"}
		}

		// Check current count
		currentRow := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE track = $1 AND status != $2", track.ID, StationStatusTerminated)
		var count int
		currentRowErr := currentRow.Scan(&count)
		if currentRowErr != nil {
			return rest.Result{Code: 500, Error: currentRowErr}
		}

		// Check if allowed
		if request.AccessToken.GetRole() == rest.RoleOperator || request.AccessToken.GetRole() == rest.RoleAdmin {
			if count >= trackConfig.MaxInstancesHard {
				return rest.Result{Code: 404, Message: "no available stations and hard limit for dynamic stations reached"}
			}
		} else {
			if count >= trackConfig.MaxInstancesSoft {
				return rest.Result{Code: 404, Message: "no available stations and soft limit for dynamic stations reached"}
			}
		}

		// Allocate one
		chosenStation = &Station{}
		if result := chosenStation.Provision(track.ID); !result.IsOk() {
			return result
		}
	}

	// Check if an available station was found or created
	if chosenStation == nil {
		return rest.Result{Code: 404, Message: "no available stations"}
	}

	// Update station, but keep the station status as-is
	chosenStation.TimeslotID = timeslot.ID.String()
	if result := chosenStation.createOrUpdate(); !result.IsOk() {
		return result
	}

	// Update timeslot
	// Warning: Potential race condition, but people are slow.
	beginTime := time.Now()
	timeslot.BeginTime = &beginTime
	endTime := time.Now().AddDate(1000, 0, 0) // +1000 years
	timeslot.EndTime = &endTime
	if result := timeslot.createOrUpdate(); !result.IsOk() {
		return result
	}

	return rest.Result{Code: 303, Location: fmt.Sprintf("%v/station/%v/", config.Config.SitePrefix, chosenStation.ID)}
}

// Post ends a timeslot.
// May be called by users assigned to the slot or by operators/admins.
func (endRequest *TimeslotEndRequest) Post(request *rest.Request) rest.Result {
	// Check params
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Get the things
	var timeslot Timeslot
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

	// Check perms
	if request.AccessToken.GetRole() != rest.RoleOperator && request.AccessToken.GetRole() != rest.RoleAdmin && request.AccessToken.OwnerUserID != timeslot.UserID {
		return rest.Result{Code: 403, Message: "Permission denied"}
	}

	// Validate stuff
	if station.TrackID != track.ID {
		return rest.Result{Code: 400, Message: "inconsistency between timeslot track and assigned station track (contact support)"}
	}

	// Update end time (and begin time if invalid)
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
