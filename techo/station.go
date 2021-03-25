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
	"strings"

	"github.com/gathering/gondulapi"
	"github.com/gathering/gondulapi/db"
	"github.com/gathering/gondulapi/receiver"
)

// Station represent a single station.
type Station struct {
	// TODO track
	ID *string `column:"id" json:"id"` // Required, unique
	// TODO enum status. PREPARING, READY, ACTIVE, DIRTY, TERMINATED, MAINTENANCE
	Status   *string `column:"status" json:"status,omitempty"`     // Status, e.g. "active"
	Endpoint *string `column:"endpoint" json:"endpoint,omitempty"` // Host and post for host/jumphost
	Password *string `column:"password" json:"password,omitempty"` // Password for host
	Notes    *string `column:"notes" json:"notes,omitempty"`       // Misc. notes to show to user. Typically track-specific
}

// Stations is a list of stations.
type Stations []*Station

func init() {
	receiver.AddHandler("/stations/", "", func() interface{} { return &Stations{} })
	receiver.AddHandler("/station/", "^(?:(?P<id>[^/]+)/)?", func() interface{} { return &Station{} })
}

// Get gets multiple stations.
func (stations *Stations) Get(request *gondulapi.Request) error {
	var queryBuilder strings.Builder
	nextQueryArgID := 1
	var queryArgs []interface{}
	queryBuilder.WriteString("SELECT id,status,endpoint,password,notes FROM stations")
	if status, ok := request.QueryArgs["status"]; ok && len(status) > 0 {
		queryBuilder.WriteString(fmt.Sprintf(" WHERE status = $%v", nextQueryArgID))
		nextQueryArgID++
		queryArgs = append(queryArgs, status)
	}
	if request.ListLimit > 0 {
		queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%v", nextQueryArgID))
		nextQueryArgID++
		queryArgs = append(queryArgs, request.ListLimit)
	}

	rows, err := db.DB.Query(queryBuilder.String(), queryArgs...)
	if err != nil {
		return gondulapi.Error{Code: 500, Message: "failed to query database"}
	}
	defer func() {
		rows.Close()
	}()

	for rows.Next() {
		var station Station
		err = rows.Scan(&station.ID, &station.Status, &station.Endpoint, &station.Password, &station.Notes)
		if err != nil {
			return gondulapi.Error{Code: 500, Message: "failed to scan entity from the database"}
		}
		*stations = append(*stations, &station)
	}

	return nil
}

// Get gets a single station.
func (station *Station) Get(request *gondulapi.Request) error {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	rows, err := db.DB.Query("SELECT id,status,endpoint,password,notes FROM stations WHERE id = $1", id)
	if err != nil {
		return gondulapi.Error{Code: 500, Message: "failed to query database"}
	}
	defer func() {
		rows.Close()
	}()

	if !rows.Next() {
		return gondulapi.Error{Code: 404, Message: "not found"}
	}

	err = rows.Scan(&station.ID, &station.Status, &station.Endpoint, &station.Password, &station.Notes)
	if err != nil {
		return gondulapi.Error{Code: 500, Message: "failed to parse data from database"}
	}

	return nil
}

// Post creates a new station.
func (station *Station) Post(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if exists, err := station.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 409, Message: "duplicate ID"}
	}
	return station.create()
}

// Put creates or updates a station.
func (station *Station) Put(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "missing ID"}
	}
	if *station.ID != id {
		return gondulapi.WriteReport{Failed: 1}, fmt.Errorf("mismatch between URL and JSON IDs")
	}
	return station.createOrUpdate()
}

// Delete deletes a station.
func (station *Station) Delete(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 400, Message: "missing ID"}
	}

	station.ID = &id
	exists, err := station.exists()
	if err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Error{Code: 404, Message: "not found"}
	}
	return db.Delete("stations", "id", "=", station.ID)
}

func (station *Station) createOrUpdate() (gondulapi.WriteReport, error) {
	exists, err := station.exists()
	if err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	if exists {
		return station.update()
	}
	return station.create()
}

func (station *Station) create() (gondulapi.WriteReport, error) {
	return db.Insert("stations", station)
}

func (station *Station) update() (gondulapi.WriteReport, error) {
	return db.Update("stations", station, "id", "=", station.ID)
}

func (station *Station) exists() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM stations WHERE id = $1", station.ID)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}
