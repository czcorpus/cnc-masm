// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of CNC-MASM.
//
//  CNC-MASM is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  CNC-MASM is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with CNC-MASM.  If not, see <https://www.gnu.org/licenses/>.

package api

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

// ActionError represents a basic user action error (e.g. a wrong parameter,
// non-existing record etc.)
type ActionError struct {
	error
}

// MarshalJSON serializes the error to JSON
func (me ActionError) MarshalJSON() ([]byte, error) {
	return json.Marshal(me.Error())
}

// NewActionErrorFrom is the default factory for creating an ActionError instance
// out of an existing error
func NewActionErrorFrom(origErr error) ActionError {
	return ActionError{origErr}
}

// NewActionError creates an Action error from provided message using
// a newly defined general error as an original error
func NewActionError(msg string, args ...interface{}) ActionError {
	return ActionError{fmt.Errorf(msg, args...)}
}

// ErrorResponse describes a wrapping object for all error HTTP responses
type ErrorResponse struct {
	Error   *ActionError `json:"error"`
	Details []string     `json:"details"`
}

// WriteJSONResponse writes 'value' to an HTTP response encoded as JSON
func WriteJSONResponse(w http.ResponseWriter, value interface{}) {
	jsonAns, err := json.Marshal(value)
	if err != nil {
		log.Err(err).Msg("failed to encode a result to JSON")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(jsonAns)
}

// WriteJSONResponseWithStatus writes 'value' to an HTTP response encoded as JSON
func WriteJSONResponseWithStatus(w http.ResponseWriter, status int, value interface{}) {
	jsonAns, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	w.Write(jsonAns)
}

func testEtagValues(headerValue, testValue string) bool {
	if headerValue == "" {
		return false
	}
	for _, item := range strings.Split(headerValue, ", ") {
		if strings.HasPrefix(item, "\"") && strings.HasSuffix(item, "\"") {
			val := item[1 : len(item)-1]
			if val == testValue {
				return true
			}

		} else {
			log.Warn().Msgf("Invalid ETag value: %s", item)
		}
	}
	return false
}

// WriteCacheableJSONResponse writes 'value' to an HTTP response encoded as JSON
// but before doing that it calculates a checksum of the JSON and in case it is
// equal to provided 'If-Match' header, 304 is returned. Otherwise a value with
// ETag header is returned.
func WriteCacheableJSONResponse(w http.ResponseWriter, req *http.Request, value interface{}) {
	jsonAns, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", 3600*24*30))
		crc := crc32.ChecksumIEEE(jsonAns)
		newEtag := fmt.Sprintf("masm-%d", crc)
		reqEtagString := req.Header.Get("If-Match")
		if testEtagValues(reqEtagString, newEtag) {
			http.Error(w, http.StatusText(http.StatusNotModified), http.StatusNotModified)

		} else {
			w.Header().Set("Etag", newEtag)
			w.Write(jsonAns)
		}
	}
}

// WriteJSONErrorResponse writes 'aerr' to an HTTP error response  as JSON
func WriteJSONErrorResponse(w http.ResponseWriter, aerr ActionError, status int, details ...string) {
	ans := &ErrorResponse{Error: &aerr, Details: details}
	jsonAns, err := json.Marshal(ans)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.WriteHeader(status)
	w.Write(jsonAns)
}
