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
	"time"

	"github.com/gathering/gondulapi"
	"github.com/gathering/gondulapi/db"
	"github.com/gathering/gondulapi/receiver"
	"github.com/google/uuid"

	log "github.com/sirupsen/logrus"
)

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
	ID          *uuid.UUID    `column:"id" json:"id"`               // Generated, required, unique
	TrackID     string        `column:"track" json:"track"`         // Required
	Shortname   string        `column:"shortname" json:"shortname"` // Required
	Name        string        `column:"name" json:"name"`
	Status      StationStatus `column:"status" json:"status"`           // Required
	Credentials string        `column:"credentials" json:"credentials"` // Host, port, password, etc. (typically hidden)
	Notes       string        `column:"notes" json:"notes"`             // Misc. notes
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
	for _, station := range stationsForAdmins {
		station.Credentials = ""
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
	userToken, userTokenOk := request.QueryArgs["user-token"]

	found, err := db.Select(station, "stations", "id", "=", id)
	if err != nil {
		return gondulapi.Result{Error: err}
	}
	if !found {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}

	// Show credentials only if a user token matching the active timeslot was provided
	credentials := station.Credentials
	station.Credentials = ""
	if userTokenOk && userToken != "" {
		now := time.Now()
		var timeslot Timeslot
		timeslotFound, timeslotErr := db.Select(timeslot, "timeslots",
			"user_token", "=", userToken,
			"track", "=", station.TrackID,
			"station_shortname", "=", station.Shortname,
			"begin_time", "<=", now,
			"end_time", ">=", now,
		)
		if timeslotErr != nil {
			return gondulapi.Result{Error: timeslotErr}
		}
		if timeslotFound && timeslot.UserToken == userToken {
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
	if result := station.validate(true); result.HasErrorOrCode() {
		return result
	}

	result := station.create()
	result.Code = 201
	result.Location = fmt.Sprintf("%v/station/%v", gondulapi.Config.SitePrefix, station.ID)
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
	if result := station.validate(false); result.HasErrorOrCode() {
		return result
	}

	return station.update()
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

func (station *Station) update() gondulapi.Result {
	if exists, err := station.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Update("stations", station, "id", "=", station.ID)
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

func (station *Station) validate(new bool) gondulapi.Result {
	switch {
	case station.ID == nil:
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	case station.TrackID == "":
		return gondulapi.Result{Code: 400, Message: "missing track ID"}
	case !station.validateStatus():
		return gondulapi.Result{Code: 400, Message: "missing or invalid status"}
	}

	// Check if existence is as expected
	if exists, err := station.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if new && exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate ID"}
	} else if !new && !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
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

	return gondulapi.Result{}
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

func (station *Station) existsTrackShortname() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE id != $1 AND track = $2 AND shortname = $3", station.ID, station.TrackID, station.Shortname)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

// Post attempts to create a new station, if the track supports it.
func (createRequest *StationProvisionRequest) Post(request *gondulapi.Request) gondulapi.Result {
	trackID, trackIDExists := request.PathArgs["track_id"]
	if !trackIDExists || trackID == "" {
		return gondulapi.Result{Code: 400, Message: "missing track ID"}
	}

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
		return gondulapi.Result{Code: 400, Message: "track type is not configured for dynamic stations"}
	}

	// Check limit, excluding terminated ones
	maxStations := trackConfig.MaxInstances
	if maxStations > 0 {
		currentRow := db.DB.QueryRow("SELECT COUNT(*) FROM stations WHERE track = $1 AND status != $2", track.ID, stationStatusTerminated)
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
	serviceData, serviceCallErr := createRequest.callService(trackConfig)
	if serviceCallErr != nil {
		return gondulapi.Result{Error: serviceCallErr}
	}

	// Create station
	var station Station
	newID := uuid.New()
	station.ID = &newID
	station.TrackID = trackID
	station.Shortname = strconv.Itoa(serviceData.ID)
	station.Name = serviceData.FQDN
	station.Status = stationStatusPreparing
	station.Credentials = fmt.Sprintf("Username: %v\nPassword: %v\nPublic IPv4 address: %v\nPublic IPv6 address: %v\nSSH port: %v",
		serviceData.Username, serviceData.Password, serviceData.IPv4Address, serviceData.IPv6Address, serviceData.SSHPort)
	station.Notes = fmt.Sprintf("FQDN: %v\nZone: %v\nVLAN ID: %v\nVLAN IPv4 Subnet: %v",
		serviceData.FQDN, serviceData.Zone, serviceData.VLANID, serviceData.VLANIPv4Subnet)
	if result := station.validate(true); result.HasErrorOrCode() {
		return result
	}

	result := station.create()
	result.Code = 201
	result.Location = fmt.Sprintf("%s/station/%s", gondulapi.Config.SitePrefix, station.ID)
	return result
}

func (createRequest *StationProvisionRequest) callService(trackConfig gondulapi.ServerTrackConfig) (*serverCreateStationResponse, error) {
	createURL := trackConfig.BaseURL + "/api/entry/new"
	var body = []byte(`{"username":"tech","uid":"gondulapi","task_type":"1"}`)
	request, requestErr := http.NewRequest("POST", createURL, bytes.NewBuffer(body))
	if requestErr != nil {
		return nil, requestErr
	}
	request.SetBasicAuth(trackConfig.AuthUsername, trackConfig.AuthPassword)
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, responseErr := client.Do(request)
	if responseErr != nil {
		return nil, responseErr
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("response contained non-2XX status: %v", response.Status)
	}

	responseBody, responseBodyErr := ioutil.ReadAll(response.Body)
	if responseBodyErr != nil {
		return nil, responseBodyErr
	}
	var responseData serverCreateStationResponse
	if err := json.Unmarshal(responseBody, &responseData); err != nil {
		return nil, err
	}

	log.Tracef("VM service created new instance: %v", responseData.ID)

	return &responseData, nil
}

// Post attempts to destroy a station, if the track supports it.
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
	serviceCallErr := destroyRequest.callService(&station, trackConfig)
	if serviceCallErr != nil {
		return gondulapi.Result{Error: serviceCallErr}
	}

	// Change state to terminated
	station.Status = stationStatusTerminated
	result, err := db.Update("stations", station, "id", "=", station.ID)
	result.Error = err
	return result
}

func (destroyRequest *StationTerminateRequest) callService(station *Station, trackConfig gondulapi.ServerTrackConfig) error {
	destroyURL := fmt.Sprintf("%v/api/entry/%v", trackConfig.BaseURL, station.Shortname)
	request, requestErr := http.NewRequest("DELETE", destroyURL, nil)
	if requestErr != nil {
		return requestErr
	}
	request.SetBasicAuth(trackConfig.AuthUsername, trackConfig.AuthPassword)

	client := &http.Client{}
	response, responseErr := client.Do(request)
	if responseErr != nil {
		return responseErr
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("response contained non-2XX status: %v", response.Status)
	}

	log.Tracef("VM service destroyed instance: %v", station.ID)

	return nil
}
