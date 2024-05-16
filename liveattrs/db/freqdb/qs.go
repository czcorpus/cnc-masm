// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Institute of the Czech National Corpus,
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

package freqdb

import "fmt"

type QSAttributes struct {
	Word     string
	Sublemma string
	Lemma    string
	Tag      string
}

// ExportCols exports columns based on their universal
// names "word", "lemma", "sublemma", "tag"
// So if e.g. Word == "col0", Lemma == "col3", Sublemma == "col5"
// and one requires ExportCols("word", "sublemma", "lemma", "sublemma")
// then the method returns []string{"col0", "col5", "col3", "col5"}
func (qsa QSAttributes) ExportCols(values ...string) []string {
	ans := make([]string, 0, len(values))
	for _, v := range values {
		switch v {
		case "word":
			ans = append(ans, qsa.Word)
		case "lemma":
			ans = append(ans, qsa.Lemma)
		case "sublemma":
			ans = append(ans, qsa.Sublemma)
		case "tag":
			ans = append(ans, qsa.Tag)
		default:
			panic(fmt.Sprintf("unknown query suggestion attribute: %s", v))
		}
	}
	return ans
}

func (qsa QSAttributes) ExportCol(name string) string {
	return qsa.ExportCols(name)[0]
}
