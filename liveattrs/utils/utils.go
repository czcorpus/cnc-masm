// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Martin Zimandl <martin.zimandl@gmail.com>
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

package utils

import "strings"

func ShortenVal(v string, maxLength int) string {
	if len(v) <= maxLength {
		return v
	}

	words := strings.Split(v, " ")
	length := 0
	for i, word := range words {
		length += len(word)
		if length > maxLength {
			if i == 0 {
				return word[:maxLength] + "..."
			}
			return strings.Join(words[:i], " ") + "..."
		}
	}
	return v
}

func ImportKey(k string) string {
	return strings.Replace(k, ".", "_", 1)
}

func ExportKey(k string) string {
	return strings.Replace(k, "_", ".", 1)
}
