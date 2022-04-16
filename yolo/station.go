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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gathering/tech-online-backend/config"
	"github.com/gathering/tech-online-backend/db"
	"github.com/gathering/tech-online-backend/rest"
	"github.com/google/uuid"

	log "github.com/sirupsen/logrus"
)

// StationStatus is the station status.
type StationStatus string

// Note that a timeslot/user may be assigned to this staion somewhat orthogonal to the below states.
const (
	// StationStatusInvalid is an invalid status.
	StationStatusInvalid StationStatus = ""
	// StationStatusAvailable means the station is ready to be manually assigned by an operator.
	StationStatusAvailable StationStatus = "available"
	// StationStatusReady means the station is ready to be auto-assigned by a user.
	StationStatusReady StationStatus = "ready"
	// StationStatusDirty means the station needs a cleanup before being reused (typically after use by net track). This will trigger auto-reprovisioning if set up.
	StationStatusDirty StationStatus = "dirty"
	// StationStatusTerminated means the station has been terminated (typically after use by server track).
	StationStatusTerminated StationStatus = "terminated"
	// StationStatusProvisioning means the station has is currently undergoing provisioning. This state should be automatically changed when it's ready.
	StationStatusProvisioning StationStatus = "provisioning"
	// StationStatusMaintenance means it should not be used by any participants.
	StationStatusMaintenance StationStatus = "maintenance"
)

// DefaultDefaultStationStatus is the default value for the default state of station.
// The default state of a station decides which state it gets e.g. after getting reprovisioned.
const DefaultDefaultStationStatus = StationStatusAvailable

// Station is station.
type Station struct {
	ID            *uuid.UUID    `column:"id" json:"id"`               // Generated, required, unique
	TrackID       string        `column:"track" json:"track"`         // Required
	Shortname     string        `column:"shortname" json:"shortname"` // Required
	Name          string        `column:"name" json:"name"`
	DefaultStatus StationStatus `column:"default_status" json:"default_status"` // Required
	Status        StationStatus `column:"status" json:"status"`                 // Required
	Credentials   string        `column:"credentials" json:"credentials"`       // Host, port, password, etc. (typically hidden)
	Notes         string        `column:"notes" json:"notes"`                   // Misc. notes
	TimeslotID    string        `column:"timeslot" json:"timeslot"`             // Timeslot currently assigned to this station, if any
}

// Stations is a list of stations.
type Stations []*Station

// StationProvisionRequest is a request to allocate a new station for the specified track, if the track supports it.
type StationProvisionRequest struct {
}

// StationTerminateRequest is a request to destroy a station for the specified track, if the track supports it.
type StationTerminateRequest struct {
}

type serverCreateStationRequest struct {
	Username string `json:"username"`
	UID      string `json:"uid"`
	TaskType string `json:"task_type"`
}

type serverCreateStationResponse struct {
	ID              int    `json:"id"`
	FQDN            string `json:"fqdn"`
	Zone            string `json:"zone"`
	Username        string `json:"orc_vm_username"`
	Password        string `json:"orc_vm_password"`
	IPv4Address     string `json:"public_ipv4"`
	IPv6Address     string `json:"public_ipv6"`
	SSHPort         int    `json:"ssh_port"`
	VLANID          int    `json:"vlan_id"`
	VLANIPv4Address string `json:"vlan_ip"`
}

func init() {
	rest.AddHandler("/stations/", "^$", func() interface{} { return &Stations{} })
	rest.AddHandler("/station/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Station{} })
	rest.AddHandler("/track/", "^(?P<track_id>[^/]+)/provision-station/$", func() interface{} { return &StationProvisionRequest{} })
	rest.AddHandler("/station/", "^(?P<id>[^/]+)/terminate/$", func() interface{} { return &StationTerminateRequest{} })
}

// Get gets multiple stations.
func (stations *Stations) Get(request *rest.Request) rest.Result {
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
	if defaultStatus, ok := request.QueryArgs["default-status"]; ok {
		whereArgs = append(whereArgs, "default_status", "=", defaultStatus)
	}
	if timeslotID, ok := request.QueryArgs["timeslot"]; ok {
		whereArgs = append(whereArgs, "timeslot", "=", timeslotID)
	}

	// Fetch stations to TMP list
	tmpStations := make(Stations, 0)
	dbResult := db.SelectMany(&tmpStations, "stations", whereArgs...)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}

	// Allow all info if operator/admin
	if request.AccessToken.GetRole() == rest.RoleOperator && request.AccessToken.GetRole() == rest.RoleAdmin {
		*stations = tmpStations
		return rest.Result{}
	}

	// Hide credentials and timeslot if not assigned to self through timeslot
	for _, station := range tmpStations {
		credentials := station.Credentials
		station.Credentials = ""
		requestUserID := request.AccessToken.OwnerUserID
		if requestUserID != nil && station.TimeslotID != "" {
			var timeslot Timeslot
			timeslotDBResult := db.Select(&timeslot, "timeslots",
				"id", "=", station.TimeslotID,
				"user", "=", requestUserID,
			)
			if timeslotDBResult.IsFailed() {
				return rest.Result{Code: 500, Error: timeslotDBResult.Error}
			}
			if timeslotDBResult.IsSuccess() {
				station.Credentials = credentials
			}
		}

		*stations = append(*stations, station)
	}
	return rest.Result{}
}

