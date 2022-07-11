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
	"encoding/json"
	"io/ioutil"
	"log"
	"masm/v3/jobs"

	vtedb "github.com/czcorpus/vert-tagextract/v2/db"
)

// CorporaDataPaths describes three
// different ways how paths to corpora
// data are specified:
// 1) CNC - a global storage path (typically slow but reliable)
// 2) Kontext - a special fast storage for KonText
// 3) abstract - a path for data consumers; points to either
// (1) or (2)
type CorporaDataPaths struct {
	Abstract string `json:"abstract"`
	CNC      string `json:"cnc"`
	Kontext  string `json:"kontext"`
}

// CorporaSetup defines masm application configuration related
// to a corpus
type CorporaSetup struct {
	RegistryDirPaths     []string          `json:"registryDirPaths"`
	TextTypesDbDirPath   string            `json:"textTypesDbDirPath"`
	CorpusDataPath       CorporaDataPaths  `json:"corpusDataPath"`
	AltAccessMapping     map[string]string `json:"altAccessMapping"` // registry => data mapping
	WordSketchDefDirPath string            `json:"wordSketchDefDirPath"`
	SyncAllowedCorpora   []string          `json:"syncAllowedCorpora"`
	VerticalFilesDirPath string            `json:"verticalFilesDirPath"`
	ManateeDynlibPath    string            `json:"manateeDynlibPath"`
}

func (cs *CorporaSetup) AllowsSyncForCorpus(name string) bool {
	for _, v := range cs.SyncAllowedCorpora {
		if v == name {
			return true
		}
	}
	return false
}

func (cs *CorporaSetup) SubdirIsInAltAccessMapping(subdir string) bool {
	_, ok := cs.AltAccessMapping[subdir]
	return ok
}

type databaseSetup struct {
	Host   string `json:"host"`
	User   string `json:"user"`
	Passwd string `json:"passwd"`
	DBName string `json:"db"`
}

type LiveAttrsConf struct {
	DB          *vtedb.Conf `json:"db"`
	ConfDirPath string      `json:"confDirPath"`
}

// Conf is a global configuration of the app
type Conf struct {
	ListenAddress         string         `json:"listenAddress"`
	ListenPort            int            `json:"listenPort"`
	ServerReadTimeoutSecs int            `json:"serverReadTimeoutSecs"`
	CorporaSetup          *CorporaSetup  `json:"corporaSetup"`
	LogFile               string         `json:"logFile"`
	CNCDB                 *databaseSetup `json:"cncDb"`
	LiveAttrs             *LiveAttrsConf `json:"liveAttrs"`
	Jobs                  *jobs.Conf     `json:"jobs"`
	KontextSoftResetURL   string         `json:"kontextSoftResetURL"`
}

func LoadConfig(path string) *Conf {
	if path == "" {
		log.Fatal("FATAL: Cannot load config - path not specified")
	}
	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal("FATAL: Cannot load config - ", err)
	}
	var conf Conf
	err = json.Unmarshal(rawData, &conf)
	if err != nil {
		log.Fatal("FATAL: Cannot load config - ", err)
	}
	return &conf
}
