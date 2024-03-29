// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Institute of the Czech National Corpus,
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

package main

import (
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
)

type NotFoundHandler struct {
}

func (handler NotFoundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uniresp.WriteJSONErrorResponse(
		w, uniresp.NewActionError("action not found"), http.StatusNotFound)
}

type NotAllowedHandler struct {
}

func (handler NotAllowedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uniresp.WriteJSONErrorResponse(
		w, uniresp.NewActionError("method not allowed"), http.StatusMethodNotAllowed)
}
