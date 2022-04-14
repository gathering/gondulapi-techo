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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/gathering/tech-online-backend/config"
	"github.com/gathering/tech-online-backend/db"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const tokenLengthBytes = 32
const encodedTokenLengthBytes = 44              // Depends on tokenLengthBytes
const tokenExpirationSeconds = 7 * 24 * 60 * 60 // A week

// Role defines a role for users and tokens.
type Role string

const (
	// RoleInvalid - Invalid.
	RoleInvalid Role = ""
	// RoleGuest - No special access, for non-authenticated requests.
	RoleGuest Role = "guest"
	// RoleParticipant - Access to participate (i.e. logged in). Valid for user tokens only.
	RoleParticipant Role = "participant"
	// RoleOperator - Access to most stuff, but can't create new tracks, push status, etc.
	RoleOperator Role = "operator"
	// RoleAdmin - Access to everything.
	RoleAdmin Role = "admin"
	// RoleTester - Access to push test data, for status scripts. Valid for non-user tokens only.
	RoleTester Role = "tester"
)

// AccessTokenEntry is a collections of access things used for the client to authenticate itself and for the backend to know more about the client.
type AccessTokenEntry struct {
	ID             uuid.UUID  `column:"id" json:"id"`
	Key            string     `column:"key" json:"key,omitempty"`
	OwnerUserID    *uuid.UUID `column:"owner_user" json:"owner_user,omitempty"`       // Optional, not used for e.g. test status scripts.
	NonUserRole    *Role      `column:"non_user_role" json:"non_user_role,omitempty"` // Role if not a user token. Call .GetRole() to get the effective role.
	CreationTime   time.Time  `column:"creation_time" json:"creation_time"`
	ExpirationTime time.Time  `column:"expiration_time" json:"expiration_time"`
	IsStatic       bool       `column:"static" json:"static"` // If the token is static, i.e. defined by the config instead of DB and can't be created or deleted through the API.
	Comment        string     `column:"comment" json:"comment"`
	OwnerUser      *User      `column:"-" json:"-"` // The linked user (if any). Do not modify this object. Call .LoadUser() again if the underlying user is modified.
}

// AccessTokenEntries is multiple AccessTokenEntry.
type AccessTokenEntries []*AccessTokenEntry

func init() {
	AddHandler("/access_tokens/", "^$", func() interface{} { return &AccessTokenEntries{} })
	AddHandler("/access_token/", "^(?:(?P<id>[^/]+)/)?$", func() interface{} { return &AccessTokenEntry{} })
}

// UpdateStaticAccessTokens deletes the previous static tokens and load new ones from the config.
// To be called at least when starting the program.
func UpdateStaticAccessTokens() error {
	// Delete all old static tokens
	dbResult := db.Delete("access_tokens", "static", "=", true)
	if dbResult.IsFailed() {
		return dbResult.Error
	}

	// Create new ones
	for tokenID, tokenConfig := range config.Config.AccessTokens {
		role := (Role)(tokenConfig.Role)
		token := AccessTokenEntry{
			ID:             tokenID,
			Key:            tokenConfig.Key,
			NonUserRole:    &role,
			CreationTime:   time.Now(),
			ExpirationTime: time.Now().AddDate(1000, 0, 0), // + 1000 years
			IsStatic:       true,
			Comment:        tokenConfig.Comment,
		}

		// Validate
		if valRes := token.validateInternal(); valRes != "" {
			log.Warnf("Failed to validate static access token, it will not be added: %v", valRes)
			continue
		}

		// Save
		dbResult := db.Insert("access_tokens", token)
		if dbResult.IsFailed() {
			return dbResult.Error
		}
	}

	return nil
}

// createUserAccessToken creates and saves an access token with a generated ID and key, starting now.
func createUserAccessToken(user *User) (*AccessTokenEntry, error) {
	newKey, newKeyErr := generateAccessTokenKey()
	if newKeyErr != nil {
		return nil, newKeyErr
	}

	token := AccessTokenEntry{
		ID:             uuid.New(),
		Key:            newKey,
		OwnerUserID:    user.ID,
		NonUserRole:    nil,
		CreationTime:   time.Now(),
		ExpirationTime: time.Now().Add(tokenExpirationSeconds * time.Second),
		IsStatic:       false,
		Comment:        fmt.Sprintf("OAuth2: %v", user.Username),
		OwnerUser:      user,
	}

	if valRes := token.validateInternal(); valRes != "" {
		return nil, fmt.Errorf("failed to validate access token: %v", valRes)
	}

	dbResult := db.Insert("access_tokens", token)
	if dbResult.IsFailed() {
		return nil, dbResult.Error
	}

	return &token, nil
}

