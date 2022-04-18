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

	"github.com/gathering/tech-online-backend/config"
	"github.com/gathering/tech-online-backend/db"
	"github.com/gathering/tech-online-backend/rest"
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
	rest.AddHandler("/tasks/", "^$", func() interface{} { return &Tasks{} })
	rest.AddHandler("/task/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &Task{} })
}

// Get gets multiple tasks.
func (tasks *Tasks) Get(request *rest.Request) rest.Result {
	// TODO order by sequence

	// Check params, prep filtering
	var whereArgs []interface{}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}
	if shortname, ok := request.QueryArgs["shortname"]; ok {
		whereArgs = append(whereArgs, "shortname", "=", shortname)
	}

	// Get
	dbResult := db.SelectMany(tasks, "tasks", whereArgs...)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

// Get gets a single task.
func (task *Task) Get(request *rest.Request) rest.Result {
	// Check params
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Get
	dbResult := db.Select(task, "tasks", "id", "=", id)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	if !dbResult.IsSuccess() {
		return rest.Result{Code: 404, Message: "not found"}
	}
	return rest.Result{}
}

// Post creates a new task.
func (task *Task) Post(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.UnauthorizedResult(request.AccessToken)
	}

	// Prepare and validate
	if task.ID == nil {
		newID := uuid.New()
		task.ID = &newID
	}
	if result := task.validate(); !result.IsOk() {
		return result
	}

	// Create and redirect
	result := task.create()
	if !result.IsOk() {
		return result
	}
	result.Code = 201
	result.Location = fmt.Sprintf("%v/task/%v/", config.Config.SitePrefix, task.ID)
	return result
}

// Put updates a task.
func (task *Task) Put(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.UnauthorizedResult(request.AccessToken)
	}

	// Check params
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return rest.Result{Code: 400, Message: "missing ID"}
	}

	// Validate
	if task.ID != nil && (*task.ID).String() != id {
		return rest.Result{Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := task.validate(); !result.IsOk() {
		return result
	}

	// Create or update
	return task.createOrUpdate()
}

// Delete deletes a task.
func (task *Task) Delete(request *rest.Request) rest.Result {
	// Check perms
	if request.AccessToken.GetRole() != rest.RoleAdmin {
		return rest.UnauthorizedResult(request.AccessToken)
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

	// Check if exists
	task.ID = &id
	exists, err := task.exists()
	if err != nil {
		return rest.Result{Code: 500, Error: err}
	}
	if !exists {
		return rest.Result{Code: 404, Message: "not found"}
	}

	// Delete
	dbResult := db.Delete("tasks", "id", "=", task.ID)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (task *Task) create() rest.Result {
	if exists, err := task.exists(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if exists {
		return rest.Result{Code: 409, Message: "duplicate"}
	}

	dbResult := db.Insert("tasks", task)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (task *Task) createOrUpdate() rest.Result {
	exists, existsErr := task.exists()
	if existsErr != nil {
		return rest.Result{Code: 500, Error: existsErr}
	}

	var dbResult db.Result
	if exists {
		dbResult = db.Update("tasks", task, "id", "=", task.ID)
	} else {
		if exists, err := task.existsTaskShortnameWithDifferentID(); err != nil {
			return rest.Result{Code: 500, Error: err}
		} else if exists {
			return rest.Result{Code: 409, Message: "Shortname is already used with a different task"}
		}
		dbResult = db.Insert("tasks", task)
	}
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
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

func (task *Task) validate() rest.Result {
	switch {
	case task.ID == nil:
		return rest.Result{Code: 400, Message: "missing ID"}
	case task.TrackID == "":
		return rest.Result{Code: 400, Message: "missing track ID"}
	case task.Shortname == "":
		return rest.Result{Code: 400, Message: "missing shortname"}
	case task.Name == "":
		return rest.Result{Code: 400, Message: "missing name"}
	}

	track := Track{ID: task.TrackID}
	if exists, err := track.exists(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if !exists {
		return rest.Result{Code: 400, Message: "referenced track does not exist"}
	}

	return rest.Result{}
}

func (task *Task) existsTaskShortnameWithDifferentID() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM tasks WHERE id != $1 AND track = $2 AND shortname = $3", task.ID, task.TrackID, task.Shortname)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}
