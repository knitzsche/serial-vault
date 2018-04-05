// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016-2018 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package app

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"

	"time"

	"fmt"

	"github.com/CanonicalLtd/serial-vault/crypt"
	"github.com/CanonicalLtd/serial-vault/datastore"
	"github.com/CanonicalLtd/serial-vault/random"
	svlog "github.com/CanonicalLtd/serial-vault/service/log"
	"github.com/CanonicalLtd/serial-vault/service/response"
	"github.com/snapcore/snapd/asserts"
	"github.com/snapcore/snapd/release"
)

const oneYearDuration = time.Duration(24*365) * time.Hour
const userAssertionRevision = "1"

// SystemUserRequest is the JSON version of the request to create a system-user assertion
type SystemUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	ModelID  int    `json:"model"`
	Since    string `json:"since"`
}

// SystemUserResponse is the response from a system-user creation
type SystemUserResponse struct {
	Success      bool   `json:"success"`
	ErrorCode    string `json:"error_code"`
	ErrorSubcode string `json:"error_subcode"`
	ErrorMessage string `json:"message"`
	Assertion    string `json:"assertion"`
}

// SystemUserAssertion is the API method to generate a signed system-user assertion for a device
func SystemUserAssertion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// Decode the body
	user := SystemUserRequest{}
	err := json.NewDecoder(r.Body).Decode(&user)
	switch {
	// Check we have some data
	case err == io.EOF:
		response.FormatStandardResponse(false, "error-user-data", "", "No system-user data supplied", w)
		return
		// Check for parsing errors
	case err != nil:
		response.FormatStandardResponse(false, "error-decode-json", "", err.Error(), w)
		return
	}

	// Get the model:
	// NOTE: As this operation is available regardless of authentication enabled/disabled, we pass
	// an empty authorization object that should allow us getting any model.
	model, err := datastore.Environ.DB.GetAllowedModel(user.ModelID, datastore.User{})
	if err != nil {
		svlog.Message("USER", "invalid-model", "Cannot find model with the selected ID")
		response.FormatStandardResponse(false, "invalid-model", "", "Cannot find model with the selected ID", w)
		return
	}

	// Check that the model has an active system-user keypair
	if !model.KeyActiveUser {
		svlog.Message("USER", "invalid-model", "The model is linked with an inactive signing-key")
		response.FormatStandardResponse(false, "invalid-model", "", "The model is linked with an inactive signing-key", w)
		return
	}

	// Fetch the account assertion from the database
	account, err := datastore.Environ.DB.GetAccount(model.AuthorityIDUser)
	if err != nil {
		svlog.Message("USER", "account-assertions", err.Error())
		response.FormatStandardResponse(false, "account-assertions", "", "Error retrieving the account assertion from the database", w)
		return
	}

	// Create the system-user assertion headers from the request
	assertionHeaders := userRequestToAssertion(user, model)

	// Sign the system-user assertion using the system-user key
	signedAssertion, err := datastore.Environ.KeypairDB.SignAssertion(asserts.SystemUserType, assertionHeaders, nil, model.AuthorityIDUser, model.KeyIDUser, model.SealedKeyUser)
	if err != nil {
		svlog.Message("USER", "signing-assertion", err.Error())
		response.FormatStandardResponse(false, "signing-assertion", "", err.Error(), w)
		return
	}

	// Get the signed assertion
	serializedAssertion := asserts.Encode(signedAssertion)

	// Format the composite assertion
	composite := fmt.Sprintf("%s\n%s\n%s", account.Assertion, model.AssertionUser, serializedAssertion)

	response := SystemUserResponse{Success: true, Assertion: composite}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		svlog.Message("USER", "signing-assertion", err.Error())
	}
}

func userRequestToAssertion(user SystemUserRequest, model datastore.Model) map[string]interface{} {

	// Create the salt from a random string
	reg, _ := regexp.Compile("[^A-Za-z0-9]+")
	randomText, err := random.GenerateRandomString(32)
	if err != nil {
		svlog.Message("USER", "generate-assertion", err.Error())
		return map[string]interface{}{}
	}
	baseSalt := reg.ReplaceAllString(randomText, "")

	// Encrypt the password
	salt := fmt.Sprintf("$6$%s$", baseSalt)
	password := crypt.CLibCryptUser(user.Password, salt)

	// Set the since and end date/times
	since, err := time.Parse("YYYY-MM-DDThh:mm:ssZ00:00", user.Since)
	if err != nil {
		since = time.Now().UTC()
	}
	until := since.Add(oneYearDuration)

	// Create the serial assertion header from the serial-request headers
	headers := map[string]interface{}{
		"type":              asserts.SystemUserType.Name,
		"revision":          userAssertionRevision,
		"authority-id":      model.AuthorityIDUser,
		"brand-id":          model.AuthorityIDUser,
		"email":             user.Email,
		"name":              user.Name,
		"username":          user.Username,
		"password":          password,
		"models":            []interface{}{model.Name},
		"series":            []interface{}{release.Series},
		"since":             since.Format(time.RFC3339),
		"until":             until.Format(time.RFC3339),
		"sign-key-sha3-384": model.KeyIDUser,
	}

	// Create a new serial assertion
	return headers
}