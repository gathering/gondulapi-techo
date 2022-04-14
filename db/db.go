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

/*
Package db integrates with generic databases, so far it doesn't do much,
but it's supposed to do more.
*/
package db

import (
	"database/sql"
	"fmt"

	"github.com/gathering/tech-online-backend/config"
	_ "github.com/lib/pq" // For postgres support
)

// DB is the main database handle used throughout the API
var DB *sql.DB

// Error - General database error.
type Error error

type dbError struct {
	message interface{}
}

func (e dbError) Error() string {
	return fmt.Sprintf("%v", e.message)
}

func newError(messageFormat string, formatVars ...interface{}) Error {
	return newErrorWithCause(messageFormat, nil, formatVars...)
}

func newErrorWithCause(messageFormat string, cause error, formatVars ...interface{}) Error {
	message := fmt.Sprintf(messageFormat, formatVars...)
	fullMessage := message
	if cause != nil {
		fullMessage = fmt.Sprintf("%s: %s", message, cause.Error())
	}
	return dbError{fullMessage}
}

// Result is an update report on write-requests. The precise meaning might
// vary, but the gist should be the same.
type Result struct {
	Affected int   `json:"affected,omitempty"`
	Ok       int   `json:"ok,omitempty"`
	Failed   int   `json:"failed,omitempty"`
	Error    error `json:"-"`
}

// IsSuccess checks that there were no failed elements, no error AND at least one OK element/operation.
func (result *Result) IsSuccess() bool {
	return result.Ok > 0 && result.Failed == 0 && result.Error == nil
}

// IsFailed checks if there is an error or if any elements failed.
func (result *Result) IsFailed() bool {
	return result.Failed > 0 || result.Error != nil
}

// Ping is a wrapper for DB.Ping: it checks that the database is alive.
// It's provided to add standard gondulapi-logging and error-types that can
// be exposed to users.
func Ping() error {
	if DB == nil {
		return newError("Database ping failed: Database not connected")
	}
	err := DB.Ping()
	if err != nil {
		return newError("Database ping failed: %v", err)
	}
	return nil
}

// Connect sets up the database connection, using the configured
// ConnectionString, and ensures it is working.
func Connect() error {
	var err error
	if DB != nil {
		return Ping()
	}

	connectionString := config.Config.DatabaseString
	if connectionString == "" {
		return newError("Missing database credentials")
	}

	DB, err = sql.Open("postgres", connectionString)
	if err != nil {
		return newError("Failed to connect to database: %v", err)
	}

	return Ping()
}
