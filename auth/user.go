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

package auth

import (
	"github.com/gathering/tech-online-backend/db"
	"github.com/gathering/tech-online-backend/receiver"
	"github.com/gathering/tech-online-backend/rest"
)

// User reperesent a single user, including registry
// information. This is retrieved from the frontend, so where it comes from
// is somewhat irrelevant.
type User struct {
	Token        string `column:"token" json:"token"`                 // Required, unique, secret
	Username     string `column:"username" json:"username"`           // Required, unique
	DisplayName  string `column:"display_name" json:"display_name"`   // Required
	EmailAddress string `column:"email_address" json:"email_address"` // Required
}

// Users is a list of users.
type Users []*User

// UsersForAdmins is a list of users and only accessible for admins.
type UsersForAdmins Users

func init() {
	receiver.AddHandler("/admin/users/", "^$", func() interface{} { return &UsersForAdmins{} }) // Admin
	receiver.AddHandler("/user/", "^(?:(?P<token>[^/]+)/)?$", func() interface{} { return &User{} })
}

// Get gets multiple users.
func (users *UsersForAdmins) Get(request *rest.Request) rest.Result {
	var whereArgs []interface{}
	if username, ok := request.QueryArgs["token"]; ok {
		whereArgs = append(whereArgs, "username", "=", username)
	}

	dbResult := db.SelectMany(users, "users", whereArgs...)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

// Put updates a user.
func (user *User) Put(request *rest.Request) rest.Result {
	token, tokenExists := request.PathArgs["token"]
	if !tokenExists || token == "" {
		return rest.Result{Code: 400, Message: "missing token"}
	}

	if user.Token != token {
		return rest.Result{Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := user.validate(); !result.IsOk() {
		return result
	}

	return user.createOrUpdate()
}

func (user *User) create() rest.Result {
	if exists, err := user.ExistsWithToken(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if exists {
		return rest.Result{Code: 409, Message: "duplicate"}
	}

	dbResult := db.Insert("users", user)
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (user *User) createOrUpdate() rest.Result {
	exists, existsErr := user.ExistsWithToken()
	if existsErr != nil {
		return rest.Result{Code: 500, Error: existsErr}
	}

	var dbResult db.Result
	if exists {
		dbResult = db.Update("users", user, "token", "=", user.Token)
	} else {
		dbResult = db.Insert("users", user)
	}
	if dbResult.IsFailed() {
		return rest.Result{Code: 500, Error: dbResult.Error}
	}
	return rest.Result{}
}

func (user *User) ExistsWithToken() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM users WHERE token = $1", user.Token)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (user *User) validate() rest.Result {
	switch {
	case user.Token == "":
		return rest.Result{Code: 400, Message: "missing token"}
	case user.Username == "":
		return rest.Result{Code: 400, Message: "missing username"}
	case user.DisplayName == "":
		return rest.Result{Code: 400, Message: "missing display name"}
	case user.EmailAddress == "":
		return rest.Result{Code: 400, Message: "missing email address"}
	}

	if exists, err := user.existsUsername(); err != nil {
		return rest.Result{Code: 500, Error: err}
	} else if exists {
		return rest.Result{Code: 409, Message: "username already exists"}
	}

	return rest.Result{}
}

func (user *User) existsUsername() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM users WHERE token != $1 AND username = $2", user.Token, user.Username)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}
