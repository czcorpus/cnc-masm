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
	"time"
)

// JSONTime is a customized time data type with predefined
// JSON serialization
type JSONTime time.Time

func (t JSONTime) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return json.Marshal(nil)
	}
	return []byte("\"" + time.Time(t).Format(time.RFC3339) + "\""), nil
}

func (t JSONTime) Before(t2 JSONTime) bool {
	return time.Time(t).Before(time.Time(t2))
}

func (t JSONTime) Sub(t2 JSONTime) time.Duration {
	return time.Time(t).Sub(time.Time(t2))
}

func (t JSONTime) Format(layout string) string {
	return time.Time(t).Format(layout)
}

func (t JSONTime) IsZero() bool {
	return time.Time(t).IsZero()
}

func CurrentDatetime() JSONTime {
	return JSONTime(time.Now())
}
