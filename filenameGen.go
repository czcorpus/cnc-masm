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
package main

import "fmt"

func GenWSDefFilename(corpusId string) string {
	return fmt.Sprintf("ws-%s.wsd", corpusId)
}

func GenWSBaseFilename(corpusId string, wsattr string) string {
	return fmt.Sprintf("%s-ws.lex.idx", wsattr)
}

func GenWSThesFilename(corpusId string, wsattr string) string {
	return fmt.Sprintf("%s-thes.idx", wsattr)
}
