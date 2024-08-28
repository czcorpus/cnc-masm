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

package corpdata

import (
	"masm/v3/cnf"
	"masm/v3/general"
)

type registrySubdir struct {
	Name     string `json:"name"`
	ReadOnly bool   `json:"readOnly"`
}

type registry struct {
	RootPaths []string `json:"rootPaths"`
}

type storageLocation struct {
	Data     string   `json:"data"`
	Registry registry `json:"registry"`
	Aligndef string   `json:"aligndef"`
}

// Actions contains all the fsops-related REST actions
type Actions struct {
	conf    *cnf.Conf
	version general.VersionInfo
}

// NewActions is the default factory
func NewActions(conf *cnf.Conf, version general.VersionInfo) *Actions {
	return &Actions{conf: conf, version: version}
}
