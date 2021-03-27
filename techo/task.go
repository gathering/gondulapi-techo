/*
Tech:Online backend
Copyright 2020, Kristian Lyngstøl <kly@kly.no>

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
	receiver.AddHandler("/task/", "^(?:(?P<id>[^/]+)/)?", func() interface{} { return &Task{} })
}

// Get gets multiple tasks.
func (tasks *Tasks) Get(request *gondulapi.Request) error {
	var whereArgs []interface{}
	if trackID, ok := request.QueryArgs["track"]; ok {
		whereArgs = append(whereArgs, "track", "=", trackID)
	}
	if shortname, ok := request.QueryArgs["shortname"]; ok {
		whereArgs = append(whereArgs, "shortname", "=", shortname)
	}

	selectErr := db.SelectMany(tasks, "tasks", whereArgs...)
	if selectErr != nil {
		return gondulapi.Error{Code: 500, Message: "failed to query database"}
	}

	return nil
}

// Get gets a single task.
func (task *Task) Get(request *gondulapi.Request) error {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	found, err := db.Select(task, "tasks", "id", "=", id)
	if err != nil {
		return err
	}
	if !found {
		return gondulapi.Error{Code: 404, Message: "not found"}
	}

	return nil
}

// Post creates a new task.
func (task *Task) Post(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if task.ID == nil {
		newID := uuid.New()
		task.ID = &newID
	}

	if err := task.validate(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}

	if exists, err := task.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 409, Message: "duplicate ID"}
	}

	return task.create()
}

// Put updates a task.
func (task *Task) Put(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	if (*task.ID).String() != id {
		return gondulapi.WriteReport{Failed: 1}, fmt.Errorf("mismatch between URL and JSON IDs")
	}

	if err := task.validate(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}

	return task.update()
}

// Delete deletes a task.
func (task *Task) Delete(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	rawID, rawIDExists := request.PathArgs["id"]
	if !rawIDExists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "missing ID"}
	}
	id, uuidError := uuid.Parse(rawID)
	if uuidError != nil {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "invalid ID"}
	}

	task.ID = &id
	exists, err := task.exists()
	if err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 404, Message: "not found"}
	}
	return db.Delete("tasks", "id", "=", task.ID)
}

func (task *Task) create() (gondulapi.WriteReport, error) {
	if exists, err := task.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 409, Message: "duplicate"}
	}

	return db.Insert("tasks", task)
}

func (task *Task) update() (gondulapi.WriteReport, error) {
	if exists, err := task.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 404, Message: "not found"}
	}

	return db.Update("tasks", task, "id", "=", task.ID)
}

func (task *Task) exists() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM tasks WHERE id = $1", task.ID)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}

func (task *Task) existsShortname() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM tasks WHERE track = $1 AND shortname = $2", task.TrackID, task.Shortname)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}

func (task *Task) validate() error {
	switch {
	case task.ID == nil:
		return gondulapi.Error{Code: 400, Message: "missing ID"}
	case task.TrackID == "":
		return gondulapi.Error{Code: 400, Message: "missing track ID"}
	case task.Shortname == "":
		return gondulapi.Error{Code: 400, Message: "missing shortname"}
	case task.Name == "":
		return gondulapi.Error{Code: 400, Message: "missing name"}
	}

	if ok, err := task.checkUniqueFields(); err != nil {
		return err
	} else if !ok {
		return gondulapi.Error{Code: 409, Message: "combination of track and shortname already exists"}
	}

	track := Track{ID: task.TrackID}
	if exists, err := track.exists(); err != nil {
		return err
	} else if !exists {
		return gondulapi.Error{Code: 400, Message: "referenced track does not exist"}
	}

	return nil
}

func (task *Task) checkUniqueFields() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM tasks WHERE id != $1 AND track = $2 AND shortname = $3", task.ID, task.TrackID, task.Shortname)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return !hasNext, nil
}