// Get gets a single station.
func (station *Station) Get(request *rest.Request) rest.Result {
	// Check params
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Fetch stations to TMP object
	var tmpStation Station
	dbResult := db.Select(&tmpStation, "stations", "id", "=", id)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	if !dbResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "not found"}
	}

	// Allow all info if operator/admin
	if request.AccessToken.GetRole() == rest.RoleOperator && request.AccessToken.GetRole() == rest.RoleAdmin {
		*station = tmpStation
		return rest.Result{}
	}

	// Hide credentials if not the active user
	credentials := station.Credentials
	station.Credentials = ""
	requestUserID := request.AccessToken.OwnerUserID
	if requestUserID != nil && station.TimeslotID != "" {
		var timeslot Timeslot
		timeslotDBResult := db.Select(&timeslot, "timeslots",
			"id", "=", station.TimeslotID,
			"user", "=", requestUserID,
		)
		if timeslotDBResult.IsFailed() {
			return rest.Result{Code: 500, Error: timeslotDBResult.Error}
		}
		if timeslotDBResult.IsSuccess() {
			station.Credentials = credentials
		}
	}
	return rest.Result{}
}

// Post creates a new station.
func (station *Station) Post(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.UnauthorizedResult(*request.AccessToken)
	}

	// Make ID
	if station.ID == nil {
		newID := uuid.New()
		station.ID = &newID
	}

	// Validate
	if result := station.validate(); !result.IsOk() {
		return result
	}

	// Create and redirect
	result := station.create()
	if !result.IsOk() {
		return result
	}
	result.Code = 201
	result.Location = fmt.Sprintf("%v/station/%v/", config.Config.SitePrefix, station.ID)
	return result
}

