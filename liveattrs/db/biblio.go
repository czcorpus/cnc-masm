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

package db

import (
	"database/sql"
	"fmt"
	"masm/v3/cnf"
	"masm/v3/corpus"
	"masm/v3/liveattrs/request/biblio"
	"strings"

	vteconf "github.com/czcorpus/vert-tagextract/v2/cnf"
)

func GetBibliography(
	db *sql.DB,
	corpusInfo *corpus.DBInfo,
	laConf *vteconf.VTEConf,
	qry biblio.Payload,
) (map[string]string, error) {
	subcorpAttrs := cnf.GetSubcorpAttrs(laConf)
	selAttrs := make([]string, len(subcorpAttrs))
	for i, attr := range subcorpAttrs {
		selAttrs[i] = ImportKey(attr)
	}

	sql1 := fmt.Sprintf(
		"SELECT %s FROM %s_item WHERE %s = ? LIMIT 1",
		strings.Join(selAttrs, ", "),
		corpusInfo.Name,
		ImportKey(corpusInfo.BibIDAttr),
	)

	rows, err := db.Query(sql1, qry.ItemID)
	ans := make(map[string]string)
	if err == sql.ErrNoRows {
		return ans, ErrorEmptyResult

	} else if err != nil {
		return map[string]string{}, err
	}

	ansVals := make([]sql.NullString, len(selAttrs))
	ansPvals := make([]any, len(selAttrs))
	for i := range ansVals {
		ansPvals[i] = &ansVals[i]
	}
	rows.Next()
	err = rows.Scan(ansPvals...)
	if err != nil {
		return nil, err
	}
	for i, val := range ansVals {
		if val.Valid {
			ans[ExportKey(selAttrs[i])] = val.String
		}
	}
	return ans, nil
}

func FindBibTitles(
	db *sql.DB,
	corpusInfo *corpus.DBInfo,
	laConf *vteconf.VTEConf,
	qry biblio.PayloadList,
) (map[string]string, error) {
	subcorpAttrs := cnf.GetSubcorpAttrs(laConf)
	selAttrs := make([]string, len(subcorpAttrs))
	for i, attr := range subcorpAttrs {
		selAttrs[i] = ImportKey(attr)
	}

	valuesPlaceholders := make([]string, len(qry.ItemIDs))
	for i := range qry.ItemIDs {
		valuesPlaceholders[i] = "?"
	}
	sql1 := fmt.Sprintf(
		"SELECT %s, %s FROM %s_item WHERE %s IN (%s)",
		ImportKey(corpusInfo.BibIDAttr),
		ImportKey(corpusInfo.BibLabelAttr),
		corpusInfo.Name,
		ImportKey(corpusInfo.BibIDAttr),
		strings.Join(valuesPlaceholders, ", "),
	)
	sqlVals := make([]any, len(qry.ItemIDs))
	for i, v := range qry.ItemIDs {
		sqlVals[i] = v
	}

	rows, err := db.Query(sql1, sqlVals...)
	ans := make(map[string]string)
	if err == sql.ErrNoRows {
		return ans, ErrorEmptyResult

	} else if err != nil {
		return map[string]string{}, err
	}

	for rows.Next() {
		var bibIdVal, bibLabelVal sql.NullString
		err = rows.Scan(&bibIdVal, &bibLabelVal)
		if err != nil {
			return nil, err
		}
		if bibLabelVal.Valid {
			ans[bibIdVal.String] = bibLabelVal.String
		}
	}

	return ans, nil
}
