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
	"github.com/google/uuid"
)

// User reperesent a single user, including registry
// information. This is retrieved from the frontend, so where it comes from
// is somewhat irrelevant.
type User struct {
	ID           *uuid.UUID `column:"id" json:"id"`                                 // Required, unique (TODO generate or get from IdP?)
	UserName     *string    `column:"user_name" json:"user_name,omitempty"`         // Required, unique
	EmailAddress *string    `column:"email_address" json:"email_address,omitempty"` // Required
	FirstName    *string    `column:"first_name" json:"first_name,omitempty"`       // Required
	LastName     *string    `column:"last_name" json:"last_name,omitempty"`         // Required
}

// Users is a list of users.
type Users []*User

func init() {
	receiver.AddHandler("/users/", func() interface{} { return &Users{} })
	receiver.AddHandler("/user/", func() interface{} { return &User{} })
}

// Get gets multiple users.
func (users *Users) Get(request *gondulapi.Request) error {
	if request.Element != "" {
		return gondulapi.Errorf(400, "element not allowed")
	}

	var queryBuilder strings.Builder
	nextQueryArgID := 1
	var queryArgs []interface{}
	_, brief := request.Args["brief"]
	if brief {
		queryBuilder.WriteString("SELECT id,user_name FROM users")
	} else {
		queryBuilder.WriteString("SELECT id,user_name,email_address,first_name,last_name FROM users")
	}
	if userName, ok := request.Args["user_name"]; ok && len(userName) > 0 && len(userName[0]) > 0 {
		queryBuilder.WriteString(fmt.Sprintf(" WHERE user_name = $%v", nextQueryArgID))
		nextQueryArgID++
		queryArgs = append(queryArgs, userName[0])
	}
	if request.Limit > 0 {
		queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%v", nextQueryArgID))
		nextQueryArgID++
		queryArgs = append(queryArgs, request.Limit)
	}

	rows, err := db.DB.Query(queryBuilder.String(), queryArgs...)
	if err != nil {
		return gondulapi.Errorf(500, "failed to query database")
	}
	defer func() {
		rows.Close()
	}()

	for rows.Next() {
		var user User
		if brief {
			err = rows.Scan(&user.ID, &user.UserName)
		} else {
			err = rows.Scan(&user.ID, &user.UserName, &user.EmailAddress, &user.FirstName, &user.LastName)
		}
		if err != nil {
			return gondulapi.Errorf(500, "failed to scan entity from the database")
		}
		*users = append(*users, &user)
	}

	return nil
}

// Get gets a single user.
func (user *User) Get(request *gondulapi.Request) error {
	if request.Element == "" {
		return gondulapi.Errorf(400, "ID required")
	}

	rows, err := db.DB.Query("SELECT id,user_name,email_address,first_name,last_name FROM users WHERE id = $1", request.Element)
	if err != nil {
		return gondulapi.Errorf(500, "failed to query database")
	}
	defer func() {
		rows.Close()
	}()

	if !rows.Next() {
		return gondulapi.Errorf(404, "not found")
	}

	err = rows.Scan(&user.ID, &user.UserName, &user.EmailAddress, &user.FirstName, &user.LastName)
	if err != nil {
		return gondulapi.Errorf(500, "failed to parse data from database")
	}

	return nil
}

// Post creates a new user.
func (user *User) Post(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if exists, err := user.exists(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	} else if exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(409, "duplicate ID")
	}
	if err := user.validate(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	return user.create()
}

// Put creates or updates a user.
func (user *User) Put(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if request.Element == "" {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "ID required.")
	}
	id, uuidErr := uuid.Parse(request.Element)
	if uuidErr != nil {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "malformed uuid")
	}
	if *user.ID != id {
		return gondulapi.WriteReport{Failed: 1}, fmt.Errorf("mismatch between URL and JSON IDs")
	}
	if err := user.validate(); err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	return user.createOrUpdate()
}

// Delete deletes a user.
func (user *User) Delete(request *gondulapi.Request) (gondulapi.WriteReport, error) {
	if request.Element == "" {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "ID required")
	}
	id, uuidErr := uuid.Parse(request.Element)
	if uuidErr != nil {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(400, "malformed uuid")
	}
	user.ID = &id
	exists, existsErr := user.exists()
	if existsErr != nil {
		return gondulapi.WriteReport{Failed: 1}, existsErr
	}
	if !exists {
		return gondulapi.WriteReport{Failed: 1}, gondulapi.Errorf(404, "not found")
	}
	return db.Delete("users", "id", "=", user.ID)
}

func (user *User) createOrUpdate() (gondulapi.WriteReport, error) {
	exists, err := user.exists()
	if err != nil {
		return gondulapi.WriteReport{Failed: 1}, err
	}
	if exists {
		return user.update()
	}
	return user.create()
}

func (user *User) create() (gondulapi.WriteReport, error) {
	return db.Insert("users", user)
}

func (user *User) update() (gondulapi.WriteReport, error) {
	return db.Update("users", user, "id", "=", user.ID)
}

func (user *User) exists() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM users WHERE id = $1", user.ID)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return hasNext, nil
}

func (user *User) validate() error {
	switch {
	case user.ID == nil:
		return gondulapi.Errorf(400, "missing ID")
	case user.UserName == nil || *user.UserName == "":
		return gondulapi.Errorf(400, "missing username")
	case user.EmailAddress == nil || *user.EmailAddress == "":
		return gondulapi.Errorf(400, "missing email address")
	case user.FirstName == nil || *user.FirstName == "" || user.LastName == nil || *user.LastName == "":
		return gondulapi.Errorf(400, "missing first or last name")
	default:
		return nil
	}
}
