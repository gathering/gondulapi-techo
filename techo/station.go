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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gathering/gondulapi"
	"github.com/gathering/gondulapi/db"
	"github.com/gathering/gondulapi/receiver"
	"github.com/google/uuid"

	log "github.com/sirupsen/logrus"
)

// StationStatus is the station status.
type StationStatus string

const (
	// StationStatusActive means the station is ready to be assigned or is currently assigned.
	StationStatusActive StationStatus = "active"
	// StationStatusDirty means the station needs a cleanup before being reused (typically after use by net track).
	StationStatusDirty StationStatus = "dirty"
	// StationStatusTerminated means the station has been terminated (typically after use by server track).
	StationStatusTerminated StationStatus = "terminated"
	// StationStatusMaintenance means it's active but should not be used by any participants.
	StationStatusMaintenance StationStatus = "maintenance"
)

// Station is station.
type Station struct {
	ID          *uuid.UUID    `column:"id" json:"id"`               // Generated, required, unique
	TrackID     string        `column:"track" json:"track"`         // Required
	Shortname   string        `column:"shortname" json:"shortname"` // Required
	Name        string        `column:"name" json:"name"`
	Status      StationStatus `column:"status" json:"status"`           // Required
	Credentials string        `column:"credentials" json:"credentials"` // Host, port, password, etc. (typically hidden)
	Notes       string        `column:"notes" json:"notes"`             // Misc. notes
	TimeslotID  string        `column:"timeslot" json:"timeslot"`       // Timeslot currently assigned to this station, if any
}

// Stations is a list of stations.
type Stations []*Station

// StationsForAdmins is a list of stations, accessible only by admins.
type StationsForAdmins Stations

// StationProvisionRequest is a request to allocate a new station for the specified track, if the track supports it.
type StationProvisionRequest struct {
}

// StationTerminateRequest is a request to destroy a station for the specified track, if the track supports it.
type StationTerminateRequest struct {
}

type serverCreateStationResponse struct {
	ID             int    `json:"id"`
	FQDN           string `json:"fqdn"`
	Zone           string `json:"zone"`
	Username       string `json:"orc_vm_username"`
	Password       string `json:"orc_vm_password"`
	IPv4Address    string `json:"public_ipv4"`
	IPv6Address    string `json:"public_ipv6"`
	SSHPort        int    `json:"ssh_port"`
	VLANID         int    `json:"vlan_id"`
	VLANIPv4Subnet string `json:"vlan_ip"`
}

func init() {
	receiver.AddHandler("/stations/", "^$", func() interface{} { return &Stations{} })
	receiver.AddHandler("/station/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Station{} })
	receiver.AddHandler("/admin/stations/", "^$", func() interface{} { return &StationsForAdmins{} })
	receiver.AddHandler("/track/", "^(?P<track_id>[^/]+)/provision-station/$", func() interface{} { return &StationProvisionRequest{} })
	receiver.AddHandler("/station/", "^(?P<id>[^/]+)/terminate/$", func() interface{} { return &StationTerminateRequest{} })
}

// Get gets multiple stations.
func (stations *Stations) Get(request *gondulapi.Request) gondulapi.Result {
	// Fetch through admin endpoint (with credentials)
	stationsForAdmins := StationsForAdmins{}
	stationsForAdminsResult := stationsForAdmins.Get(request)
	if stationsForAdminsResult.HasErrorOrCode() {
		return stationsForAdminsResult
	}

	// Copy and hide credentials
	_, timeslotIDOk := request.QueryArgs["timeslot"]
	userToken, userTokenOk := request.QueryArgs["user-token"]
	for _, station := range stationsForAdmins {
		credentials := station.Credentials
		station.Credentials = ""
		// If filtering by timeslot and user token, show credentials if correct user token
		if timeslotIDOk && userTokenOk && userToken != "" && station.TimeslotID != "" {
			var timeslot Timeslot
			timeslotFound, timeslotErr := db.Select(&timeslot, "timeslots",
				"id", "=", station.TimeslotID,
				"user_token", "=", userToken,
			)
			if timeslotErr != nil {
				return gondulapi.Result{Error: timeslotErr}
			}
			if timeslotFound {
				station.Credentials = credentials
			}
		}

		*stations = append(*stations, station)
	}

	return gondulapi.Result{}
}

// Get gets multiple stations. For admins.
func (stations *StationsForAdmins) Get(request *gondulapi.Request) gondulapi.Result {
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
	if timeslotID, ok := request.QueryArgs["timeslot"]; ok {
		whereArgs = append(whereArgs, "timeslot", "=", timeslotID)
	}

	selectErr := db.SelectMany(stations, "stations", whereArgs...)
	if selectErr != nil {
		return gondulapi.Result{Error: selectErr}
	}

	return gondulapi.Result{}
}

// Get gets a single station.
func (station *Station) Get(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	}

	found, err := db.Select(station, "stations", "id", "=", id)
	if err != nil {
		return gondulapi.Result{Error: err}
	}
	if !found {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}

	// Show credentials only if a user token matching the active timeslot was provided
	userToken, userTokenOk := request.QueryArgs["user-token"]
	credentials := station.Credentials
	station.Credentials = ""
	if userTokenOk && userToken != "" && station.TimeslotID != "" {
		var timeslot Timeslot
		timeslotFound, timeslotErr := db.Select(&timeslot, "timeslots",
			"id", "=", station.TimeslotID,
			"user_token", "=", userToken,
		)
		if timeslotErr != nil {
			return gondulapi.Result{Error: timeslotErr}
		}
		if timeslotFound {
			station.Credentials = credentials
		}
	}

	return gondulapi.Result{}
}

