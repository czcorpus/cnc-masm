// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Institute of the Czech National Corpus,
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

package manatee

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"log"
	"masm/cnf"
	"net/http"
	"strings"
)

func testEtagValues(headerValue, testValue string) bool {
	for _, item := range strings.Split(headerValue, ", ") {
		if strings.HasPrefix(item, "\"") && strings.HasSuffix(item, "\"") {
			val := item[1 : len(item)-1]
			if val == testValue {
				return true
			}

		} else {
			log.Printf("WARNING: Invalid ETag value: %s", item)
		}
	}
	return false
}

// Actions wraps liveattrs-related actions
type Actions struct {
	conf *cnf.Conf
}

// DynamicFunctions provides a list of Manatee internal + our configured functions
// for generating dynamic attributes
func (a *Actions) DynamicFunctions(w http.ResponseWriter, req *http.Request) {
	fullList := dynFnList[:]
	fullList = append(fullList, DynFn{
		Name:        "geteachncharbysep",
		Args:        []string{"n"},
		Description: "Separate a string by \"|\" and return all the pos-th elements from respective items",
		Dynlib:      a.conf.CorporaSetup.ManateeDynlibPath,
	})

	jsonAns, err := json.Marshal(fullList)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", 3600*24*30))
		crc := crc32.ChecksumIEEE(jsonAns)
		newEtag := fmt.Sprintf("dfn-%d", crc)
		reqEtagString := req.Header.Get("If-Match")
		if testEtagValues(reqEtagString, newEtag) {
			http.Error(w, http.StatusText(http.StatusNotModified), http.StatusNotModified)

		} else {
			w.Header().Set("Etag", newEtag)
			w.Write(jsonAns)
		}
	}
}

// NewActions is the default factory for Actions
func NewActions(
	conf *cnf.Conf,
) *Actions {
	return &Actions{
		conf: conf,
	}
}
