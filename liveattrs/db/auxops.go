// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
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

package db

import (
	"database/sql"
	"fmt"
	"masm/v3/liveattrs/utils"
	"strings"
)

func DeleteTable(tx *sql.Tx, groupedName string, corpusName string) error {
	_, err := tx.Exec(
		fmt.Sprintf("DELETE FROM %s_liveattrs_entry WHERE corpus_id = ?", groupedName),
		corpusName,
	)
	if err != nil {
		return err
	}
	if groupedName == corpusName {
		_, err = tx.Exec(
			fmt.Sprintf("DROP TABLE %s_liveattrs_entry", groupedName),
		)
	}
	return err
}

func GetSubcSize(db *sql.DB, groupedName string, corpora []string, attrMap map[string][]string) (int, error) {
	joinSQL := make([]string, 0, 10)
	whereSQL := []string{
		"t1.corpus_id = ?",
		"t1.poscount is NOT NULL",
	}
	whereValues := []any{corpora[0]}
	for i, item := range corpora[1:] {
		iOffs := i + 2
		joinSQL = append(
			joinSQL,
			fmt.Sprintf(
				"JOIN `%s_liveattrs_entry` AS t%d ON t1.item_id = t%d.item_id",
				groupedName, iOffs, iOffs,
			),
		)
		whereSQL = append(
			whereSQL,
			fmt.Sprintf("t%d.corpus_id = ?", iOffs),
		)
		whereValues = append(whereValues, item)
	}
	for k, vlist := range attrMap {
		tmp := make([]string, 0, len(vlist)*5)
		for _, v := range vlist {
			whereValues = append(whereValues, v)
			tmp = append(
				tmp,
				fmt.Sprintf("t1.%s = ?", utils.ImportKey(k)),
			)
		}
		whereSQL = append(
			whereSQL,
			fmt.Sprintf("(%s)", strings.Join(tmp, " OR ")),
		)
	}
	cur := db.QueryRow(
		fmt.Sprintf(
			"SELECT SUM(t1.poscount) FROM `%s_liveattrs_entry` AS t1 %s WHERE %s",
			groupedName,
			strings.Join(joinSQL, " "),
			strings.Join(whereSQL, " AND "),
		),
		whereValues...,
	)
	var ans int
	err := cur.Scan(&ans)
	return ans, err
}