// Post creates a new station.
func (station *Station) Post(request *gondulapi.Request) gondulapi.Result {
	if station.ID == nil {
		newID := uuid.New()
		station.ID = &newID
	}
	if result := station.validate(); result.HasErrorOrCode() {
		return result
	}

	result := station.create()
	if result.HasErrorOrCode() {
		return result
	}

	result.Code = 201
	result.Location = fmt.Sprintf("%v/station/%v/", gondulapi.Config.SitePrefix, station.ID)
	return result
}

// Put updates a station.
func (station *Station) Put(request *gondulapi.Request) gondulapi.Result {
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists || rawID == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}
	id, uuidErr := uuid.Parse(rawID)
	if uuidErr != nil {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid ID"}
	}

	if *station.ID != id {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := station.validate(); result.HasErrorOrCode() {
		return result
	}

	return station.createOrUpdate()
}

// Delete deletes a station.
func (station *Station) Delete(request *gondulapi.Request) gondulapi.Result {
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists || rawID == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}
	id, uuidErr := uuid.Parse(rawID)
	if uuidErr != nil {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid ID"}
	}

	station.ID = &id
	exists, err := station.exists()
	if err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	}
	if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Delete("stations", "id", "=", station.ID)
	result.Error = err
	return result
}

func (station *Station) create() gondulapi.Result {
	if exists, err := station.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate"}
	}

	result, err := db.Insert("stations", station)
	result.Error = err
	return result
}

