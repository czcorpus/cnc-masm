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
	"masm/v3/corpus"
	"masm/v3/liveattrs/request/query"
	"masm/v3/liveattrs/utils"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type RequestData struct {
	CorpusID string
	Payload  query.Payload
	Created  time.Time
	IsCached bool
	ProcTime time.Duration
}

func (rd RequestData) toLogJSON() []byte {
	ans, err := json.Marshal(struct {
		Created        string   `json:"time"`
		Corpus         string   `json:"corpus"`
		AlignedCorpora []string `json:"alignedCorpora,omitempty"`
		IsAutocomplete bool     `json:"isAutocomplete"`
		IsCached       bool     `json:"isCached"`
		ProcTimeSecs   float64  `json:"procTimeSecs"`
	}{
		Created:        rd.Created.Format(time.RFC3339),
		Corpus:         rd.CorpusID,
		AlignedCorpora: rd.Payload.Aligned,
		IsAutocomplete: rd.Payload.AutocompleteAttr != "",
		IsCached:       rd.IsCached,
		ProcTimeSecs:   rd.ProcTime.Seconds(),
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal query log")
	}
	return ans
}

func (rd RequestData) toZeroLog(evt *zerolog.Event) {
	evt.
		Bool("isQuery", true).
		Str("corpus", rd.CorpusID).
		Strs("alignedCorpora", rd.Payload.Aligned).
		Bool("isAutocomplete", rd.Payload.AutocompleteAttr != "").
		Bool("isCached", rd.IsCached).
		Float64("procTimeSecs", rd.ProcTime.Seconds()).
		Msg("")
}

type StructAttrUsage struct {
	db      *sql.DB
	channel <-chan RequestData
}

func (sau *StructAttrUsage) RunHandler() {
	for data := range sau.channel {
		data.toZeroLog(log.Info())
		if !data.IsCached {
			err := sau.save(data)
			if err != nil {
				log.Error().Err(err).Msg("Unable to save struct. attrs usage data")
			}
		}
	}
}

func (sau *StructAttrUsage) save(data RequestData) error {
	sql_template := "INSERT INTO `usage` (`corpus_id`, `structattr_name`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `num_used`=`num_used`+1"
	context, err := sau.db.Begin()
	if err != nil {
		return err
	}
	for attr := range data.Payload.Attrs {
		_, err := context.Query(sql_template, data.CorpusID, utils.ImportKey(attr))
		if err != nil {
			return err
		}
	}
	context.Commit()
	return nil
}

func NewStructAttrUsage(laDB *sql.DB, saveData <-chan RequestData) *StructAttrUsage {
	return &StructAttrUsage{
		db:      laDB,
		channel: saveData,
	}
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

func UpdateIndexes(laDB *sql.DB, corpusInfo *corpus.DBInfo, maxColumns int) updIdxResult {
	// get most used columns
	rows, err := laDB.Query(
		"SELECT structattr_name "+
			"FROM `usage` "+
			"WHERE corpus_id = ? AND num_used > 0 ORDER BY num_used DESC LIMIT ?",
		corpusInfo.Name, maxColumns,
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
	var sqlTemplate string
	if corpusInfo.GroupedName() == corpusInfo.Name {
		sqlTemplate = "CREATE INDEX IF NOT EXISTS `%s` ON `%s_liveattrs_entry` (`%s`)"
	} else {
		sqlTemplate = "CREATE INDEX IF NOT EXISTS `%s` ON `%s_liveattrs_entry` (`%s`, `corpus_id`)"
	}
	usedIndexes := make([]any, len(columns))
	context, err := laDB.Begin()
	if err != nil {
		return updIdxResult{Error: err}
	}
	for i, column := range columns {
		usedIndexes[i] = fmt.Sprintf("%s_autoindex", column)
		_, err := context.Query(fmt.Sprintf(sqlTemplate, usedIndexes[i], corpusInfo.GroupedName(), column))
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
	values := append([]any{fmt.Sprintf("`%s_liveattrs_entry`", corpusInfo.GroupedName())}, usedIndexes...)
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
	sqlTemplate = "DROP INDEX %s ON `%s_liveattrs_entry`"
	context, err = laDB.Begin()
	if err != nil {
		return updIdxResult{Error: err}
	}
	for _, index := range unusedIndexes {
		_, err := context.Query(fmt.Sprintf(sqlTemplate, index, corpusInfo.GroupedName()))
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