// Put updates a station.
func (station *Station) Put(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleAdmin && request.AccessToken.GetRole() != rest.RoleRunner {
		return rest.UnauthorizedResult(*request.AccessToken)
	}

	// Check params
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists || rawID == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}
	id, uuidErr := uuid.Parse(rawID)
	if uuidErr != nil {
		return rest.Result{Code: 400, Message: "invalid ID"}
	}

	// Validate
	if *station.ID != id {
		return rest.Result{Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := station.validate(); !result.IsOk() {
		return result
	}

	// Create or update
	return station.createOrUpdate()
}

// Delete deletes a station.
func (station *Station) Delete(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.UnauthorizedResult(*request.AccessToken)
	}

	// Check params
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists || rawID == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}
	id, uuidErr := uuid.Parse(rawID)
	if uuidErr != nil {
		return rest.Result{Code: 400, Message: "invalid ID"}
	}

	// Check if exists
	station.ID = &id
	exists, err := station.exists()
	if err != nil {
		return rest.Result{Code: 500, Error: err}
	}
	if !exists {
		return rest.Result{Code: 404, Message: "not found"}
	}

	// Delete
	dbResult := db.Delete("stations", "id", "=", station.ID)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (station *Station) create() rest.Result {
	if exists, err := station.exists(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if exists {
		return rest.Result{Code: 409, Message: "duplicate"}
	}

	dbResult := db.Insert("stations", station)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (station *Station) createOrUpdate() rest.Result {
	exists, existsErr := station.exists()
	if existsErr != nil {
		return rest.Result{Code: 500, Error: existsErr}
	}

	var dbResult db.Result
	if exists {
		dbResult = db.Update("stations", station, "id", "=", station.ID)
	} else {
		dbResult = db.Insert("stations", station)
	}
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (station *Station) exists() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE id = $1", station.ID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (station *Station) existsShortname() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE track = $1 AND shortname = $2", station.TrackID, station.Shortname)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (station *Station) validate() rest.Result {
	switch {
	case station.ID == nil:
		return rest.Result{Code: 400, Message: "missing ID"}
	case station.TrackID == "":
		return rest.Result{Code: 400, Message: "missing track ID"}
	case !station.validateStatus():
		return rest.Result{Code: 400, Message: "missing or invalid default status or status"}
	}

	if exists, err := station.anotherExistsWithTrackShortname(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if exists {
		return rest.Result{Code: 409, Message: "combination of track and shortname already exists"}
	}

	track := Track{ID: station.TrackID}
	if exists, err := track.exists(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if !exists {
		return rest.Result{Code: 400, Message: "referenced track does not exist"}
	}

	if station.TimeslotID != "" {
		timeslotID, timeslotIDErr := uuid.Parse(station.TimeslotID)
		if timeslotIDErr != nil {
			return rest.Result{Code: 400, Message: "invalid timeslot ID"}
		}
		timeslot := Timeslot{ID: &timeslotID}
		if exists, err := timeslot.existsWithTrack(station.TrackID); err != nil {
			return rest.Result{Code: 500, Error: err}
		} else if !exists {
			return rest.Result{Code: 400, Message: "referenced timeslot does not exist or has wrong track type"}
		}
	}

	if station.TimeslotID != "" {
		if exists, err := station.anotherExistsWithTimeslot(); err != nil {
			return rest.Result{Code: 500, Error: err}
		} else if exists {
			return rest.Result{Code: 400, Message: "another station is already bound to the referenced timeslot"}
		}
	}

	return rest.Result{}
}

func (station *Station) validateStatus() bool {
	return validateStationStatus(station.DefaultStatus) && validateStationStatus(station.Status)
}

func validateStationStatus(status StationStatus) bool {
	switch status {
	case StationStatusAvailable:
		fallthrough
	case StationStatusReady:
		fallthrough
	case StationStatusDirty:
		fallthrough
	case StationStatusTerminated:
		fallthrough
	case StationStatusProvisioning:
		fallthrough
	case StationStatusMaintenance:
		return true
	default:
		return false
	}
}

func (station *Station) anotherExistsWithTrackShortname() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE id != $1 AND track = $2 AND shortname = $3", station.ID, station.TrackID, station.Shortname)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (station *Station) anotherExistsWithTimeslot() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE id != $1 AND timeslot = $2", station.ID, station.TimeslotID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

// Post attempts to manually create a new station, if the track supports it.
func (createRequest *StationProvisionRequest) Post(request *rest.Request) rest.Result {
	trackID, trackIDExists := request.PathArgs["track_id"]
	if !trackIDExists || trackID == "" {
		return rest.Result{Code: 400, Message: "missing track ID"}
	}

	var station Station
	return station.Provision(trackID)
}

// Provision attempts to allocate a station, if the track supports it.
// The receiver station will get overwritten with the created station,
// plus the result will contain the location of the newly created station.
// The status will be "maintenance".
func (station *Station) Provision(trackID string) rest.Result {
	// Load track
	var track Track
	dbResult := db.Select(&track, "tracks", "id", "=", trackID)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	if !dbResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "track not found"}
	}

	// Check if track type supports it and if the config is present
	if track.Type != trackTypeServer {
		return rest.Result{Code: 400, Message: "track type does not support dynamic stations"}
	}
	trackConfig, trackConfigOk := config.Config.ServerTracks[trackID]
	if !trackConfigOk || trackConfig.BaseURL == "" {
		return rest.Result{Code: 400, Message: "track is not configured for dynamic stations"}
	}

	// Check limit, excluding terminated ones
	maxStations := trackConfig.MaxInstancesHard
	if maxStations > 0 {
		currentRow := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE track = $1 AND status != $2", track.ID, StationStatusTerminated)
		var count int
		currentRowErr := currentRow.Scan(&count)
		if currentRowErr != nil {
			return rest.Result{Code: 500, Error: currentRowErr}
		}
		if count+1 > maxStations {
			return rest.Result{Code: 400, Message: "Too many active stations for dynamic track"}
		}
	}

	// Call station service
	serviceURL := trackConfig.BaseURL + "/api/entry/new"
	serviceRequestData := serverCreateStationRequest{
		Username: "tech",
		UID:      "techo",
		TaskType: trackConfig.TaskType,
	}
	requestJSON, requestJSONError := json.Marshal(serviceRequestData)
	if requestJSONError != nil {
		return rest.Result{Code: 500, Error: requestJSONError}
	}
	serviceRequest, serviceRequestErr := http.NewRequest("POST", serviceURL, bytes.NewBuffer(requestJSON))
	if serviceRequestErr != nil {
		return rest.Result{Code: 500, Error: serviceRequestErr}
	}
	serviceRequest.SetBasicAuth(trackConfig.AuthUsername, trackConfig.AuthPassword)
	serviceRequest.Header.Set("Content-Type", "application/json")
	serviceClient := &http.Client{}
	serviceResponse, serviceResponseErr := serviceClient.Do(serviceRequest)
	if serviceResponseErr != nil {
		return rest.Result{Code: 500, Error: serviceResponseErr}
	}
	defer serviceResponse.Body.Close()
	if serviceResponse.StatusCode < 200 || serviceResponse.StatusCode > 299 {
		return rest.Result{Code: 500, Error: fmt.Errorf("response contained non-2XX status: %v", serviceResponse.Status)}
	}
	serviceResponseBody, serviceResponseBodyErr := ioutil.ReadAll(serviceResponse.Body)
	if serviceResponseBodyErr != nil {
		return rest.Result{Code: 500, Error: serviceResponseBodyErr}
	}
	var responseData serverCreateStationResponse
	if err := json.Unmarshal(serviceResponseBody, &responseData); err != nil {
		return rest.Result{Code: 500, Error: err}
	}
	log.Tracef("VM service created new instance: %v", responseData.ID)

	// Create station
	newID := uuid.New()
	station.ID = &newID
	station.TrackID = trackID
	station.Shortname = strconv.Itoa(responseData.ID)
	station.Name = fmt.Sprintf("Station #%v", responseData.ID)
	station.Status = StationStatusMaintenance
	// Markdown
	station.Credentials = fmt.Sprintf("**Username**: %v\n\n**Password**: %v\n\n**Public address (IPv4)**: %v\n\n**Public address (IPv6)**: %v\n\n**SSH port**: %v",
		responseData.Username, responseData.Password, responseData.IPv4Address, responseData.IPv6Address, responseData.SSHPort)
	// Markdown
	station.Notes = fmt.Sprintf("**FQDN**: %v\n\n**Zone**: %v\n\n**VLAN ID**: %v\n\n**VLAN Address (IPv4)**: %v\n\nNote that the station may take a few minutes to start before you can connect.",
		responseData.FQDN, responseData.Zone, responseData.VLANID, responseData.VLANIPv4Address)
	if result := station.validate(); !result.IsOk() {
		return result
	}

	result := station.create()
	if !result.IsOk() {
		return result
	}

	result.Code = 201
	result.Location = fmt.Sprintf("%s/station/%s/", config.Config.SitePrefix, station.ID)
	return result
}

// Post attempts to manually destroy a station, if the track supports it.
func (destroyRequest *StationTerminateRequest) Post(request *rest.Request) rest.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Get station
	var station Station
	stationDBResult := db.Select(&station, "stations", "id", "=", id)
	if stationDBResult.IsFailed() {
		return rest.Result{Code: 500, Error: stationDBResult.Error}
	}
	if !stationDBResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "not found"}
	}

	return station.Terminate()
}

