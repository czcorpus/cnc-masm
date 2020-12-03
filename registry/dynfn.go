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

package registry

import (
	"encoding/json"
	"strings"
)

// note: the function description data are taken from https://www.sketchengine.eu/dynamic-functions/

// DynFn describes Manatee dynamic function
// (either an internal one or some external)
type DynFn struct {
	Name        string
	Args        []string
	Dynlib      string
	Description string
}

func (df *DynFn) Funtype() string {
	ans := make([]string, len(df.Args))
	for i, arg := range df.Args {
		switch arg {
		case "str":
			ans[i] = "s"
		case "n":
			ans[i] = "i"
		case "c":
			ans[i] = "c"
		case "pointer":
			ans[i] = "0"
		}
	}
	return strings.Join(ans, "")
}

func (df DynFn) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Name        string   `json:"name"`
		Args        []string `json:"args"`
		Dynlib      string   `json:"dynlib"`
		Description string   `json:"description"`
		Funtype     string   `json:"funtype"`
	}{
		Name:        df.Name,
		Args:        df.Args,
		Dynlib:      df.Dynlib,
		Description: df.Description,
		Funtype:     df.Funtype(),
	})
}

var dynFnList = []DynFn{
	{"striplastn", []string{"str", "n"}, "internal", "returns str striped from last n characters"},
	{"lowercase", []string{"str", "locale"}, "internal", "returns str in lowercase (for any single-byte encoding and the corresponding locale"},
	{"utf8lowercase", []string{"str"}, "internal", "returns str in lowercase (for any utf-8 encoded string str"},
	{"utf8uppercase", []string{"str"}, "internal", "returns str in uppercase (for any utf-8 encoded string str"},
	{"utf8capital", []string{"str"}, "internal", "returns str with first character capitalized (for any utf-8 encoded string str"},
	{"getfirstn", []string{"str", "n"}, "internal", "returns first n characters of str"},
	{"getlastn", []string{"str", "n"}, "internal", "returns last n characters of str (for any single-byte encoding)"},
	{"utf8getlastn", []string{"str", "n"}, "internal", "returns last n characters of str (for any utf-8 encoded string)"},
	{"getfirstbysep", []string{"str, c"}, "internal", "returns prefix of str up to the character c (excluding)"},
	{"getnbysep", []string{"str", "c", "n"}, "internal", "returns n-th component of str according to the delimiter c (excluding)"},
	{"getnchar", []string{"str", "n"}, "internal", "returns n-th character of str"},
	{"getnextchars", []string{"str", "c", "n"}, "internal", "returns n characters after character c"},
	{"getnextchar", []string{"str", "c"}, "internal", "returns the character after character c"},
	{"url2domain", []string{"str", "n"}, "internal", "returns n-th component of the URL (0 = web domain, 1 = top level domain, 2 = second level domain)"},
	{"ascii", []string{"str", "enc", "locale"}, "internal", "returns ASCII transliteration of the string according to the given encoding and locale"},
}
