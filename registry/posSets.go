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

type PosItem struct {
	Label          string `json:"label"`
	TagSrchPattern string `json:"tagSrchPattern"`
}

type PosSimple struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Pos struct {
	ID          string
	Name        string
	Description string
	Values      []PosItem
}

func (p *Pos) ExportWposlist() string {
	ans := make([]string, len(p.Values)*2+1)
	ans[0] = ""
	for i, v := range p.Values {
		ans[2*i+1] = v.Label
		ans[2*i+2] = v.TagSrchPattern
	}
	return strings.Join(ans, ",")
}

func (p Pos) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ID          string    `json:"id"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
		Values      []PosItem `json:"values"`
		Wposlist    string    `json:"wposlist"`
	}{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Values:      p.Values,
		Wposlist:    p.ExportWposlist(),
	})
}

var (
	posList []Pos = []Pos{
		{
			ID:   "pp_tagset",
			Name: "Prague positional tagset",
			Values: []PosItem{
				{"podstatné jméno", "N.*"},
				{"přídavné jméno", "A.*"},
				{"zájmeno", "P.*"},
				{"číslovka", "C.*"},
				{"sloveso", "V.*"},
				{"příslovce", "D.*"},
				{"předložka", "R.*"},
				{"spojka", "J.*"},
				{"částice", "T.*"},
				{"citoslovce", "I.*"},
				{"interpunkce", "Z.*"},
				{"neznámý", "X.*"},
			},
		},
		{
			ID:   "bnc",
			Name: "BNC tagset",
			Values: []PosItem{
				{"adjective", "AJ."},
				{"adverb", "AV."},
				{"conjunction", "CJ."},
				{"determiner", "AT0"},
				{"noun", "NN."},
				{"noun singular", "NN1"},
				{"noun plural", "NN2"},
				{"preposition", "PR."},
				{"pronoun", "DPS"},
				{"verb", "VV."},
			},
		},
		{
			ID:   "rapcor",
			Name: "PoS from the Rapcor corpus",
			Values: []PosItem{
				{"adjective", "ADJ"},
				{"adverb", "ADV"},
				{"conjunction", "KON"},
				{"determiner", "DET.*"},
				{"interjection", "INT"},
				{"noun", "(NOM|NAM)"},
				{"numeral", "NUM"},
				{"preposition", "PRE.*"},
				{"pronoun", "PRO.*"},
				{"verb", "VER.*"},
				{"full stop", "SENT"},
			},
		},
	}
)
