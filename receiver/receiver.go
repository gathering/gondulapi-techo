/*
Gondul GO API, http receiver code
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

package receiver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gathering/gondulapi"

	log "github.com/sirupsen/logrus"
)

var handles map[string]Allocator

type input struct {
	method string
	data   []byte
	url    *url.URL
	query  map[string][]string
	pretty bool
	limit  int
}

type output struct {
	code         int
	data         interface{}
	cachecontrol string
}

type receiver struct {
	alloc Allocator
	path  string
}

// answer replies to a HTTP request with the provided output, optionally
// formatting the output prettily. It also calculates an ETag.
func (rcvr receiver) answer(w http.ResponseWriter, input input, output output) {
	code := output.code

	var b []byte
	var jsonErr error
	if input.pretty {
		b, jsonErr = json.MarshalIndent(output.data, "", "  ")
	} else {
		b, jsonErr = json.Marshal(output.data)
	}
	if jsonErr != nil {
		log.Printf("Json marshal error: %v", jsonErr)
		b = []byte(`{"Message": "JSON marshal error. Very weird."}`)
		code = 500
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	etagraw := sha256.Sum256(b)
	etagstr := hex.EncodeToString(etagraw[:])
	w.Header().Set("ETag", etagstr)

	w.WriteHeader(code)
	if code != 204 {
		fmt.Fprintf(w, "%s\n", b)
	}
}

// get is a badly named function in the context of HTTP since what it
// really does is just read the body of a HTTP request. In my defence, it
// used to do more. But what have it done for me lately?!
func (rcvr receiver) get(w http.ResponseWriter, r *http.Request) (input, error) {
	var input input
	input.url = r.URL
	input.query = r.URL.Query()
	input.method = r.Method
	log.WithFields(log.Fields{
		"url":     r.URL,
		"method":  r.Method,
		"address": r.RemoteAddr,
	}).Infof("Request")

	if r.ContentLength != 0 {
		input.data = make([]byte, r.ContentLength)

		if n, err := io.ReadFull(r.Body, input.data); err != nil {
			log.WithFields(log.Fields{
				"address":  r.RemoteAddr,
				"error":    err,
				"numbytes": n,
			}).Error("Read error from client")
			return input, fmt.Errorf("read failed: %v", err)
		}
	}

	input.pretty = len(r.URL.Query()["pretty"]) > 0

	if limitArgs := r.URL.Query()["limit"]; len(limitArgs) > 0 {
		if i, err := strconv.Atoi(limitArgs[0]); err == nil {
			input.limit = i
		}
	}

	return input, nil
}

// message is a convenience function
func message(str string, v ...interface{}) (m struct {
	Message string
	Error   string `json:",omitempty"`
}) {
	m.Message = fmt.Sprintf(str, v...)
	return
}

// handle figures out what Method the input has, casts item to the correct
// interface and calls the relevant function, if any, for that data. For
// PUT and POST it also parses the input data.
func handle(item interface{}, input input, path string) (output output) {
	output.code = 200
	var report gondulapi.WriteReport
	var err error

	defer func() {
		log.WithFields(log.Fields{
			"output.code": output.code,
			"output.data": output.data,
			"error":       err,
		}).Trace("Request handled")
		gerr, havegerr := err.(gondulapi.Error)
		if err != nil && report.Error == nil {
			report.Error = err
		}

		if report.Code != 0 {
			output.code = report.Code
		} else if havegerr {
			log.Tracef("During REST defered reply, we got a gondulapi.Error: %v", gerr)
			output.code = gerr.Code
		} else if report.Error != nil {
			output.code = 500
		}

		if output.data == nil && output.code != 204 {
			output.data = report
		}
	}()

	request := gondulapi.Request{
		Element: input.url.Path[len(path):],
		Args:    input.query,
		Limit:   input.limit,
	}

	switch input.method {
	case "GET":
		get, ok := item.(gondulapi.Getter)
		if !ok {
			output.code = 405
			output.data = message("method not allowed")
			return
		}
		err = get.Get(&request)
		if err != nil {
			return
		}
		output.data = get
	case "PUT":
		if err := json.Unmarshal(input.data, &item); err != nil {
			output.code = 400
			output.data = message("malformed data")
			return
		}
		put, ok := item.(gondulapi.Putter)
		if !ok {
			output.code = 405
			output.data = message("method not allowed")
			return
		}
		report, err = put.Put(&request)
		output.data = report
	case "DELETE":
		del, ok := item.(gondulapi.Deleter)
		if !ok {
			output.code = 405
			output.data = message("method not allowed")
			return
		}
		report, err = del.Delete(&request)
		output.data = report
	case "POST":
		if err := json.Unmarshal(input.data, &item); err != nil {
			output.code = 400
			output.data = message("malformed data")
			return
		}
		post, ok := item.(gondulapi.Poster)
		if !ok {
			output.code = 405
			output.data = message("method not allowed")
			return
		}
		report, err = post.Post(&request)
		output.data = report
	default:
		output.code = 405
		output.data = message("method not allowed")
		return
	}
	return
}

// ServeHTTP implements the net/http ServeHTTP handler. It does this by
// first reading input data, then allocating a data structure specified on
// the receiver originally through AddHandler, then parses input data onto
// that data and replies. All input/output is valid JSON.
func (rcvr receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	input, err := rcvr.get(w, r)
	log.WithFields(log.Fields{
		"data": string(input.data),
		"err":  err,
	}).Trace("Got")
	item := rcvr.alloc()
	output := handle(item, input, rcvr.path)
	rcvr.answer(w, input, output)
}