func (station *Station) createOrUpdate() gondulapi.Result {
	exists, existsErr := station.exists()
	if existsErr != nil {
		return gondulapi.Result{Failed: 1, Error: existsErr}
	}

	if exists {
		result, err := db.Update("stations", station, "id", "=", station.ID)
		result.Error = err
		return result
	}

	result, err := db.Insert("stations", station)
	result.Error = err
	return result
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

func (station *Station) validate() gondulapi.Result {
	switch {
	case station.ID == nil:
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	case station.TrackID == "":
		return gondulapi.Result{Code: 400, Message: "missing track ID"}
	case !station.validateStatus():
		return gondulapi.Result{Code: 400, Message: "missing or invalid status"}
	}

	if exists, err := station.existsTrackShortname(); err != nil {
		return gondulapi.Result{Error: err}
	} else if exists {
		return gondulapi.Result{Code: 409, Message: "combination of track and shortname already exists"}
	}

	track := Track{ID: station.TrackID}
	if exists, err := track.exists(); err != nil {
		return gondulapi.Result{Error: err}
	} else if !exists {
		return gondulapi.Result{Code: 400, Message: "referenced track does not exist"}
	}

	if station.TimeslotID != "" {
		timeslotID, timeslotIDErr := uuid.Parse(station.TimeslotID)
		if timeslotIDErr != nil {
			return gondulapi.Result{Code: 400, Message: "invalid timeslot ID"}
		}
		timeslot := TimeslotForAdmins{ID: &timeslotID}
		if exists, err := timeslot.existsWithTrack(station.TrackID); err != nil {
			return gondulapi.Result{Error: err}
		} else if !exists {
			return gondulapi.Result{Code: 400, Message: "referenced timeslot does not exist or has wrong track type"}
		}
	}

	if station.TimeslotID != "" {
		if exists, err := station.existsTimeslot(); err != nil {
			return gondulapi.Result{Error: err}
		} else if exists {
			return gondulapi.Result{Code: 400, Message: "another station is already bound to the referenced timeslot"}
		}
	}

	return gondulapi.Result{}
}

func (station *Station) validateStatus() bool {
	switch station.Status {
	case StationStatusActive:
		fallthrough
	case StationStatusDirty:
		fallthrough
	case StationStatusTerminated:
		fallthrough
	case StationStatusMaintenance:
		return true
	default:
		return false
	}
}

func (station *Station) existsTrackShortname() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE id != $1 AND track = $2 AND shortname = $3", station.ID, station.TrackID, station.Shortname)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (station *Station) existsTimeslot() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE id != $1 AND timeslot = $2", station.ID, station.TimeslotID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

// Post attempts to manually create a new station, if the track supports it.
func (createRequest *StationProvisionRequest) Post(request *gondulapi.Request) gondulapi.Result {
	trackID, trackIDExists := request.PathArgs["track_id"]
	if !trackIDExists || trackID == "" {
		return gondulapi.Result{Code: 400, Message: "missing track ID"}
	}

	var station Station
	return station.Provision(trackID)
}

// Provision attempts to allocate a station, if the track supports it.
// The receiver station will get overwritten with the created station,
// plus the result will contain the location of the newly created station.
// The status will be "maintenance".
func (station *Station) Provision(trackID string) gondulapi.Result {
	// Load track
	var track Track
	found, selectErr := db.Select(&track, "tracks", "id", "=", trackID)
	if selectErr != nil {
		return gondulapi.Result{Error: selectErr}
	}
	if !found {
		return gondulapi.Result{Code: 404, Message: "track not found"}
	}

	// Check if track type supports it and if the config is present
	if track.Type != trackTypeServer {
		return gondulapi.Result{Code: 400, Message: "track type does not support dynamic stations"}
	}
	trackConfig, trackConfigOk := gondulapi.Config.ServerTracks[trackID]
	if !trackConfigOk || trackConfig.BaseURL == "" {
		return gondulapi.Result{Code: 400, Message: "track is not configured for dynamic stations"}
	}

	// Check limit, excluding terminated ones
	maxStations := trackConfig.MaxInstances
	if maxStations > 0 {
		currentRow := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE track = $1 AND status != $2", track.ID, StationStatusTerminated)
		var count int
		currentRowErr := currentRow.Scan(&count)
		if currentRowErr != nil {
			return gondulapi.Result{Error: currentRowErr}
		}
		if count+1 > maxStations {
			return gondulapi.Result{Code: 400, Message: "too many current stations for dynamic track"}
		}
	}

	// Call station service
	serviceURL := trackConfig.BaseURL + "/api/entry/new"
	serviceBody := []byte(`{"username":"tech","uid":"gondulapi","task_type":"1"}`)
	serviceRequest, serviceRequestErr := http.NewRequest("POST", serviceURL, bytes.NewBuffer(serviceBody))
	if serviceRequestErr != nil {
		return gondulapi.Result{Error: serviceRequestErr}
	}
	serviceRequest.SetBasicAuth(trackConfig.AuthUsername, trackConfig.AuthPassword)
	serviceRequest.Header.Set("Content-Type", "application/json")
	serviceClient := &http.Client{}
	serviceResponse, serviceResponseErr := serviceClient.Do(serviceRequest)
	if serviceResponseErr != nil {
		return gondulapi.Result{Error: serviceResponseErr}
	}
	defer serviceResponse.Body.Close()
	if serviceResponse.StatusCode < 200 || serviceResponse.StatusCode > 299 {
		return gondulapi.Result{Error: fmt.Errorf("response contained non-2XX status: %v", serviceResponse.Status)}
	}
	serviceResponseBody, serviceResponseBodyErr := ioutil.ReadAll(serviceResponse.Body)
	if serviceResponseBodyErr != nil {
		return gondulapi.Result{Error: serviceResponseBodyErr}
	}
	var serviceData serverCreateStationResponse
	if err := json.Unmarshal(serviceResponseBody, &serviceData); err != nil {
		return gondulapi.Result{Error: err}
	}
	log.Tracef("VM service created new instance: %v", serviceData.ID)

	// Create station
	newID := uuid.New()
	station.ID = &newID
	station.TrackID = trackID
	station.Shortname = strconv.Itoa(serviceData.ID)
	station.Name = serviceData.FQDN
	station.Status = StationStatusMaintenance
	station.Credentials = fmt.Sprintf("Username: %v\nPassword: %v\nPublic IPv4 address: %v\nPublic IPv6 address: %v\nSSH port: %v",
		serviceData.Username, serviceData.Password, serviceData.IPv4Address, serviceData.IPv6Address, serviceData.SSHPort)
	station.Notes = fmt.Sprintf("FQDN: %v\nZone: %v\nVLAN ID: %v\nVLAN IPv4 Subnet: %v",
		serviceData.FQDN, serviceData.Zone, serviceData.VLANID, serviceData.VLANIPv4Subnet)
	if result := station.validate(); result.HasErrorOrCode() {
		return result
	}

	result := station.create()
	if result.HasErrorOrCode() {
		return result
	}

	result.Code = 201
	result.Location = fmt.Sprintf("%s/station/%s/", gondulapi.Config.SitePrefix, station.ID)
	return result
}

// Post attempts to manually destroy a station, if the track supports it.
func (destroyRequest *StationTerminateRequest) Post(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	}

	// Get station
	var station Station
	stationFound, stationSelectErr := db.Select(&station, "stations", "id", "=", id)
	if stationSelectErr != nil {
		return gondulapi.Result{Error: stationSelectErr}
	}
	if !stationFound {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}

	return station.Terminate()
}

