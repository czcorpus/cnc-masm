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

package laconf

import (
	vteCnf "github.com/czcorpus/vert-tagextract/v2/cnf"
	vteDb "github.com/czcorpus/vert-tagextract/v2/db"
)

// PatchArgs is a subset of vert-tagextract's VTEConf
// used to overwrite stored liveattrs configs - either dynamically
// as part of some actions or to PATCH the config via MASM's REST API.
//
// Please note that when using this type, there is an important distinction
// between an attribute being nil and being of a zero value. The former
// means: "do not update this item in the updated config",
// while the latter says:
// "reset a respective value to its zero value in the updated config"
// This allows us to selectively update different parts of "Liveattrs".
//
// To safely obtain non-nil/non-pointer values, you can use the provided getter methods
// which always replace nil values with respective zero values.
//
// Note: the most important self join functions are: "identity", "intecorp"
type PatchArgs struct {
	VerticalFiles []string            `json:"verticalFiles"`
	MaxNumErrors  *int                `json:"maxNumErrors"`
	AtomStructure *string             `json:"atomStructure"`
	SelfJoin      *vteDb.SelfJoinConf `json:"selfJoin"`
	BibView       *vteDb.BibViewConf  `json:"bibView"`
	Ngrams        *vteCnf.NgramConf   `json:"ngrams"`
}

func (la *PatchArgs) GetVerticalFiles() []string {
	if la.VerticalFiles == nil {
		return []string{}
	}
	return la.VerticalFiles
}

func (la *PatchArgs) GetMaxNumErrors() int {
	if la.MaxNumErrors == nil {
		return 0
	}
	return *la.MaxNumErrors
}

func (la *PatchArgs) GetAtomStructure() string {
	if la.AtomStructure == nil {
		return ""
	}
	return *la.AtomStructure
}

func (la *PatchArgs) GetSelfJoin() vteDb.SelfJoinConf {
	if la.SelfJoin == nil {
		return vteDb.SelfJoinConf{}
	}
	return *la.SelfJoin
}

func (la *PatchArgs) GetBibView() vteDb.BibViewConf {
	if la.BibView == nil {
		return vteDb.BibViewConf{}
	}
	return *la.BibView
}

func (la *PatchArgs) GetNgrams() vteCnf.NgramConf {
	if la.Ngrams == nil {
		return vteCnf.NgramConf{}
	}
	return *la.Ngrams
}
