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

package actions

import (
	"errors"
	"masm/v3/liveattrs/qs"

	"github.com/czcorpus/vert-tagextract/v2/cnf"
	"github.com/czcorpus/vert-tagextract/v2/ptcount/modders"
)

var (
	errorPosNotDefined = errors.New("PoS not defined")
)

func appendPosModder(prev string, curr qs.SupportedTagset) string {
	if prev == "" {
		return string(curr)
	}
	return prev + ":" + string(curr)
}

// posExtractorFactory creates a proper modders.StringTransformer instance
// to extract PoS in MASM and also a string representation of it for proper
// vert-tagexract configuration.
func posExtractorFactory(
	currMods string,
	tagsetName qs.SupportedTagset,
) (*modders.StringTransformerChain, string) {
	modderSpecif := appendPosModder(currMods, tagsetName)
	return modders.NewStringTransformerChain(modderSpecif), modderSpecif
}

// applyPosProperties takes posIdx and posTagset and adds a column modder
// to Ngrams.columnMods column matching the "PoS" one (preserving string modders
// already configured there!).
func applyPosProperties(
	conf *cnf.VTEConf,
	posIdx int,
	posTagset qs.SupportedTagset,
) (*modders.StringTransformerChain, error) {
	for i, col := range conf.Ngrams.VertColumns {
		if posIdx == col.Idx {
			fn, modderSpecif := posExtractorFactory(col.ModFn, posTagset)
			col.ModFn = modderSpecif
			conf.Ngrams.VertColumns[i] = col
			return fn, nil
		}
	}
	return modders.NewStringTransformerChain(""), errorPosNotDefined
}
