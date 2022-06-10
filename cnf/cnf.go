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

package cnf

import (
	"encoding/json"
	"io/ioutil"
	"log"

	vtedb "github.com/czcorpus/vert-tagextract/v2/db"
)

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
	StatusDataPath        string         `json:"statusDataPath"`
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

// VersionInfo provides a detailed information about the actual build
type VersionInfo struct {
	Version   string `json:"version"`
	BuildDate string `json:"buildDate"`
	GitCommit string `json:"gitCommit"`
}
