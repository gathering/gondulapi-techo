/*
Gondul GO API, auth
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

package auth

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gathering/gondulapi"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

// OAuth2Authenticator is a HTTP handler for OAuth2 authorization code flow authentication of clients.
type OAuth2Authenticator struct {
	config *oauth2.Config
}

func (authenticator OAuth2Authenticator) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	log.WithFields(log.Fields{
		"url":    request.URL,
		"method": request.Method,
		"client": request.RemoteAddr,
	}).Infof("Request (auth)")

	code := request.URL.Query().Get("code")
	if code == "" {
		log.Tracef("No authorization code provided, redirecting to IdP.")
		http.Redirect(writer, request, authenticator.config.AuthCodeURL(""), 303)
		return
	}

	log.Tracef("Authorization code provided, exchanging it with IdP for token internally.")
	token, err := authenticator.config.Exchange(context.Background(), code)
	if err != nil {
		// TODO JSON error message
		writer.Write([]byte("Failed to exchange authorization code for token. Invalid code?"))
		writer.WriteHeader(400)
		return
	}
	accessToken := token.AccessToken

	// TODO fetch profile and update local profile
	// TODO create session key to give to back to client
	log.Tracef("Got access token: %v", token.AccessToken)
	authenticator.demo(accessToken)
}

func (authenticator *OAuth2Authenticator) demo(accessToken string) error {
	accountURL := "https://unicorn.zoodo.io/api/accounts/users/@me/"
	request, requestErr := http.NewRequest("GET", accountURL, nil)
	if requestErr != nil {
		return requestErr
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	response, responseErr := client.Do(request)
	if responseErr != nil {
		return responseErr
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("response contained non-2XX status: %v", response.Status)
	}

	responseBody, responseBodyErr := ioutil.ReadAll(response.Body)
	if responseBodyErr != nil {
		return responseBodyErr
	}

	// TODO demo
	log.Tracef("OAuth2 profile data (demo): %v", string(responseBody))

	return nil
}

// MakeAuthenticator initializes an OIDC HTTP handler.
func MakeAuthenticator() OAuth2Authenticator {
	var authenticator OAuth2Authenticator
	authenticator.config = &oauth2.Config{
		ClientID:     gondulapi.Config.OAuth2ClientID,
		ClientSecret: gondulapi.Config.OAuth2ClientSecret,
		RedirectURL:  gondulapi.Config.OAuth2RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  gondulapi.Config.OAuth2AuthorizeURL,
			TokenURL: gondulapi.Config.OAuth2TokenURL,
		},
		// Scopes: []string{"all"},
	}
	return authenticator
}
