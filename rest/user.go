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

package rest

import (
	"github.com/gathering/tech-online-backend/db"
	"github.com/google/uuid"

	log "github.com/sirupsen/logrus"
)

// User reperesent a single user, including registry
// information. This is retrieved from the frontend, so where it comes from
// is somewhat irrelevant.
type User struct {
	ID           *uuid.UUID `column:"id" json:"id"`                       // Required, unique
	Username     string     `column:"username" json:"username"`           // Required, unique
	DisplayName  string     `column:"display_name" json:"display_name"`   // Required
	EmailAddress string     `column:"email_address" json:"email_address"` // Required
	Role         Role       `column:"role" json:"role"`                   // Required (valid)
}

// Users is a list of users.
type Users []*User

// UsersForAdmins is a list of users and only accessible for admins.
// type UsersForAdmins Users

func init() {
	AddHandler("/users/", "^$", func() interface{} { return &Users{} })
	AddHandler("/user/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &User{} })
}

// Get gets multiple users.
func (users *Users) Get(request *Request) Result {
	var whereArgs []interface{}
	if username, ok := request.QueryArgs["username"]; ok {
		whereArgs = append(whereArgs, "username", "=", username)
	}

	// Limit to only self if not operator/admin
	role := *request.AccessToken.GetRole()
	if role != RoleOperator && role != RoleAdmin {
		if request.AccessToken.User != nil {
			whereArgs = append(whereArgs, "id", "=", request.AccessToken.User.ID)
		} else {
			return Result{}
		}
	}

	dbResult := db.SelectMany(users, "users", whereArgs...)
	if dbResult.IsFailed() {
		return Result{Code: 500, Error: dbResult.Error}
	}
	return Result{}
}

// Get gets a user.
func (user *User) Get(request *Request) Result {
	strID, strIDExists := request.PathArgs["id"]
	if !strIDExists || strID == "" {
		return Result{Code: 400, Message: "missing ID"}
	}
	id, idParseErr := uuid.Parse(strID)
	if idParseErr != nil {
		return Result{Code: 400, Message: "invalid user ID"}
	}

	// Check if self or operator/admin
	role := *request.AccessToken.GetRole()
	if role != RoleOperator && role != RoleAdmin {
		if request.AccessToken.User == nil || request.AccessToken.User != nil && *request.AccessToken.User.ID != id {
			return Result{Code: 403, Message: "Access denied"}
		}
	}

	dbResult := db.Select(user, "users", "id", "=", id)
	if dbResult.IsFailed() {
		return Result{Code: 500, Error: dbResult.Error}
	}
	if !dbResult.IsSuccess() {
		return Result{Code: 404, Message: "not found"}
	}
	return Result{}
}

// Gets a user by ID if it exists, returns nil if not.
func getUserByID(id uuid.UUID) *User {
	var user *User
	dbResult := db.Select(user, "users", "id", "=", id)
	if dbResult.IsFailed() {
		log.WithError(dbResult.Error).Error("Failed to load user which may or may not not exist")
		return nil
	}
	return user
}

// Saves the user.
func (user *User) save() error {
	dbResult := db.Insert("users", user)
	if dbResult.IsFailed() {
		return dbResult.Error
	}
	return nil
}

// Put updates a user.
// func (user *User) Put(request *Request) Result {
// 	strID, strIDExists := request.PathArgs["id"]
// 	if !strIDExists || strID == "" {
// 		return Result{Code: 400, Message: "missing ID"}
// 	}
// 	id, idParseErr := uuid.Parse(strID)
// 	if idParseErr != nil {
// 		return Result{Code: 400, Message: "invalid ID"}
// 	}
// 	if result := user.validate(); !result.IsOk() {
// 		return result
// 	}
// 	if *user.ID != id {
// 		return Result{Code: 400, Message: "mismatch between URL and JSON IDs"}
// 	}

// 	return user.createOrUpdate()
// }

// func (user *User) create() Result {
// 	if exists, err := user.ExistsWithID(); err != nil {
// 		return Result{Code: 500, Error: err}
// 	} else if exists {
// 		return Result{Code: 409, Message: "duplicate"}
// 	}

// 	dbResult := db.Insert("users", user)
// 	if dbResult.IsFailed() {
// 		return Result{Code: 500, Error: dbResult.Error}
// 	}
// 	return Result{}
// }

func loadUser(id uuid.UUID) (*User, error) {
	var user *User
	dbResult := db.Select(user, "users", "id", "=", id)
	if dbResult.IsFailed() {
		return nil, dbResult.Error
	}
	return user, nil
}

func (user *User) createOrUpdate() Result {
	exists, existsErr := user.ExistsWithID()
	if existsErr != nil {
		return Result{Code: 500, Error: existsErr}
	}

	var dbResult db.Result
	if exists {
		dbResult = db.Update("users", user, "id", "=", user.ID)
	} else {
		dbResult = db.Insert("users", user)
	}
	if dbResult.IsFailed() {
		return Result{Code: 500, Error: dbResult.Error}
	}
	return Result{}
}

func (user *User) validate() Result {
	switch {
	case user.ID == nil:
		return Result{Code: 400, Message: "missing ID"}
	case user.Username == "":
		return Result{Code: 400, Message: "missing username"}
	case user.DisplayName == "":
		return Result{Code: 400, Message: "missing display name"}
	case user.EmailAddress == "":
		return Result{Code: 400, Message: "missing email address"}
	}

	if exists, err := user.ExistsWithUsername(); err != nil {
		return Result{Code: 500, Error: err}
	} else if exists {
		return Result{Code: 409, Message: "username already exists"}
	}

	return Result{}
}

// ExistsWithID checks whether a user with the specified ID exists or not.
func (user *User) ExistsWithID() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM users WHERE id = $1", user.ID)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

// ExistsWithUsername checks whether a user with the specified username exists or not.
func (user *User) ExistsWithUsername() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM users WHERE id != $1 AND username = $2", user.ID, user.Username)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}
