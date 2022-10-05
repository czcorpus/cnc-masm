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
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type ClientBase struct {
	BaseURL string
	DBName  string
}

func (cb *ClientBase) DoRequest(method string, spath string, data Storable) (string, error) {
	url, err := url.Parse(cb.BaseURL)
	if err != nil {
		return "", err
	}
	url.Path = path.Join(url.Path, cb.DBName, spath)
	var dataBytes []byte
	if data != nil {
		dataBytes, err = data.ToJSON()
		if err != nil {
			return "", err
		}
	}
	req, err := http.NewRequest(method, url.String(), strings.NewReader(string(dataBytes)))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	if err != nil {
		return "", err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
