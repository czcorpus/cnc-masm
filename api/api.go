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
	"net/http"
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Write(jsonAns)
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
