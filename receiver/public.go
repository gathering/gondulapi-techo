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

/*
Package receiver is scaffolding around net/http that facilitates a
RESTful HTTP API with certain patterns implicitly enforced:

- When working on the same urls, all Methods should use the exact same
data structures. E.g.: What you PUT is the same as what you GET out
again. No cheating.

- ETag is computed for all responses.

- All responses are JSON-encoded, including error messages.

See objects/thing.go for how to use this, but the essence is:

1. Make whatever data structure you need.
2. Implement one or more of gondulapi.Getter/Putter/Poster/Deleter.
3. Use AddHandler() to register that data structure on a URL path
4. Grab lunch.

Receiver tries to do all HTTP and caching-related tasks for you, so you
don't have to.
*/
package receiver

import (
	"fmt"
	"net/http"
	"regexp"

	gapi "github.com/gathering/gondulapi"
	log "github.com/sirupsen/logrus"
)

// AddHandler registeres an allocator/data structure with a url. The
// allocator should be a function returning an empty datastrcuture which
// implements one or more of gondulapi.Getter, Putter, Poster and Deleter
func AddHandler(pathPrefix string, pathPattern string, allocator Allocator) error {
	if receiverSets == nil {
		receiverSets = make(map[string]*receiverSet)
	}

	var set *receiverSet
	if value, exists := receiverSets[pathPrefix]; exists {
		set = value
	} else {
		newSet := receiverSet{pathPrefix: pathPrefix}
		set = &newSet
		receiverSets[pathPrefix] = &newSet
	}

	var compiledPathPattern *regexp.Regexp
	if result, err := regexp.Compile(pathPattern); err == nil {
		compiledPathPattern = result
	} else {
		err := fmt.Errorf("invalid regexp pattern for path: %v", pathPattern)
		log.WithError(err).Error("failed to compile path pattern for handler")
		return err
	}

	receiver := receiver{*compiledPathPattern, allocator}
	set.receivers = append(set.receivers, receiver)
	return nil
}

// Allocator is used to allocate a data structure that implements at least
// one of Getter, Putter, Poster or Deleter from gondulapi.
type Allocator func() interface{}

// Start a net/http server and handle all requests registered. Never
// returns.
func Start() {
	var server http.Server
	serveMux := http.NewServeMux()
	server.Handler = serveMux
	if gapi.Config.ListenAddress == "" {
		server.Addr = ":8080"
	}
	server.Addr = gapi.Config.ListenAddress

	// serveMux.Handle(gapi.Config.SitePrefix+"/auth/", auth.Handler)
	// log.Infof("Added auth handler.")

	if receiverSets != nil {
		for _, set := range receiverSets {
			serveMux.Handle(gapi.Config.SitePrefix+set.pathPrefix, set)
			for _, receiver := range set.receivers {
				log.Infof("Added receiver %v[%v][%v]' for [%T].", gapi.Config.SitePrefix, set.pathPrefix, receiver.pathPattern.String(), receiver.allocator())
			}
		}
	}

	log.WithField("listen_address", server.Addr).Info()
	log.WithField("path_prefix", gapi.Config.SitePrefix).Info()
	log.Fatal(server.ListenAndServe())
}
