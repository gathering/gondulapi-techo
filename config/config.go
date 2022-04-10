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

package config

import (
	"encoding/json"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
)

// Config covers global configuration, and if need be it will provide
// mechanisms for local overrides (similar to Skogul).
var Config struct {
	ListenAddress  string                       `json:"listen_address"`  // Defaults to :8080
	DatabaseString string                       `json:"database_string"` // For database connections
	SitePrefix     string                       `json:"site_prefix"`     // URL prefix, e.g. "/api"
	Debug          bool                         `json:"debug"`           // Enables trace-debugging
	ServerTracks   map[string]ServerTrackConfig `json:"server_tracks"`   // Static config for server tracks
}

// ServerTrackConfig contains the static config for a single server track.
type ServerTrackConfig struct {
	BaseURL      string `json:"base_url"`
	MaxInstances int    `json:"max_instances"`
	AuthUsername string `json:"auth_username"`
	AuthPassword string `json:"auth_password"`
}

// ParseConfig reads a file and parses it as JSON, assuming it will be a
// valid configuration file.
func ParseConfig(file string) bool {
	dat, err := ioutil.ReadFile(file)
	if err != nil {
		log.WithError(err).Fatal("Failed to read config file")
		return false
	}
	if err := json.Unmarshal(dat, &Config); err != nil {
		log.WithError(err).Fatal("Failed to parse config file")
		return false
	}
	log.Tracef("Parsed config file as %v", Config)
	if Config.Debug {
		log.SetLevel(log.TraceLevel)
	}
	return true
}
