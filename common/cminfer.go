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

package common

import (
	"fmt"
	"masm/v3/liveattrs/db/freqdb"
	"os"
	"path/filepath"

	"github.com/czcorpus/rexplorer/parser"
)

type SupportedTagset string

// Validate tests whether the value is one of known types.
// Please note that the empty value is also considered OK
// (otherwise we wouldn't have a valid zero value)
func (st SupportedTagset) Validate() error {
	if st == TagsetCSCNC2000SPK || st == TagsetCSCNC2000 || st == TagsetCSCNC2020 || st == "" {
		return nil
	}
	return fmt.Errorf("invalid tagset type: %s", st)
}

func (st SupportedTagset) String() string {
	return string(st)
}

const (
	TagsetCSCNC2000SPK SupportedTagset = "cs_cnc2000_spk"
	TagsetCSCNC2000    SupportedTagset = "cs_cnc2000"
	TagsetCSCNC2020    SupportedTagset = "cs_cnc2020"
)

func InferQSAttrMapping(regPath string, tagset SupportedTagset) (freqdb.QSAttributes, error) {
	ans := freqdb.QSAttributes{
		Word:     -1,
		Sublemma: -1,
		Lemma:    -1,
		Tag:      -1,
		Pos:      -1,
	}
	regBytes, err := os.ReadFile(regPath)
	if err != nil {
		return ans, fmt.Errorf("failed to infer qs attribute mapping: %w", err)
	}
	doc, err := parser.ParseRegistryBytes(filepath.Base(regPath), regBytes)
	if err != nil {
		return ans, fmt.Errorf("failed to infer qs attribute mapping: %w", err)
	}
	var i int
	for _, attr := range doc.PosAttrs {
		if attr.GetProperty("DYNAMIC") == "" {
			switch attr.Name {
			case freqdb.AttrWord:
				ans.Word = i
			case freqdb.AttrSublemma:
				ans.Sublemma = i
			case freqdb.AttrLemma:
				ans.Lemma = i
			case freqdb.AttrTag:
				ans.Tag = i
			case freqdb.AttrPos:
				ans.Pos = i
			}
			i++
		}
	}
	return ans, nil
}
