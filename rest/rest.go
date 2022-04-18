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

import "github.com/google/uuid"

// Request contains the last part of the URL (without the handler prefix), certain query args,
// and a limit on how many elements to get.
type Request struct {
	ID          uuid.UUID
	Method      string
	AccessToken AccessTokenEntry
	PathArgs    map[string]string
	QueryArgs   map[string]string
	ListLimit   int  // How many elements to return in listings (convenience)
	ListBrief   bool // If only the most relevant fields should be included listings (convenience)
}

// Result is an update report on write-requests. The precise meaning might
// vary, but the gist should be the same.
type Result struct {
	Message  string `json:"message,omitempty"` // Message for client
	Code     int    `json:"-"`                 // HTTP status
	Location string `json:"-"`                 // For location header if code 3xx
	Error    error  `json:"-"`                 // Internal error, forces code 500, hidden from client to avoid leak
}

// IsOk checks if error free and either not set code or a non-error code.
func (result *Result) IsOk() bool {
	return result.Error == nil && result.Code >= 0 && result.Code < 400
}

// Getter implements Get method, which should fetch the object represented
// by the element path.
type Getter interface {
	Get(request *Request) Result
}

// Putter is an idempotent method that requires an absolute path. It should
// (over-)write the object found at the element path.
type Putter interface {
	Put(request *Request) Result
}

// Poster is not necessarily idempotent, but can be. It should write the
// object provided, potentially generating a new ID for it if one isn't
// provided in the data structure itself.
// Post should ignore the request element.
type Poster interface {
	Post(request *Request) Result
}

// Deleter should delete the object identified by the element. It should be
// idempotent, in that it should be safe to call it on already-deleted
// items.
type Deleter interface {
	Delete(request *Request) Result
}
