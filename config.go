/*
Gondul GO API
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

package gondulapi

import (
	"encoding/json"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
)

// Config covers global configuration, and if need be it will provide
// mechanisms for local overrides (similar to Skogul).
var Config struct {
	ListenAddress      string                       `json:"listen_address"`       // Defaults to :8080
	DB                 string                       `json:"db"`                   // For database connections
	SitePrefix         string                       `json:"site_prefix"`          // URL prefix, e.g. "/api"
	Debug              bool                         `json:"debug"`                // Enables trace-debugging
	ServerTracks       map[string]ServerTrackConfig `json:"server_tracks"`        // Static config for server tracks
	OAuth2ClientID     string                       `json:"oauth2_client_id"`     // OAuth2 Client ID
	OAuth2ClientSecret string                       `json:"oauth2_client_secret"` // OAuth2 Client Secret
	OAuth2AuthorizeURL string                       `json:"oauth2_authorize_url"` // OAuth2 authorize URL
	OAuth2TokenURL     string                       `json:"oauth2_token_url"`     // OAuth2 token URL
	OAuth2RedirectURL  string                       `json:"oauth2_redirect_url"`  // OAuth2 redirect URL
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
func ParseConfig(file string) error {
	dat, err := ioutil.ReadFile(file)
	if err != nil {
		log.WithError(err).Fatal("Failed to read config file")
		return Error{500, "Failed to read config file"}
	}
	if err := json.Unmarshal(dat, &Config); err != nil {
		log.WithError(err).Fatal("Failed to parse config file")
		return err
	}
	log.Tracef("Parsed config file as %v", Config)
	if Config.Debug {
		log.SetLevel(log.TraceLevel)
	}
	return nil
}
