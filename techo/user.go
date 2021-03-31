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
func (users *UsersForAdmins) Get(request *gondulapi.Request) gondulapi.Result {
	var whereArgs []interface{}
	if username, ok := request.QueryArgs["token"]; ok {
		whereArgs = append(whereArgs, "username", "=", username)
	}

	selectErr := db.SelectMany(users, "users", whereArgs...)
	if selectErr != nil {
		return gondulapi.Result{Error: selectErr}
	}

	return gondulapi.Result{}
}

// Put updates a user.
func (user *User) Put(request *gondulapi.Request) gondulapi.Result {
	token, tokenExists := request.PathArgs["token"]
	if !tokenExists || token == "" {
		return gondulapi.Result{Code: 400, Message: "missing token"}
	}

	if user.Token != token {
		return gondulapi.Result{Failed: 1, Code: 400, Message: "mismatch between URL and JSON IDs"}
	}
	if result := user.validate(); result.HasErrorOrCode() {
		return result
	}

	return user.createOrUpdate()
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

func (user *User) createOrUpdate() gondulapi.Result {
	exists, existsErr := user.exists()
	if existsErr != nil {
		return gondulapi.Result{Failed: 1, Error: existsErr}
	}

	if exists {
		result, err := db.Update("users", user, "token", "=", user.Token)
		result.Error = err
		return result
	}

	result, err := db.Insert("users", user)
	result.Error = err
	return result
}

func (user *User) exists() (bool, error) {
	var count int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM users WHERE token = $1", user.Token)
	rowErr := row.Scan(&count)
	if rowErr != nil {
		return false, rowErr
	}
	return count > 0, nil
}

func (user *User) validate() gondulapi.Result {
	switch {
	case user.Token == "":
		return gondulapi.Result{Code: 400, Message: "missing token"}
	case user.Username == "":
		return gondulapi.Result{Code: 400, Message: "missing username"}
	case user.DisplayName == "":
		return gondulapi.Result{Code: 400, Message: "missing display name"}
	case user.EmailAddress == "":
		return gondulapi.Result{Code: 400, Message: "missing email address"}
	}

	if exists, err := user.existsUsername(); err != nil {
		return gondulapi.Result{Error: err}
	} else if exists {
		return gondulapi.Result{Code: 409, Message: "username already exists"}
	}

	return gondulapi.Result{}
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
