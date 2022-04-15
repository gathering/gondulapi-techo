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
	"net/url"

	"github.com/gathering/tech-online-backend/config"
	"github.com/gathering/tech-online-backend/db"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

// Oauth2LoginData is the object for OAuth2 login requests.
type Oauth2LoginData struct {
	User  User             `json:"user"`
	Token AccessTokenEntry `json:"token"`
}

// Oauth2LogoutData is the object for OAuth2 login requests.
type Oauth2LogoutData struct{}

// Oauth2InfoData is the object for OAuth2 info requests.
type Oauth2InfoData struct {
	ClientID    string `json:"client_id"`
	AuthURL     string `json:"auth_url"`
	RedirectURL string `json:"redirect_url"`
}

type unicornProfile struct {
	ID           uuid.UUID `json:"uuid"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	EmailAddress string    `json:"email"`
}

func init() {
	AddHandler("/oauth2/info/", "^$", func() interface{} { return &Oauth2InfoData{} })
	AddHandler("/oauth2/login/", "^$", func() interface{} { return &Oauth2LoginData{} })
	AddHandler("/oauth2/logout/", "^$", func() interface{} { return &Oauth2LogoutData{} })
}

// Get gets OAuth2 info.
func (response *Oauth2InfoData) Get(request *Request) Result {
	response.ClientID = config.Config.OAuth2.ClientID
	response.AuthURL = config.Config.OAuth2.AuthURL
	response.RedirectURL = config.Config.OAuth2.RedirectURL
	return Result{}
}

// Post attempts to login using OAuth2.
func (response *Oauth2LoginData) Post(request *Request) Result {
	oauth2Config := makeOAuth2Config()

	// Check for provided code
	oauth2Code, oauth2CodeFound := request.QueryArgs["code"]
	if !oauth2CodeFound {
		return Result{Code: 400, Message: "No code provided"}
	}

	// Check for alternative redirect URL (only allows variations with host=localhost for testing purposes)
	rawNewRedirectURL, redirectURLFound := request.QueryArgs["redirect-url"]
	if redirectURLFound {
		newRedirectURL, newRedirectURLErr := url.Parse(rawNewRedirectURL)
		if newRedirectURLErr != nil {
			return Result{Code: 400, Message: "Invalid redirect URL provided"}
		}
		if rawNewRedirectURL != oauth2Config.RedirectURL && newRedirectURL.Hostname() != "localhost" {
			return Result{Code: 400, Message: "Illegal redirect URL provided"}
		}
		oauth2Config.RedirectURL = newRedirectURL.String()
	}

	// Exchange code for token
	oauth2Token, oauth2TokenExchangeErr := oauth2Config.Exchange(context.TODO(), oauth2Code)
	if oauth2TokenExchangeErr != nil {
		log.WithError(oauth2TokenExchangeErr).Trace("OAuth2: Token exchange failed")
		return Result{Code: 400, Message: "IdP didn't accept the provided code"}
	}

	// Get profile from Unicorn
	httpRequest, httpRequestErr := http.NewRequest("GET", config.Config.Unicorn.ProfileURL, nil)
	if httpRequestErr != nil {
		return Result{Code: 500, Error: httpRequestErr}
	}
	httpRequest.Header.Set("Authorization", "Bearer "+oauth2Token.AccessToken)
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
	if err := json.Unmarshal(responseBody, &profile); err != nil {
		log.WithError(err).Warn("OAuth2: Failed to unmarshal Unicorn profile")
		return Result{Code: 500}
	}

	// Update user
	user := getUserByID(profile.ID)
	if user == nil {
		user = &User{ID: &profile.ID}
	}
	user.Username = profile.Username
	user.DisplayName = profile.DisplayName
	user.EmailAddress = profile.EmailAddress
	if user.Role == "" {
		user.Role = RoleParticipant
	}
	log.Tracef("Got user: %v", user)
	if err := user.save(); err != nil {
		log.WithError(err).Warn("OAuth2: Failed to save new or updated user")
		return Result{Code: 500}
	}

	// Create access token
	token, tokenErr := createUserAccessToken(user)
	if tokenErr != nil {
		log.WithError(tokenErr).Warn("OAuth2: Failed to create new access token for user")
		return Result{Code: 500}
	}

	response.Token = *token
	response.User = *user
	return Result{}
}

// Post deletes the current access token.
// Supports user tokens only.
func (response *Oauth2LogoutData) Post(request *Request) Result {
	if request.AccessToken.OwnerUserID == nil {
		dbResult := db.Delete("access_tokens", "id", "<=", request.AccessToken.ID)
		if dbResult.IsFailed() {
			log.WithError(dbResult.Error).Error("Failed to delete user access token on logout")
		}
		request.AccessToken = makeGuestAccessToken()
	} else {
		return Result{Code: 400, Message: "This access token type doesn't support logouts"}
	}
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
