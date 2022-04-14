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

import "fmt"

// HTTPErrorf is a convenience-function to provide an Error data structure,
// which is essentially the same as fmt.Errorf(), but with an HTTP status
// code embedded into it which can be extracted.
func HTTPErrorf(code int, str string, v ...interface{}) HTTPError {
	return HTTPErrori(code, fmt.Sprintf(str, v...))
}

// HTTPErrori creates an error with the given status-code, with i as the
// message. i should either be a text string or implement fmt.Stringer
func HTTPErrori(code int, i interface{}) HTTPError {
	err := HTTPError{
		Code:    code,
		Message: i,
	}
	return err
}

// HTTPError is used to combine a text-based error with a HTTP error code.
type HTTPError struct {
	Code    int `json:"-"`
	Message interface{}
}

// InternalHTTPError is provided for the common case of returning an opaque
// error that can be passed to a user.
var InternalHTTPError = HTTPError{500, "Internal server error"}

// Error allows Error to implement the error interface. That's a whole lot
// of error in one sentence...
func (e HTTPError) Error() string {
	if m, ok := e.Message.(string); ok {
		return m
	}
	if m, ok := e.Message.(fmt.Stringer); ok {
		return m.String()
	}

	return fmt.Sprintf("%v", e.Message)
}
