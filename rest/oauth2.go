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

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gathering/tech-online-backend/config"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

// Oauth2LoginResponse is the object for OAuth2 login requests.
type Oauth2LoginResponse struct {
	User  User             `json:"user"`
	Token AccessTokenEntry `json:"token"`
}

// Oauth2InfoResponse is the object for OAuth2 info requests.
type Oauth2InfoResponse struct {
	ClientID string `json:"client_id"`
	AuthURL  string `json:"auth_url"`
}

type unicornProfile struct {
	ID           uuid.UUID `json:"uuid"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	EmailAddress string    `json:"email"`
}

func init() {
	AddHandler("/oauth2/info/", "^$", func() interface{} { return &Oauth2InfoResponse{} })
	AddHandler("/oauth2/login/", "^$", func() interface{} { return &Oauth2LoginResponse{} })
}

// Get gets OAuth2 info.
func (response *Oauth2InfoResponse) Get(request *Request) Result {
	response.ClientID = config.Config.OAuth2.ClientID
	response.AuthURL = config.Config.OAuth2.AuthURL
	return Result{}
}

// Post attempts to login using OAuth2.
func (response *Oauth2LoginResponse) Post(request *Request) Result {
	oauth2Config := makeOAuth2Config()

	// Check for provided code
	oauth2Code, oauth2CodeOk := request.QueryArgs["code"]
	if !oauth2CodeOk {
		return Result{Code: 400, Message: "No code provided"}
	}

	// Exchange code for token
	oauth2Token, oauth2TokenExchangeErr := oauth2Config.Exchange(context.TODO(), oauth2Code)
	if oauth2TokenExchangeErr != nil {
		log.WithError(oauth2TokenExchangeErr).Trace("OAuth2: Token exchange failed")
		return Result{Code: 400, Message: "IdP didn't accept the provided code"}
	}
	// TODO
	oauth2AccessToken := oauth2Token.AccessToken
	log.Tracef("Got token: %v", oauth2Token)

	// Get profile from Unicorn
	httpRequest, httpRequestErr := http.NewRequest("GET", config.Config.Unicorn.ProfileURL, nil)
	if httpRequestErr != nil {
		return Result{Code: 500, Error: httpRequestErr}
	}
	httpRequest.Header.Set("Authorization", "Bearer "+oauth2AccessToken)
	client := &http.Client{}
	httpResponse, httpResponseErr := client.Do(httpRequest)
	if httpResponseErr != nil {
		log.WithError(httpResponseErr).Warn("OAuth2: Failed to call profile endpoint")
		return Result{Code: 500}
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode > 299 {
		log.Warnf("OAuth2: Failed to read Unicorn profile response data")
		return Result{Code: 500}
	}
	responseBody, responseBodyErr := ioutil.ReadAll(httpResponse.Body)
	if responseBodyErr != nil {
		log.WithError(responseBodyErr).Warn("OAuth2: Failed to read Unicorn profile response data")
		return Result{Code: 500}
	}
	var profile *unicornProfile
	if err := json.Unmarshal(responseBody, profile); err != nil {
		log.WithError(err).Warn("OAuth2: Failed to unmarshal Unicorn profile")
		return Result{Code: 500}
	}

	// TODO
	log.Tracef("OAuth2 profile data (demo): %v", string(responseBody))
	log.Tracef("Unicorn profile data (demo): %v", profile)

	// Update user
	user := getUserByID(profile.ID)
	if user == nil {
		user = &User{ID: &profile.ID}
	}
	user.Username = profile.Username
	user.DisplayName = profile.DisplayName
	user.EmailAddress = profile.EmailAddress
	if user.Role == "" {
		user.Role = DefaultUserRole
	}
	if err := user.save(); err != nil {
		log.WithError(err).Warn("OAuth2: Failed to save new or updated user")
		return Result{Code: 500}
	}

	// Create access token
	token, tokenErr := CreateUserAccessToken(user)
	if tokenErr != nil {
		log.WithError(tokenErr).Warn("OAuth2: Failed to create new access token for user")
		return Result{Code: 500}
	}

	response.Token = *token
	response.User = *user
	return Result{}
}

// makeOAuth2Config creates/loads the OAuth2 config from the main config.
func makeOAuth2Config() oauth2.Config {
	return oauth2.Config{
		ClientID:     config.Config.OAuth2.ClientID,
		ClientSecret: config.Config.OAuth2.ClientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: config.Config.OAuth2.TokenURL,
		},
		RedirectURL: config.Config.OAuth2.RedirectURL,
		// Scopes: []string{"all"},
	}
}