// Terminate attempts to destroy a station, if the track supports it.
// The receiver station should already be loaded and exist in the database.
func (station *Station) Terminate() rest.Result {
	// Check if already terminated
	if station.Status == StationStatusTerminated {
		return rest.Result{Code: 400, Message: "station already terminated"}
	}

	// Get track
	var track Track
	trackDBResult := db.Select(&track, "tracks", "id", "=", station.TrackID)
	if trackDBResult.IsFailed() {
		return rest.Result{Code: 500, Error: trackDBResult.Error}
	}
	if !trackDBResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "track not found"}
	}

	// Check if track type supports it and if the config is present
	if track.Type != trackTypeServer {
		return rest.Result{Code: 400, Message: "track type does not support dynamic stations"}
	}
	trackConfig, trackConfigOk := config.Config.ServerTracks[track.ID]
	if !trackConfigOk || trackConfig.BaseURL == "" {
		return rest.Result{Code: 400, Message: "track type is not configured for dynamic stations"}
	}

	// Call station service
	serviceURL := fmt.Sprintf("%v/api/entry/%v", trackConfig.BaseURL, station.Shortname)
	serviceRequest, serviceRequestErr := http.NewRequest("DELETE", serviceURL, nil)
	if serviceRequestErr != nil {
		return rest.Result{Code: 500, Error: serviceRequestErr}
	}
	serviceRequest.SetBasicAuth(trackConfig.AuthUsername, trackConfig.AuthPassword)
	serviceClient := &http.Client{}
	serviceResponse, serviceResponseErr := serviceClient.Do(serviceRequest)
	if serviceResponseErr != nil {
		return rest.Result{Code: 500, Error: serviceResponseErr}
	}
	defer serviceResponse.Body.Close()
	if serviceResponse.StatusCode < 200 || serviceResponse.StatusCode > 299 {
		return rest.Result{Code: 500, Error: fmt.Errorf("response contained non-2XX status: %v", serviceResponse.Status)}
	}
	log.Tracef("VM service destroyed instance: %v", station.ID)

	// Change state to terminated and remove any assigned timeslot
	station.Status = StationStatusTerminated
	station.TimeslotID = ""

	dbResult := db.Update("stations", station, "id", "=", station.ID)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}
