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
	"fmt"
	"masm/cnf"
	"masm/fsops"
	"masm/mango"
	"path/filepath"
	"strings"
)

// FileMappedValue is an abstraction of a configured file-related
// value where 'Value' represents the value to be inserted into
// some configuration and may or may not be actual file path.
type FileMappedValue struct {
	Value        string  `json:"value"`
	Path         string  `json:"-"`
	FileExists   bool    `json:"exists"`
	LastModified *string `json:"lastModified"`
	Size         int64   `json:"size"`
}

// WordSketchConf wraps different word-sketches related data/configuration files
type WordSketchConf struct {
	WSDef  FileMappedValue `json:"wsdef"`
	WSBase FileMappedValue `json:"wsbase"`
	WSThes FileMappedValue `json:"wsthes"`
}

// RegistryConf wraps registry configuration related info
type RegistryConf struct {
	Paths        []FileMappedValue `json:"paths"`
	Vertical     FileMappedValue   `json:"vertical"`
	WordSketches WordSketchConf    `json:"wordSketch"`
}

// TTDBRecord wraps information about text types data configuration
type TTDBRecord struct {
	Path FileMappedValue `json:"path"`
}

// Data wraps information about indexed corpus data/files
type Data struct {
	Size int64           `json:"size"`
	Path FileMappedValue `json:"path"`
}

// Info wraps information about a corpus installation
type Info struct {
	ID           string       `json:"id"`
	IndexedData  Data         `json:"indexedData"`
	TextTypesDB  TTDBRecord   `json:"textTypesDb"`
	RegistryConf RegistryConf `json:"registry"`
}

// NotFound is an error mapped to a similar Manatee error
type NotFound struct {
	error
}

// InfoError is a general corpus data information error
// Please note that we do not consider 'data not being present'
// an error.
type InfoError struct {
	error
}

// bindValueToPath creates a new FileMappedValue instance
// using 'value' argument. Then it tests whether the
// 'path' exists and if so then it sets related properties
// (FileExists, LastModified, Size) to proper values
func bindValueToPath(value, path string) FileMappedValue {
	ans := FileMappedValue{Value: value}
	if fsops.IsFile(path) {
		mTime := fsops.GetFileMtime(path)
		ans.FileExists = true
		ans.LastModified = &mTime
		ans.Size = fsops.FileSize(path)
	}
	return ans
}

func findVerticalFile(basePath, corpusID string) FileMappedValue {
	suffixes := []string{".tar.gz", ".tar.bz2", ".tgz", ".tbz2", ".7z", ".zip", ".tar", ".rar", ""}
	var verticalPath string
	if IsIntercorpFilename(corpusID) {
		verticalPath = filepath.Join(basePath, GenCorpusGroupName(corpusID), "vertikaly", corpusID)

	} else {
		verticalPath = filepath.Join(basePath, corpusID, "vertikala")
	}

	ans := FileMappedValue{Value: verticalPath}
	for _, suff := range suffixes {
		fullPath := verticalPath + suff
		if fsops.IsFile(fullPath) {
			mTime := fsops.GetFileMtime(fullPath)
			ans.LastModified = &mTime
			ans.Value = fullPath
			ans.Path = fullPath
			ans.FileExists = true
			ans.Size = fsops.FileSize(fullPath)
			return ans
		}
	}
	return ans
}

func attachWordSketchConfInfo(corpusID string, wsattr string, conf *cnf.CorporaSetup, result *Info) {
	tmp := GenWSDefFilename(conf.WordSketchDefDirPath, corpusID)
	result.RegistryConf.WordSketches = WordSketchConf{
		WSDef: bindValueToPath(tmp, tmp),
	}

	wsBaseFile, wsBaseVal := GenWSBaseFilename(conf.CorpusDataPath.Abstract, corpusID, wsattr)
	result.RegistryConf.WordSketches.WSBase = bindValueToPath(wsBaseVal, wsBaseFile)

	wsThesFile, wsThesVal := GenWSThesFilename(conf.CorpusDataPath.Abstract, corpusID, wsattr)
	result.RegistryConf.WordSketches.WSThes = bindValueToPath(wsThesVal, wsThesFile)
}

func attachTextTypeDbInfo(corpusID string, conf *cnf.CorporaSetup, result *Info) {
	dbFileName := GenCorpusGroupName(corpusID) + ".db"
	absPath := filepath.Join(conf.TextTypesDbDirPath, dbFileName)
	result.TextTypesDB = TTDBRecord{}
	result.TextTypesDB.Path = bindValueToPath(absPath, absPath)
}

// GetCorpusInfo provides miscellaneous corpus installation information mostly
// related to different data files.
// It should return an error only in case Manatee or filesystem produces some
// error (i.e. not in case something is just not found).
func GetCorpusInfo(corpusID string, wsattr string, setup *cnf.CorporaSetup) (*Info, error) {
	ans := &Info{ID: corpusID}
	ans.IndexedData = Data{}
	ans.RegistryConf = RegistryConf{Paths: make([]FileMappedValue, 0, 10)}
	ans.RegistryConf.Vertical = findVerticalFile(setup.VerticalFilesDirPath, corpusID)

	for _, regPathRoot := range setup.RegistryDirPaths {
		regPath := filepath.Join(regPathRoot, corpusID)
		if fsops.IsFile(regPath) {
			ans.RegistryConf.Paths = append(ans.RegistryConf.Paths, bindValueToPath(regPath, regPath))
			corp, err := mango.OpenCorpus(regPath)
			if err != nil {
				if strings.Contains(err.Error(), "CorpInfoNotFound") {
					return nil, NotFound{fmt.Errorf("Manatee cannot open/find corpus %s", corpusID)}

				}
				return nil, InfoError{err}
			}

			defer mango.CloseCorpus(corp)
			ans.IndexedData.Size, err = mango.GetCorpusSize(corp)
			if err != nil {
				return nil, InfoError{err}
			}

			corpDataPath, err := mango.GetCorpusConf(corp, "PATH")
			if err != nil {
				return nil, InfoError{err}
			}

			items, err := fsops.ListFilesInDir(corpDataPath, true)
			if err != nil {
				return nil, InfoError{err}
			}

			mTime := items.First().ModTime().Format("2006-01-02T15:04:05-0700")
			ans.IndexedData.Path = FileMappedValue{
				Value:        filepath.Clean(corpDataPath),
				LastModified: &mTime,
				FileExists:   true,
				Size:         fsops.FileSize(filepath.Clean(corpDataPath)),
			}

		} else {
			ans.IndexedData.Size = 0
			ans.IndexedData.Path = FileMappedValue{
				Value:        filepath.Clean(filepath.Join(setup.CorpusDataPath.Abstract, corpusID)),
				LastModified: nil,
				FileExists:   false,
				Path:         filepath.Clean(filepath.Join(setup.CorpusDataPath.Abstract, corpusID)),
			}
		}
	}

	attachWordSketchConfInfo(corpusID, wsattr, setup, ans)
	attachTextTypeDbInfo(corpusID, setup, ans)

	return ans, nil
}
