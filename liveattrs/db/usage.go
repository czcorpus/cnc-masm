// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
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
//
// This struct tracks column usage in liveattrs search.
// We use it to optimize db indexes.
//
// CREATE TABLE usage (
//	corpus_id varchar(127) NOT NULL,
//	structattr_name varchar(127) NOT NULL,
//	num_used int NOT NULL DEFAULT 1,
//	UNIQUE (corpus_id, structattr_name)
// )

package db

import (
	"database/sql"
	"log"
	"masm/v3/liveattrs/request/query"
	"os"
)

type RequestData struct {
	corpusId string
	payload  query.Payload
}

type StructAttrUsage struct {
	db      *sql.DB
	channel chan RequestData
}

func (sau *StructAttrUsage) SendData(corpusId string, payload query.Payload) {
	sau.channel <- RequestData{corpusId, payload}
}

func (sau *StructAttrUsage) RunHandler(exitEvent <-chan os.Signal) {
	for {
		select {
		case data, ok := <-sau.channel:
			if !ok {
				break
			}
			err := sau.save(data)
			if err != nil {
				log.Printf("Unable to save structattr usage data: %s", err)
			}
		case <-exitEvent:
			close(sau.channel)
		}
	}
}

func (sau *StructAttrUsage) save(data RequestData) error {
	sql_template := "INSERT INTO `usage` (`corpus_id`, `structattr_name`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `num_used`=`num_used`+1"
	context, err := sau.db.Begin()
	if err != nil {
		return err
	}
	for attr := range data.payload.Attrs {
		_, err := context.Query(sql_template, data.corpusId, ImportKey(attr))
		if err != nil {
			return err
		}
	}
	context.Commit()
	return nil
}

func NewStructAttrUsage(laDB *sql.DB) *StructAttrUsage {
	attrStats := StructAttrUsage{db: laDB, channel: make(chan RequestData)}
	return &attrStats
}

func LoadUsage(laDB *sql.DB, corpusId string) (map[string]int, error) {
	rows, err := laDB.Query("SELECT `structattr_name`, `num_used` FROM `usage` WHERE `corpus_id` = ?", corpusId)
	ans := make(map[string]int)
	if err == sql.ErrNoRows {
		return ans, nil

	} else if err != nil {
		return nil, err
	}

	for rows.Next() {
		var structattrName string
		var numUsed int
		if err := rows.Scan(&structattrName, &numUsed); err != nil {
			return nil, err
		}
		ans[structattrName] = numUsed
	}
	return ans, nil
}
