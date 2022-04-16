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

// Test is a test of a task.
// Track ID, task shortname and station shortname are used because clients aren't expected to know the task or station UUIDs.
type Test struct {
	ID                *uuid.UUID `column:"id" json:"id"`                               // Generated, required, unique
	TrackID           string     `column:"track" json:"track"`                         // Required
	TaskShortname     string     `column:"task_shortname" json:"task_shortname"`       // Required
	Shortname         string     `column:"shortname" json:"shortname"`                 // Required
	StationShortname  string     `column:"station_shortname" json:"station_shortname"` // Required
	TimeslotID        string     `column:"timeslot" json:"timeslot"`                   // Automatic, NULL if no current timeslot
	Name              string     `column:"name" json:"name"`                           // Required
	Description       string     `column:"description" json:"description"`
	Sequence          *int       `column:"sequence" json:"sequence"`
	Timestamp         *time.Time `column:"timestamp" json:"timestamp"`           // Generated, required
	StatusSuccess     *bool      `column:"status_success" json:"status_success"` // Required
	StatusDescription string     `column:"status_description" json:"status_description"`
}

// Tests is a list of tests.
type Tests []*Test

func init() {
	rest.AddHandler("/tests/", "^$", func() interface{} { return &Tests{} })
	rest.AddHandler("/test/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Test{} })
}

// Get gets multiple tests.
func (tests *Tests) Get(request *rest.Request) rest.Result {
	// TODO order by sequence

	// Check params, prep filtering
	var whereArgs []interface{}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}
	if taskShortname, ok := request.QueryArgs["task-shortname"]; ok {
		whereArgs = append(whereArgs, "task_shortname", "=", taskShortname)
	}
	if shortname, ok := request.QueryArgs["shortname"]; ok {
		whereArgs = append(whereArgs, "shortname", "=", shortname)
	}
	if stationShortname, ok := request.QueryArgs["station-shortname"]; ok {
		whereArgs = append(whereArgs, "station_shortname", "=", stationShortname)
	}
	if timeslot, ok := request.QueryArgs["timeslot"]; ok {
		whereArgs = append(whereArgs, "timeslot", "=", timeslot)
	}
	if _, ok := request.QueryArgs["latest"]; ok {
		whereArgs = append(whereArgs, "timeslot", "=", "")
	}

	// Get
	dbResult := db.SelectMany(tests, "tests", whereArgs...)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

// Post posts multiple tests which may overwrite old ones.
func (tests *Tests) Post(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleTester && request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.UnauthorizedResult(*request.AccessToken)
	}

	// Feed individual tests to the individual post endpoint, stop on first error
	totalResult := rest.Result{}
	for _, test := range *tests {
		result := test.Post(request)
		if !result.IsOk() {
			return result
		}
	}

	return totalResult
}

// Delete delete multiple tests.
func (tests *Tests) Delete(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleTester && request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.UnauthorizedResult(*request.AccessToken)
	}

	// Check params, prep filtering
	var whereArgs []interface{}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}
	if taskShortname, ok := request.QueryArgs["task-shortname"]; ok {
		whereArgs = append(whereArgs, "task_shortname", "=", taskShortname)
	}
	if shortname, ok := request.QueryArgs["shortname"]; ok {
		whereArgs = append(whereArgs, "shortname", "=", shortname)
	}
	if stationShortname, ok := request.QueryArgs["station-shortname"]; ok {
		whereArgs = append(whereArgs, "station_shortname", "=", stationShortname)
	}
	if timeslot, ok := request.QueryArgs["timeslot"]; ok {
		whereArgs = append(whereArgs, "timeslot", "=", timeslot)
	}
	if _, ok := request.QueryArgs["latest"]; ok {
		whereArgs = append(whereArgs, "timeslot", "=", "")
	}

	// Find all to delete
	dbResult := db.SelectMany(tests, "tests", whereArgs...)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}

	// Delete one by one, exit on first error
	for _, test := range *tests {
		dbResult := db.Delete("tests", "id", "=", test.ID)
		if dbResult.IsFailed() {
			return rest.Result{Code: 500, Error: dbResult.Error}
		}
	}

	return rest.Result{}
}

// Get gets a single test.
func (test *Test) Get(request *rest.Request) rest.Result {
	// Check params
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Get
	dbResult := db.Select(test, "tests", "id", "=", id)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	if !dbResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "not found"}
	}
	return rest.Result{}
}

