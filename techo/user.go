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
	"github.com/gathering/gondulapi"
	"github.com/gathering/gondulapi/db"
	"github.com/gathering/gondulapi/receiver"
	"github.com/google/uuid"
)

/*
 * TODO:
 * - Authorize access to sensitive info.
 */

// User reperesent a single user, including registry
// information. This is retrieved from the frontend, so where it comes from
// is somewhat irrelevant.
type User struct {
	ID           *uuid.UUID `column:"id" json:"id"`                       // Generated, required, unique
	UserName     string     `column:"user_name" json:"user_name"`         // Required, unique
	EmailAddress string     `column:"email_address" json:"email_address"` // Required
	FirstName    string     `column:"first_name" json:"first_name"`       // Required
	LastName     string     `column:"last_name" json:"last_name"`         // Required
}

// Users is a list of users.
type Users []*User

func init() {
	receiver.AddHandler("/users/", "^$", func() interface{} { return &Users{} })
	receiver.AddHandler("/user/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &User{} })
}

// Get gets multiple users.
func (users *Users) Get(request *gondulapi.Request) gondulapi.Result {
	var whereArgs []interface{}
	if userName, ok := request.QueryArgs["user_name"]; ok {
		whereArgs = append(whereArgs, "user_name", "=", userName)
	}

	selectErr := db.SelectMany(users, "users", whereArgs...)
	if selectErr != nil {
		return gondulapi.Result{Error: selectErr}
	}

	return gondulapi.Result{}
}

// Get gets a single user.
func (user *User) Get(request *gondulapi.Request) gondulapi.Result {
	id, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	}

	found, err := db.Select(user, "users", "id", "=", id)
	if err != nil {
		return gondulapi.Result{Error: err}
	}
	if !found {
		return gondulapi.Result{Code: 404, Message: "not found"}
	}

	return gondulapi.Result{}
}

// Post creates a new user.
func (user *User) Post(request *gondulapi.Request) gondulapi.Result {
	if user.ID == nil {
		newID := uuid.New()
		user.ID = &newID
	}
	if result := user.validate(); result.HasErrorOrCode() {
		return result
	}

	if exists, err := user.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate ID"}
	}

	return user.create()
}

// Put updates a user.
func (user *User) Put(request *gondulapi.Request) gondulapi.Result {
	rawID, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}
	id, uuidErr := uuid.Parse(rawID)
	if uuidErr != nil {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "invalid ID"}
	}

	if *user.ID != id {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := user.validate(); result.HasErrorOrCode() {
		return result
	}

	return user.update()
}

// Delete deletes a user.
func (user *User) Delete(request *gondulapi.Request) gondulapi.Result {
	rawID, idExists := request.PathArgs["id"]
	if !idExists {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "missing ID"}
	}
	id, uuidErr := uuid.Parse(rawID)
	if uuidErr != nil {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "malformed UUID"}
	}

	user.ID = &id
	exists, existsErr := user.exists()
	if existsErr != nil {
		return gondulapi.Result{Failed: 1, Error: existsErr}
	}
	if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Delete("users", "id", "=", user.ID)
	result.Error = err
	return result
}

func (user *User) create() gondulapi.Result {
	if exists, err := user.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if exists {
		return gondulapi.Result{Failed: 1, Code: 409, Message: "duplicate"}
	}

	result, err := db.Insert("users", user)
	result.Error = err
	return result
}

func (user *User) update() gondulapi.Result {
	if exists, err := user.exists(); err != nil {
		return gondulapi.Result{Failed: 1, Error: err}
	} else if !exists {
		return gondulapi.Result{Failed: 1, Code: 404, Message: "not found"}
	}

	result, err := db.Update("users", user, "id", "=", user.ID)
	result.Error = err
	return result
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

func (user *User) validate() gondulapi.Result {
	switch {
	case user.ID == nil:
		return gondulapi.Result{Code: 400, Message: "missing ID"}
	case user.UserName == "":
		return gondulapi.Result{Code: 400, Message: "missing username"}
	case user.EmailAddress == "":
		return gondulapi.Result{Code: 400, Message: "missing email address"}
	case user.FirstName == "" || user.LastName == "":
		return gondulapi.Result{Code: 400, Message: "missing first or last name"}
	}

	if ok, err := user.checkUniqueFields(); err != nil {
		return gondulapi.Result{Error: err}
	} else if !ok {
		return gondulapi.Result{Code: 409, Message: "user_name already exists"}
	}

	return gondulapi.Result{}
}

func (user *User) checkUniqueFields() (bool, error) {
	rows, err := db.DB.Query("SELECT id FROM users WHERE id != $1 AND user_name = $2", user.ID, user.UserName)
	if err != nil {
		return false, err
	}
	defer func() {
		rows.Close()
	}()

	hasNext := rows.Next()
	return !hasNext, nil
}
