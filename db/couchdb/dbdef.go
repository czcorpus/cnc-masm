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
	"fmt"
	"net/http"
)

/*
NOTE:

To create a new CouchDB user, insert the following record to the '_users' database:

{
    "_id": "org.couchdb.user:user_name",
    "name": "user_name",
    "type": "user",
    "roles": [],
    "password": "user_password"
}

*/

const (
	designDocName = "search"
	viewName      = "any-form"
	viewDef       = `function (doc) {
	emit(doc.lemma, 'lemma');
	for (var i = 0; i < doc.forms.length; i++) {
	  emit(doc.forms[i].word.toLowerCase(), 'word');
	};
	for (var i = 0; i < doc.sublemmas.length; i++) {
	  emit(doc.sublemmas[i].value.toLowerCase(), 'sublemma');
	};
  }`
)

type viewSpecif struct {
	Map    string `json:"map"`
	Reduce string `json:"reduce,omitempty"`
}

type createViewsPayload struct {
	Views map[string]viewSpecif `json:"views"`
}

func (cvp *createViewsPayload) ToJSON() ([]byte, error) {
	return json.Marshal(cvp)
}

type securityEntry struct {
	Names []string `json:"names"`
	Roles []string `json:"roles"`
}

type security struct {
	Admins  securityEntry `json:"admins"`
	Members securityEntry `json:"members"`
}

func (sec *security) ToJSON() ([]byte, error) {
	return json.Marshal(sec)
}

type Schema struct {
	db *ClientBase
}

func (db *Schema) CreateDatabase(readAccessUsers []string) error {
	_, err := db.db.DoRequest(
		http.MethodDelete,
		"",
		nil,
	)
	if err != nil {
		return err
	}
	_, err = db.db.DoRequest(
		http.MethodPut,
		"",
		nil,
	)
	if err != nil {
		return err
	}
	_, err = db.db.DoRequest(
		http.MethodPut,
		"_security",
		&security{
			Admins:  securityEntry{Names: []string{}, Roles: []string{}},
			Members: securityEntry{Names: readAccessUsers, Roles: []string{}},
		},
	)
	return err

}

func (db *Schema) CreateViews() error {
	_, err := db.db.DoRequest(
		http.MethodPut,
		fmt.Sprintf("_design/%s", designDocName),
		&createViewsPayload{
			Views: map[string]viewSpecif{
				viewName: {Map: viewDef},
			},
		},
	)
	return err
}

func NewSchema(db *ClientBase) *Schema {
	return &Schema{
		db: db,
	}
}
