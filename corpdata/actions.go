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
	"masm/v3/corpus"
	"masm/v3/general"
	"net/http"
	"os"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/cnc-gokit/uniresp"
)

type registrySubdir struct {
	Name     string `json:"name"`
	ReadOnly bool   `json:"readOnly"`
}

type registry struct {
	RootPaths []string         `json:"rootPaths"`
	SubDirs   []registrySubdir `json:"subDirs"`
}

type storageLocation struct {
	Data     string   `json:"data"`
	Registry registry `json:"registry"`
}

// Actions contains all the fsops-related REST actions
type Actions struct {
	conf    *corpus.Conf
	version general.VersionInfo
}

func (a *Actions) OnExit() {}

// AvailableDataLocations provides pairs of registry_path=>data_path available
// to a user
func (a *Actions) AvailableDataLocations(w http.ResponseWriter, req *http.Request) {
	location := &storageLocation{
		Registry: registry{
			RootPaths: make([]string, 0, 10),
			SubDirs:   make([]registrySubdir, 0, 50),
		},
		Data: a.conf.CorporaSetup.CorpusDataPath.Abstract,
	}
	subdirs := make(map[string]bool) // path => readonly

	for _, regPathRoot := range a.conf.CorporaSetup.RegistryDirPaths {
		regPaths, err := fs.ListDirsInDir(regPathRoot, false)
		if err != nil {
			uniresp.WriteJSONErrorResponse(
				w, uniresp.NewActionError("failed to get data locations: %w", err), http.StatusInternalServerError)
			return
		}
		regPaths.ForEach(func(info os.FileInfo, idx int) bool {
			subdirs[info.Name()] = a.conf.CorporaSetup.SubdirIsInAltAccessMapping(info.Name())
			return true
		})
		location.Registry.RootPaths = append(location.Registry.RootPaths, regPathRoot)
	}
	for name, readonly := range subdirs {
		location.Registry.SubDirs = append(
			location.Registry.SubDirs,
			registrySubdir{Name: name, ReadOnly: readonly})
	}
	uniresp.WriteJSONResponse(w, location)
}

// NewActions is the default factory
func NewActions(conf *corpus.Conf, version general.VersionInfo) *Actions {
	return &Actions{conf: conf, version: version}
}
