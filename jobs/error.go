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

package jobs

import (
	"encoding/json"
)

// JSONError is a customized error data type with specific JSON
// serialization (e.g. null for no error)
type JSONError struct {
	Message string
}

func (e JSONError) MarshalJSON() ([]byte, error) {
	if e.Message != "" {
		return json.Marshal(e.Message)
	}
	return json.Marshal(nil)
}

func (e *JSONError) Error() string {
	return e.Message
}

func (e *JSONError) IsEmpty() bool {
	return e.Message == ""
}

func NewJSONError(err error) JSONError {
	ans := JSONError{}
	if err != nil {
		ans.Message = err.Error()
	}
	return ans
}
