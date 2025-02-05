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

package corpus

import (
	"path/filepath"

	"github.com/czcorpus/cnc-gokit/fs"
)

const (
	CorpusVariantPrimary CorpusVariant = "primary"
	CorpusVariantLimited CorpusVariant = "omezeni"
)

type CorpusVariant string

func (cv CorpusVariant) SubDir() string {
	if cv == "primary" {
		return ""
	}
	return string(cv)
}

// CorporaSetup defines masm application configuration related
// to a corpus
type CorporaSetup struct {
	RegistryDirPaths     []string          `json:"registryDirPaths"`
	RegistryTmpDir       string            `json:"registryTmpDir"`
	ConcCacheDirPath     string            `json:"concCacheDirPath"`
	AligndefDirPath      string            `json:"aligndefDirPath"`
	AltAccessMapping     map[string]string `json:"altAccessMapping"` // registry => data mapping
	WordSketchDefDirPath string            `json:"wordSketchDefDirPath"`
	ManateeDynlibPath    string            `json:"manateeDynlibPath"`
}

func (cs *CorporaSetup) GetFirstValidRegistry(corpusID, subDir string) string {
	for _, dir := range cs.RegistryDirPaths {
		d := filepath.Join(dir, subDir, corpusID)
		pe := fs.PathExists(d)
		isf, _ := fs.IsFile(d)
		if pe && isf {
			return d
		}
	}
	return ""
}

func (cs *CorporaSetup) SubdirIsInAltAccessMapping(subdir string) bool {
	_, ok := cs.AltAccessMapping[subdir]
	return ok
}

type DatabaseSetup struct {
	Host                     string `json:"host"`
	User                     string `json:"user"`
	Passwd                   string `json:"passwd"`
	Name                     string `json:"db"`
	OverrideCorporaTableName string `json:"overrideCorporaTableName"`
	OverridePCTableName      string `json:"overridePcTableName"`
}