// Terminate attempts to destroy a station, if the track supports it.
// The receiver station should already be loaded and exist in the database.
func (station *Station) Terminate() gondulapi.Result {
	// Check if already terminated
	if station.Status == StationStatusTerminated {
		return gondulapi.Result{Code: 400, Message: "station already terminated"}
	}

	// Get track
	var track Track
	trackFound, trackSelectErr := db.Select(&track, "tracks", "id", "=", station.TrackID)
	if trackSelectErr != nil {
		return gondulapi.Result{Error: trackSelectErr}
	}
	if !trackFound {
		return gondulapi.Result{Code: 404, Message: "track not found"}
	}

	// Check if track type supports it and if the config is present
	if track.Type != trackTypeServer {
		return gondulapi.Result{Code: 400, Message: "track type does not support dynamic stations"}
	}
	trackConfig, trackConfigOk := gondulapi.Config.ServerTracks[track.ID]
	if !trackConfigOk || trackConfig.BaseURL == "" {
		return gondulapi.Result{Code: 400, Message: "track type is not configured for dynamic stations"}
	}

	// Call station service
	serviceURL := fmt.Sprintf("%v/api/entry/%v", trackConfig.BaseURL, station.Shortname)
	serviceRequest, serviceRequestErr := http.NewRequest("DELETE", serviceURL, nil)
	if serviceRequestErr != nil {
		return gondulapi.Result{Error: serviceRequestErr}
	}
	serviceRequest.SetBasicAuth(trackConfig.AuthUsername, trackConfig.AuthPassword)
	serviceClient := &http.Client{}
	serviceResponse, serviceResponseErr := serviceClient.Do(serviceRequest)
	if serviceResponseErr != nil {
		return gondulapi.Result{Error: serviceResponseErr}
	}
	defer serviceResponse.Body.Close()
	if serviceResponse.StatusCode < 200 || serviceResponse.StatusCode > 299 {
		return gondulapi.Result{Error: fmt.Errorf("response contained non-2XX status: %v", serviceResponse.Status)}
	}
	log.Tracef("VM service destroyed instance: %v", station.ID)

	// Change state to terminated and remove any assigned timeslot
	station.Status = StationStatusTerminated
	station.TimeslotID = ""

	result, err := db.Update("stations", station, "id", "=", station.ID)
	result.Error = err
	return result
}
