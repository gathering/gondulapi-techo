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

// Test is a test of a task.
// Track ID, task shortname and station shortname are used because clients aren't expected to know the task or station UUIDs.
type Test struct {
	ID                *uuid.UUID `column:"id" json:"id"`                               // Generated, required, unique
	TrackID           string     `column:"track" json:"track"`                         // Required
	TaskShortname     string     `column:"task_shortname" json:"task_shortname"`       // Required
	Shortname         string     `column:"shortname" json:"shortname"`                 // Required
	StationShortname  string     `column:"station_shortname" json:"station_shortname"` // Required
	Timeslot          string     `column:"timeslot" json:"timeslot"`                   // Automatic, empty string (not NULL!) if no current timeslot
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
	receiver.AddHandler("/tests/", "^$", func() interface{} { return &Tests{} })
	receiver.AddHandler("/test/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Test{} })
}

// Get gets multiple tests.
func (tests *Tests) Get(request *gondulapi.Request) gondulapi.Result {
	var whereArgs []interface{}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}
	if taskShortname, ok := request.QueryArgs["task_shortname"]; ok {
		whereArgs = append(whereArgs, "task_shortname", "=", taskShortname)
	}
	if shortname, ok := request.QueryArgs["shortname"]; ok {
		whereArgs = append(whereArgs, "shortname", "=", shortname)
	}
	if stationShortname, ok := request.QueryArgs["station_shortname"]; ok {
		whereArgs = append(whereArgs, "station_shortname", "=", stationShortname)
	}
	if timeslot, ok := request.QueryArgs["timeslot"]; ok {
		whereArgs = append(whereArgs, "timeslot", "=", timeslot)
	}
	if _, ok := request.QueryArgs["latest"]; ok {
		whereArgs = append(whereArgs, "timeslot", "=", "")
	}

	selectErr := db.SelectMany(tests, "tests", whereArgs...)
	if selectErr != nil {
		return gondulapi.Result{Error: selectErr}
	}

	return gondulapi.Result{}
}

// Post posts multiple tests which may overwrite old ones.
func (tests *Tests) Post(request *gondulapi.Request) gondulapi.Result {
	// TODO do this better (transaction?)

	// Feed individual tests to the individual post endpoint, stop on first error
	totalResult := gondulapi.Result{}
	for _, test := range *tests {
		result := test.Post(request)
		if result.HasErrorOrCode() {
			return result
		}
		totalResult.Affected += result.Affected
		totalResult.Ok += result.Ok
		totalResult.Failed += result.Failed
	}

	return totalResult
}

// Get gets a single test.
func (test *Test) Get(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	}

	found, err := db.Select(test, "tests", "id", "=", id)
	if err != nil {
		return gondulapi.Result{Error: err}
	}
	if !found {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}

	return gondulapi.Result{}
}

// Post creates a new test. Existing tests with the same track/task/test/station/timeslot will get overwritten.
func (test *Test) Post(request *gondulapi.Request) gondulapi.Result {
	// Overwrite UUID
	newID := uuid.New()
	test.ID = &newID

	timestamp := time.Now()
	test.Timestamp = &timestamp

	if result := test.validate(); result.HasErrorOrCode() {
		return result
	}

	// For the astronomic chance of a UUID collision
	if exists, err := test.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate ID"}
	}

	// Bind to the active timeslot, or NULL if none is active
	// TODO
	test.Timeslot = ""

	// Delete old tests with and without timeslot
	_, deleteErr := db.DB.Exec("DELETE FROM tests WHERE track = $1 AND task_shortname = $2 AND shortname = $3 AND station_shortname = $4 AND (timeslot = $5 OR timeslot = '')", test.TrackID, test.TaskShortname, test.Shortname, test.StationShortname, test.Timeslot)
	if deleteErr != nil {
		return gondulapi.Result{Error: deleteErr}
	}

	var totalResult gondulapi.Result

	// Save copy for the timeslot
	if test.Timeslot != "" {
		result := test.create()
		if result.HasErrorOrCode() {
			return result
		}
		totalResult.Affected += result.Affected
		totalResult.Ok += result.Ok
		totalResult.Failed += result.Failed
	}

	// Save copy without timeslot (to fetch latest and between timeslots)
	test.Timeslot = ""
	result := test.create()
	if result.HasErrorOrCode() {
		return result
	}
	totalResult.Affected += result.Affected
	totalResult.Ok += result.Ok
	totalResult.Failed += result.Failed

	return totalResult
}

func (test *Test) create() gondulapi.Result {
	if exists, err := test.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate"}
	}

	result, err := db.Insert("tests", test)
	result.Error = err
	return result
}

func (test *Test) update() gondulapi.Result {
	if exists, err := test.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Update("tests", test, "id", "=", test.ID)
	result.Error = err
	return result
}

func (test *Test) exists() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM tests WHERE id = $1", test.ID)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}

func (test *Test) validate() gondulapi.Result {
	switch {
	case test.ID == nil:
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	case test.TrackID == "":
		return gondulapi.Result{Code: 400, Message: "missing track ID"}
	case test.TaskShortname == "":
		return gondulapi.Result{Code: 400, Message: "missing task shortname"}
	case test.Shortname == "":
		return gondulapi.Result{Code: 400, Message: "missing shortname"}
	case test.StationShortname == "":
		return gondulapi.Result{Code: 400, Message: "missing station shortname"}
	case test.Name == "":
		return gondulapi.Result{Code: 400, Message: "missing name"}
	case test.StatusSuccess == nil:
		return gondulapi.Result{Code: 400, Message: "missing success status"}
	case test.Timestamp == nil:
		return gondulapi.Result{Code: 400, Message: "missing timestamp"}
	}

	// Note: No need to check for duplicate /task/track/test/station since it will get overwritten

	track := Track{ID: test.TrackID}
	if exists, err := track.exists(); err != nil {
		return gondulapi.Result{Error: err}
	} else if !exists {
		return gondulapi.Result{Code: 400, Message: "referenced track does not exist"}
	}
	task := Task{TrackID: test.TrackID, Shortname: test.TaskShortname}
	if exists, err := task.existsShortname(); err != nil {
		return gondulapi.Result{Error: err}
	} else if !exists {
		return gondulapi.Result{Code: 400, Message: "referenced task does not exist"}
	}
	station := Station{TrackID: test.TrackID, Shortname: test.StationShortname}
	if exists, err := station.existsShortname(); err != nil {
		return gondulapi.Result{Error: err}
	} else if !exists {
		return gondulapi.Result{Code: 400, Message: "referenced station does not exist"}
	}

	return gondulapi.Result{}
}
