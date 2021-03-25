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
	"regexp"
	"strconv"
	"strings"

	"github.com/gathering/gondulapi"

	log "github.com/sirupsen/logrus"
)

type receiver struct {
	pathPattern regexp.Regexp
	allocator   Allocator
}

type receiverSet struct {
	pathPrefix string
	receivers  []receiver
}

// Map of all receiver sets
var receiverSets map[string]*receiverSet

type input struct {
	url        *url.URL
	pathPrefix string
	pathSuffix string
	method     string
	data       []byte
	query      map[string][]string
	pretty     bool
}

type output struct {
	code         int
	data         interface{}
	cachecontrol string
}

func (set receiverSet) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	log.WithFields(log.Fields{
		"url":    request.URL,
		"method": request.Method,
		"client": request.RemoteAddr,
	}).Infof("Request")

	input, err := getInput(set.pathPrefix, request)
	if err != nil {
		log.WithFields(log.Fields{
			"data": string(input.data),
			"err":  err,
		}).Warn("Request input failed")
		return
	}

	var foundReceiver *receiver
	for _, receiver := range set.receivers {
		if receiver.pathPattern.MatchString(input.pathSuffix) {
			foundReceiver = &receiver
			break
		}
	}

	output := handleRequest(foundReceiver, input)
	answerRequest(writer, input, output)
}

// get is a badly named function in the context of HTTP since what it
// really does is just read the body of a HTTP request. In my defence, it
// used to do more. But what have it done for me lately?!
func getInput(pathPrefix string, request *http.Request) (input, error) {
	var input input
	fullPath := request.URL.Path
	// Make sure path always ends with "/"
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	input.url = request.URL
	input.pathPrefix = pathPrefix
	input.pathSuffix = fullPath[len(gondulapi.Config.Prefix+pathPrefix):]
	input.query = request.URL.Query()
	input.method = request.Method

	if request.ContentLength != 0 {
		input.data = make([]byte, request.ContentLength)

		if n, err := io.ReadFull(request.Body, input.data); err != nil {
			log.WithFields(log.Fields{
				"address":  request.RemoteAddr,
				"error":    err,
				"numbytes": n,
			}).Error("Read error from client")
			return input, fmt.Errorf("read failed: %v", err)
		}
	}

	input.pretty = len(request.URL.Query()["pretty"]) > 0

	return input, nil
}

// handle figures out what Method the input has, casts item to the correct
// interface and calls the relevant function, if any, for that data. For
// PUT and POST it also parses the input data.
func handleRequest(receiver *receiver, input input) (output output) {
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

	if receiver == nil {
		output.code = 404
		output.data = message("not found")
		return
	}

	var request gondulapi.Request
	request.PathArgs = make(map[string]string)
	argCaptures := receiver.pathPattern.FindStringSubmatch(input.pathSuffix)
	argCaptureNames := receiver.pathPattern.SubexpNames()
	for i := range argCaptures {
		if i > 0 {
			if argCaptureNames[i] != "" {
				request.PathArgs[argCaptureNames[i]] = argCaptures[i]
			}
		}
	}
	request.QueryArgs = make(map[string]string)
	for key, value := range input.query {
		// Only use first arg for each key
		if len(value) > 0 {
			request.QueryArgs[key] = value[0]
		} else {
			request.QueryArgs[key] = ""
		}
	}
	if value, exists := request.QueryArgs["limit"]; exists {
		if i, err := strconv.Atoi(value); err == nil {
			request.ListLimit = i
		}
	}
	if _, exists := request.QueryArgs["brief"]; exists {
		request.ListBrief = true
	}

	item := receiver.allocator()
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

// message is a convenience function
func message(str string, v ...interface{}) (m struct {
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}) {
	m.Message = fmt.Sprintf(str, v...)
	return
}

// answer replies to a HTTP request with the provided output, optionally
// formatting the output prettily. It also calculates an ETag.
func answerRequest(w http.ResponseWriter, input input, output output) {
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
