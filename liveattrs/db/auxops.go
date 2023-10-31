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
	"masm/v3/corpus"
	"masm/v3/liveattrs/db/qbuilder/adhoc"
	"masm/v3/liveattrs/request/query"
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

func GetSubcSize(laDB *sql.DB, corpusInfo *corpus.DBInfo, corpora []string, attrMap query.Attrs) (int, error) {
	sizeCalc := adhoc.SubcSize{
		CorpusInfo:          corpusInfo,
		AttrMap:             attrMap,
		AlignedCorpora:      corpora[1:],
		EmptyValPlaceholder: "", // TODO !!!!
	}
	sqlq, args := sizeCalc.Query()
	cur := laDB.QueryRow(sqlq, args...)
	var ans sql.NullInt64
	if err := cur.Scan(&ans); err != nil {
		return 0, err
	}
	if ans.Valid {
		return int(ans.Int64), nil
	}
	return 0, nil
}
