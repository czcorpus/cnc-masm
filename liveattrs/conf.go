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

package liveattrs

import (
	vtedb "github.com/czcorpus/vert-tagextract/v3/db"
)

type Conf struct {
	DB *vtedb.Conf `json:"db"`

	// TextTypesDbDirPath is an alternative to DB attribute
	// when dealing with SQLite3-based setup
	TextTypesDbDirPath   string `json:"textTypesDbDirPath"`
	ConfDirPath          string `json:"confDirPath"`
	VertMaxNumErrors     int    `json:"vertMaxNumErrors"`
	VerticalFilesDirPath string `json:"verticalFilesDirPath"`
}

type NgramDBConf struct {
	URL             string   `json:"url"`
	ReadAccessUsers []string `json:"readAccessUsers"`
}
