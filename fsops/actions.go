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

package fsops

import (
	"masm/api"
	"masm/cnf"
	"net/http"
	"os"
	"path/filepath"
)

type storageLocation struct {
	Registry string `json:"registry"`
	Data     string `json:"data"`
	ReadOnly bool   `json:"readOnly"`
}

// Actions contains all the fsops-related REST actions
type Actions struct {
	conf    *cnf.Conf
	version cnf.VersionInfo
}

func (a *Actions) OnExit() {}

// AvailableDataLocations provides pairs of registry_path=>data_path available
// to a user
func (a *Actions) AvailableDataLocations(w http.ResponseWriter, req *http.Request) {
	regPaths, err := ListFilesInDir(a.conf.CorporaSetup.RegistryDirPath, false)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	dataPaths, err := ListFilesInDir(a.conf.CorporaSetup.CorpusDataPath.Abstract, false)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	ans := make([]*storageLocation, 0, 100)
	ans = append(
		ans,
		&storageLocation{
			Registry: a.conf.CorporaSetup.RegistryDirPath,
			Data:     a.conf.CorporaSetup.CorpusDataPath.Abstract,
		},
	)
	for regPath, dataPath := range a.conf.CorporaSetup.AltAccessMapping {
		ans = append(
			ans,
			&storageLocation{
				Registry: filepath.Join(a.conf.CorporaSetup.RegistryDirPath, regPath),
				Data:     filepath.Join(a.conf.CorporaSetup.CorpusDataPath.Abstract, dataPath),
				ReadOnly: true,
			},
		)
	}

	regPaths.ForEach(func(item os.FileInfo, i int) bool {
		bs := filepath.Base(item.Name())
		if item.IsDir() && !a.conf.CorporaSetup.SubdirIsInAltAccessMapping(item.Name()) {
			dataPaths.ForEach(func(item2 os.FileInfo, i int) bool {
				if item2.IsDir() {
					bs2 := filepath.Base(item2.Name())
					if bs == bs2 {
						ans = append(ans, &storageLocation{
							Data:     filepath.Join(a.conf.CorporaSetup.CorpusDataPath.Abstract, item.Name()),
							Registry: filepath.Join(a.conf.CorporaSetup.RegistryDirPath, item2.Name()),
						})
						return false
					}
				}
				return true
			})
			return true
		}
		return true
	})
	api.WriteJSONResponse(w, ans)
}

// NewActions is the default factory
func NewActions(conf *cnf.Conf, version cnf.VersionInfo) *Actions {
	return &Actions{conf: conf, version: version}
}