// Post creates a new test. Existing tests with the same track/task/test/station/timeslot will get overwritten.
func (test *Test) Post(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleTester && request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.UnauthorizedResult(*request.AccessToken)
	}

	// Overwrite certain fields
	newID := uuid.New()
	test.ID = &newID
	test.TimeslotID = ""
	now := time.Now()
	test.Timestamp = &now

	// Validate
	if result := test.validate(); !result.IsOk() {
		return result
	}

	// Bind to the active timeslot, if any
	var station Station
	stationDBResult := db.Select(&station, "stations",
		"track", "=", test.TrackID,
		"shortname", "=", test.StationShortname,
	)
	if stationDBResult.IsFailed() {
		return rest.Result{Code: 500, Error: stationDBResult.Error}
	}
	if !stationDBResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "station not found"}
	}
	test.TimeslotID = station.TimeslotID

	// Delete old equivalent tests, both without timeslot and with the current timeslot
	_, deleteErr := db.DB.Exec("DELETE FROM tests WHERE track = $1 AND task_shortname = $2 AND shortname = $3 AND station_shortname = $4 AND (timeslot = $5 OR timeslot = '')",
		test.TrackID, test.TaskShortname, test.Shortname, test.StationShortname, test.TimeslotID)
	if deleteErr != nil {
		return rest.Result{Code: 500, Error: deleteErr}
	}

	// Save clone without timeslot
	if test.TimeslotID != "" {
		cloneTest := *test
		cloneTest.TimeslotID = ""
		newCloneID := uuid.New()
		cloneTest.ID = &newCloneID
		result := cloneTest.create()
		if !result.IsOk() {
			return result
		}
	}

	// Save original with timeslot
	result := test.create()
	if !result.IsOk() {
		return result
	}
	result.Code = 201
	result.Location = fmt.Sprintf("%v/test/%v", config.Config.SitePrefix, test.ID)
	return result
}

// Delete deletes a test.
func (test *Test) Delete(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleTester && request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.UnauthorizedResult(*request.AccessToken)
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
	test.ID = &id
	exists, err := test.exists()
	if err != nil {
		return rest.Result{Code: 500, Error: err}
	}
	if !exists {
		return rest.Result{Code: 404, Message: "not found"}
	}

	// Delete it
	dbResult := db.Delete("tests", "id", "=", test.ID)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (test *Test) create() rest.Result {
	if exists, err := test.exists(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if exists {
		return rest.Result{Code: 409, Message: "duplicate"}
	}

	dbResult := db.Insert("tests", test)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (test *Test) exists() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM tests WHERE id = $1", test.ID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (test *Test) validate() rest.Result {
	switch {
	case test.ID == nil:
		return rest.Result{Code: 400, Message: "missing ID"}
	case test.TrackID == "":
		return rest.Result{Code: 400, Message: "missing track ID"}
	case test.TaskShortname == "":
		return rest.Result{Code: 400, Message: "missing task shortname"}
	case test.Shortname == "":
		return rest.Result{Code: 400, Message: "missing shortname"}
	case test.StationShortname == "":
		return rest.Result{Code: 400, Message: "missing station shortname"}
	case test.Name == "":
		return rest.Result{Code: 400, Message: "missing name"}
	case test.StatusSuccess == nil:
		return rest.Result{Code: 400, Message: "missing success status"}
	case test.Timestamp == nil:
		return rest.Result{Code: 400, Message: "missing timestamp"}
	}

	track := Track{ID: test.TrackID}
	if exists, err := track.exists(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if !exists {
		return rest.Result{Code: 400, Message: "referenced track does not exist"}
	}
	task := Task{TrackID: test.TrackID, Shortname: test.TaskShortname}
	if exists, err := task.existsShortname(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if !exists {
		return rest.Result{Code: 400, Message: "referenced task does not exist"}
	}
	station := Station{TrackID: test.TrackID, Shortname: test.StationShortname}
	if exists, err := station.existsShortname(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if !exists {
		return rest.Result{Code: 400, Message: "referenced station does not exist"}
	}
	if test.TimeslotID != "" {
		timeslotID, timeslotIDErr := uuid.Parse(test.TimeslotID)
		if timeslotIDErr != nil {
			return rest.Result{Code: 400, Message: "invalid timeslot ID"}
		}
		timeslot := Timeslot{ID: &timeslotID}
		if exists, err := timeslot.exists(); err != nil {
			return rest.Result{Code: 500, Error: err}
		} else if !exists {
			return rest.Result{Code: 400, Message: "referenced timeslot does not exist"}
		}
	}

	return rest.Result{}
}
