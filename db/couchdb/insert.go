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

package couchdb

import (
	"encoding/json"
	"net/http"
)

type Storable interface {
	ToJSON() ([]byte, error)
}

type bulkDocs[T Storable] struct {
	Docs []T `json:"docs"`
}

func (bd *bulkDocs[T]) ToJSON() ([]byte, error) {
	return json.Marshal(bd)
}

// DocHandler is a CouchDB client specifically for
// handling concrete type of document
type DocHandler[T Storable] struct {
	db *ClientBase
}

func (c *DocHandler[T]) BulkInsert(values []T) error {
	bdocs := bulkDocs[T]{
		Docs: make([]T, len(values)),
	}
	copy(bdocs.Docs, values)
	_, err := c.db.DoRequest(http.MethodPost, "_bulk_docs", &bdocs)
	return err
}

func NewDocHandler[T Storable](cb *ClientBase) *DocHandler[T] {
	return &DocHandler[T]{
		db: cb,
	}
}