// loadAccessTokenByKey returns a valid token for the provided key or nil if none exists.
// If a token key header was specified but no valid token could be found for it,
// the request should probably be denied.
func loadAccessTokenByKey(key string) *AccessTokenEntry {
	if key == "" {
		return nil
	}

	// Get from DB, if created and not expired
	var token AccessTokenEntry
	now := time.Now()
	var whereArgs []interface{}
	whereArgs = append(whereArgs, "key", "=", key)
	whereArgs = append(whereArgs, "creation_time", "<=", now)
	whereArgs = append(whereArgs, "expiration_time", ">=", now)
	dbResult := db.Select(&token, "access_tokens", whereArgs...)
	if dbResult.IsFailed() {
		log.WithError(dbResult.Error).Error("Failed to select access token from DB")
		return nil
	}
	if !dbResult.IsSuccess() {
		return nil
	}

	// Load user (if any)
	if token.OwnerUserID != nil {
		user, userErr := loadUser(*token.OwnerUserID)
		token.OwnerUser = user
		if userErr != nil {
			log.WithFields(log.Fields{
				"token_id": token.ID,
				"user_id":  token.OwnerUserID,
			}).WithError(userErr).Warning("Failed to referenced user from token")
			return nil
		}
	}

	return &token
}

// makeGuestAccessToken creates an empty-ish guest access token, such that all requests (authenticated or not) have a role.
func makeGuestAccessToken() *AccessTokenEntry {
	id, _ := uuid.FromBytes([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	role := RoleGuest
	time := time.Now()
	return &AccessTokenEntry{
		ID:             id,
		Key:            "",
		OwnerUserID:    nil,
		NonUserRole:    &role,
		CreationTime:   time,
		ExpirationTime: time,
		IsStatic:       false,
		Comment:        "Guest",
	}
}

// purgeExpiredAccessTokens deletes all expired tokens. Should be called periodically.
func purgeExpiredAccessTokens() {
	now := time.Now()
	dbResult := db.Delete("access_tokens", "expiration_time", "<=", now)
	if dbResult.IsFailed() {
		log.WithError(dbResult.Error).Error("Failed to purge old access tokens")
	}
}

// Generate a Base64-encoded token key using a secure amount of random bytes.
func generateAccessTokenKey() (string, error) {
	buffer := make([]byte, tokenLengthBytes)
	_, err := rand.Read(buffer)
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(buffer)
	return encoded, nil
}

// Validate the token entry.
// If the returned string is non-empty, it contains the user-safe error message and the tokens isn't valid.
// It does not care if the token is "not created yet" or expired.
func (token *AccessTokenEntry) validateInternal() string {
	switch {
	case token.Key == "":
		return "missing key"
	case token.OwnerUserID != nil && token.NonUserRole != nil || token.OwnerUserID == nil && token.NonUserRole == nil:
		return "exactly one of user ID and non-user role must be set"
	}

	return ""
}

// GetRole returns the non-user role if non-user token or the user role if user token.
// Assumes the user is already loaded if user token.
// Returns an empty string (the invalid role) if inconsistent token.
func (token *AccessTokenEntry) GetRole() Role {
	if token.OwnerUser != nil {
		return token.OwnerUser.Role
	}
	if token.NonUserRole != nil {
		return *token.NonUserRole
	}
	return RoleInvalid
}

// Get gets multiple access tokens.
func (tokens *AccessTokenEntries) Get(request *Request) Result {
	var whereArgs []interface{}
	if userID, ok := request.QueryArgs["user"]; ok {
		whereArgs = append(whereArgs, "user", "=", userID)
	}
	if role, ok := request.QueryArgs["role"]; ok {
		whereArgs = append(whereArgs, "role", "=", role)
	}
	if rawStatic, ok := request.QueryArgs["static"]; ok {
		static, err := strconv.ParseBool(rawStatic)
		if err == nil {
			whereArgs = append(whereArgs, "static", "=", static)
		}
	}

	// Limit to only self if not operator/admin
	role := request.AccessToken.GetRole()
	if role != RoleAdmin {
		if request.AccessToken.OwnerUser != nil {
			whereArgs = append(whereArgs, "user", "=", request.AccessToken.OwnerUser.ID)
		} else {
			// No access, just leave
			return Result{}
		}
	}

	dbResult := db.SelectMany(tokens, "access_tokens", whereArgs...)
	if dbResult.IsFailed() {
		return Result{Code: 500, Error: dbResult.Error}
	}

	// Hide key
	for _, token := range *tokens {
		token.Key = ""
	}

	return Result{}
}

// Get gets a single access token.
func (token *AccessTokenEntry) Get(request *Request) Result {
	id, idExists := request.PathArgs["id"]
	if !idExists || id == "" {
		return Result{Code: 400, Message: "missing ID"}
	}

	// Check if self or operator/admin
	role := request.AccessToken.GetRole()
	if role != RoleAdmin {
		if request.AccessToken.OwnerUser != nil && request.AccessToken.OwnerUser.ID.String() != id {
			return Result{Code: 403, Message: "Access denied"}
		}
		if request.AccessToken.OwnerUser == nil {
			return Result{Code: 403, Message: "Access denied"}
		}
	}

	dbResult := db.Select(token, "access_tokens", "id", "=", id)
	if dbResult.IsFailed() {
		return Result{Code: 500, Error: dbResult.Error}
	}
	if !dbResult.IsSuccess() {
		return Result{Code: 404, Message: "not found"}
	}

	// Hide key
	token.Key = ""

	return Result{}
}
