/*
Gondul GO API, http receiver code
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
	location     string
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
	input.pathSuffix = fullPath[len(gondulapi.Config.SitePrefix+pathPrefix):]
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
	var result gondulapi.Result

	// Handle handler handling
	defer func() {
		if result.Error != nil {
			// Internal server error, ignore everything else
			log.WithError(result.Error).Warn("internal server error")
			output.code = 500
			output.data = message("internal server error")
			return
		}

		// Override default code if handles provided one
		if result.Code != 0 {
			output.code = result.Code
		}

		if output.code >= 300 && output.code <= 399 {
			// Redirect
			output.location = result.Location
		} else if output.data == nil && output.code != 204 {
			// Output available, show report
			output.data = result
		}
	}()

	// No handler
	if receiver == nil {
		output.code = 404
		output.data = message("handler not found")
		return
	}

	// Prepare request object
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

	// Find handler and handle
	item := receiver.allocator()
	switch input.method {
	case "GET":
		output.code = 200
		get, ok := item.(gondulapi.Getter)
		if !ok {
			output.code = 405
			output.data = message("method not allowed")
			return
		}
		result = get.Get(&request)
		output.data = get
	case "POST":
		output.code = 200
		if len(input.data) > 0 {
			if err := json.Unmarshal(input.data, &item); err != nil {
				output.code = 400
				output.data = message("malformed data")
				return
			}
		}
		post, ok := item.(gondulapi.Poster)
		if !ok {
			output.code = 405
			output.data = message("method not allowed")
			return
		}
		result = post.Post(&request)
	case "PUT":
		output.code = 200
		if len(input.data) > 0 {
			if err := json.Unmarshal(input.data, &item); err != nil {
				output.code = 400
				output.data = message("malformed data")
				return
			}
		}
		put, ok := item.(gondulapi.Putter)
		if !ok {
			output.code = 405
			output.data = message("method not allowed")
			return
		}
		result = put.Put(&request)
	case "DELETE":
		output.code = 200
		del, ok := item.(gondulapi.Deleter)
		if !ok {
			output.code = 405
			output.data = message("method not allowed")
			return
		}
		result = del.Delete(&request)
	default:
		output.code = 405
		output.data = message("method not allowed")
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
	if output.code >= 200 && output.code <= 299 {
		log.WithFields(log.Fields{
			"code":     output.code,
			"location": output.location,
		}).Trace("Request done")
	} else {
		log.WithFields(log.Fields{
			"code":     output.code,
			"location": output.location,
			"data":     output.data,
		}).Trace("Request done")
	}

	code := output.code

	// Serialize output data as JSON
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

	// Caching header
	etagraw := sha256.Sum256(b)
	etagstr := hex.EncodeToString(etagraw[:])
	w.Header().Set("ETag", etagstr)

	// Redirect
	if code >= 300 && code <= 399 {
		w.Header().Set("Location", output.location)
	}

	// Finalize head and add body
	w.WriteHeader(code)
	if code != 204 {
		fmt.Fprintf(w, "%s\n", b)
	}
}
