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
	"encoding/json"
	"fmt"
	"log"
	"masm/v3/liveattrs/request/query"
	"os"
	"strings"
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

// --

type updIdxResult struct {
	UsedIndexes    []string
	RemovedIndexes []string
	Error          error
}

func (res *updIdxResult) MarshalJSON() ([]byte, error) {
	var errStr string
	if res.Error != nil {
		errStr = res.Error.Error()
	}
	return json.Marshal(struct {
		UsedIndexes    []string `json:"usedIndexes"`
		RemovedIndexes []string `json:"removedIndexes"`
		Error          string   `json:"error,omitempty"`
	}{
		UsedIndexes:    res.UsedIndexes,
		RemovedIndexes: res.RemovedIndexes,
		Error:          errStr,
	})
}

// --

func UpdateIndexes(laDB *sql.DB, corpusId string, maxColumns int) updIdxResult {
	// get most used columns
	rows, err := laDB.Query(
		"SELECT structattr_name "+
			"FROM `usage` "+
			"WHERE corpus_id = ? AND num_used > 0 ORDER BY num_used DESC LIMIT ?",
		corpusId, maxColumns,
	)
	if err != nil && err != sql.ErrNoRows {
		return updIdxResult{Error: err}
	}
	columns := make([]string, 0, maxColumns)
	for rows.Next() {
		var structattrName string
		if err := rows.Scan(&structattrName); err != nil {
			return updIdxResult{Error: err}
		}
		columns = append(columns, structattrName)
	}

	// create indexes if necessary with `_autoindex` appendix
	sqlTemplate := "CREATE INDEX IF NOT EXISTS `%s` ON `%s_item` (`%s`)"
	usedIndexes := make([]any, len(columns))
	context, err := laDB.Begin()
	if err != nil {
		return updIdxResult{Error: err}
	}
	for i, column := range columns {
		usedIndexes[i] = fmt.Sprintf("%s_autoindex", column)
		_, err := context.Query(fmt.Sprintf(sqlTemplate, usedIndexes[i], corpusId, column))
		if err != nil {
			return updIdxResult{Error: err}
		}
	}
	context.Commit()

	// get remaining unused indexes with `_autoindex` appendix
	valuesPlaceholders := make([]string, len(usedIndexes))
	for i := 0; i < len(valuesPlaceholders); i++ {
		valuesPlaceholders[i] = "?"
	}
	sqlTemplate = fmt.Sprintf(
		"SELECT INDEX_NAME FROM information_schema.statistics where TABLE_NAME = ? AND INDEX_NAME LIKE '%%_autoindex' AND INDEX_NAME NOT IN (%s)",
		strings.Join(valuesPlaceholders, ", "),
	)
	values := append([]any{fmt.Sprintf("%s_item", corpusId)}, usedIndexes...)
	rows, err = laDB.Query(sqlTemplate, values...)
	if err != nil && err != sql.ErrNoRows {
		return updIdxResult{Error: err}
	}
	unusedIndexes := make([]string, 0, 10)
	for rows.Next() {
		var indexName string
		if err := rows.Scan(&indexName); err != nil {
			return updIdxResult{Error: err}
		}
		unusedIndexes = append(unusedIndexes, indexName)
	}

	// drop unused indexes
	sqlTemplate = "DROP INDEX %s ON `%s_item`"
	context, err = laDB.Begin()
	if err != nil {
		return updIdxResult{Error: err}
	}
	for _, index := range unusedIndexes {
		_, err := context.Query(fmt.Sprintf(sqlTemplate, index, corpusId))
		if err != nil {
			return updIdxResult{Error: err}
		}
	}
	context.Commit()

	ans := updIdxResult{
		UsedIndexes:    make([]string, len(usedIndexes)),
		RemovedIndexes: unusedIndexes,
	}
	for i, v := range usedIndexes {
		ans.UsedIndexes[i] = v.(string)
	}
	return ans
}
