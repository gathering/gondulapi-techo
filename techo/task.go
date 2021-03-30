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

	"github.com/gathering/gondulapi"
	"github.com/gathering/gondulapi/db"
	"github.com/gathering/gondulapi/receiver"
	"github.com/google/uuid"
)

// Task is the components of a track.
type Task struct {
	ID          *uuid.UUID `column:"id" json:"id"`               // Generated, required, unique
	TrackID     string     `column:"track" json:"track"`         // Required
	Shortname   string     `column:"shortname" json:"shortname"` // Required, unique together with track
	Name        string     `column:"name" json:"name"`           // Required
	Description string     `column:"description" json:"description"`
	Sequence    *int       `column:"sequence" json:"sequence,omitempty"`
}

// Tasks is a list of tasks.
type Tasks []*Task

func init() {
	receiver.AddHandler("/tasks/", "^$", func() interface{} { return &Tasks{} })
	receiver.AddHandler("/task/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Task{} })
}

// Get gets multiple tasks.
func (tasks *Tasks) Get(request *gondulapi.Request) gondulapi.Result {
	var whereArgs []interface{}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}
	if shortname, ok := request.QueryArgs["shortname"]; ok {
		whereArgs = append(whereArgs, "shortname", "=", shortname)
	}

	selectErr := db.SelectMany(tasks, "tasks", whereArgs...)
	if selectErr != nil {
		return gondulapi.Result{Error: selectErr}
	}

	return gondulapi.Result{}
}

// Get gets a single task.
func (task *Task) Get(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	}

	found, err := db.Select(task, "tasks", "id", "=", id)
	if err != nil {
		return gondulapi.Result{Error: err}
	}
	if !found {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}

	return gondulapi.Result{}
}

// Post creates a new task.
func (task *Task) Post(request *gondulapi.Request) gondulapi.Result {
	if task.ID == nil {
		newID := uuid.New()
		task.ID = &newID
	}
	if result := task.validate(true); result.HasErrorOrCode() {
		return result
	}

	result := task.create()
	result.Code = 201
	result.Location = fmt.Sprintf("%v/task/%v", gondulapi.Config.SitePrefix, task.ID)
	return result
}

// Put updates a task.
func (task *Task) Put(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}

	if task.ID != nil && (*task.ID).String() != id {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "mismatch between URL and JSON IDs"}
	}

	if result := task.validate(false); result.HasErrorOrCode() {
		return result
	}

	return task.update()
}

// Delete deletes a task.
func (task *Task) Delete(request *gondulapi.Request) gondulapi.Result {
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists || rawID == "" {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}
	id, uuidError := uuid.Parse(rawID)
	if uuidError != nil {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid ID"}
	}

	task.ID = &id
	exists, err := task.exists()
	if err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	}
	if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Delete("tasks", "id", "=", task.ID)
	result.Error = err
	return result
}

func (task *Task) create() gondulapi.Result {
	if exists, err := task.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate"}
	}

	result, err := db.Insert("tasks", task)
	result.Error = err
	return result
}

func (task *Task) update() gondulapi.Result {
	if exists, err := task.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Update("tasks", task, "id", "=", task.ID)
	result.Error = err
	return result
}

func (task *Task) exists() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM tasks WHERE id = $1", task.ID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (task *Task) existsShortname() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM tasks WHERE track = $1 AND shortname = $2", task.TrackID, task.Shortname)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (task *Task) validate(new bool) gondulapi.Result {
	switch {
	case task.ID == nil:
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	case task.TrackID == "":
		return gondulapi.Result{Code: 400, Message: "missing track ID"}
	case task.Shortname == "":
		return gondulapi.Result{Code: 400, Message: "missing shortname"}
	case task.Name == "":
		return gondulapi.Result{Code: 400, Message: "missing name"}
	}

	// Check if existence is as expected
	if exists, err := task.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if new && exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate ID"}
	} else if !new && !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	if exists, err := task.existsTrackShortname(); err != nil {
		return gondulapi.Result{Error: err}
	} else if exists {
		return gondulapi.Result{Code: 409, Message: "combination of track and shortname already exists"}
	}

	track := Track{ID: task.TrackID}
	if exists, err := track.exists(); err != nil {
		return gondulapi.Result{Error: err}
	} else if !exists {
		return gondulapi.Result{Code: 400, Message: "referenced track does not exist"}
	}

	return gondulapi.Result{}
}

func (task *Task) existsTrackShortname() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM tasks WHERE id != $1 AND track = $2 AND shortname = $3", task.ID, task.TrackID, task.Shortname)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}
