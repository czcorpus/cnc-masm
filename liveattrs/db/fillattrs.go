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
	"masm/v3/liveattrs/request/fillattrs"
	"strings"
)

// For a structattr and its values find values of structattrs specified in fill list
// Returns a dict of dicts {search_attr_value: {attr: value}}.
// In case nothing is found, ErrorEmptyResult is returned
func FillAttrs(
	db *sql.DB,
	corpusID string,
	qry fillattrs.Payload,
) (map[string]map[string]string, error) {

	selAttrs := make([]string, len(qry.Fill)+1)
	selAttrs[0] = ImportKey(qry.Search)
	for i, f := range qry.Fill {
		selAttrs[i+1] = ImportKey(f)
	}
	valuesPlaceholders := make([]string, len(qry.Values))
	for i := range qry.Values {
		valuesPlaceholders[i] = "?"
	}
	sql1 := fmt.Sprintf(
		"SELECT %s FROM %s_item WHERE %s IN (%s)",
		strings.Join(selAttrs, ", "),
		corpusID,
		ImportKey(qry.Search),
		strings.Join(valuesPlaceholders, ", "),
	)
	sqlVals := make([]any, len(qry.Values))
	for i, v := range qry.Values {
		sqlVals[i] = v
	}

	rows, err := db.Query(sql1, sqlVals...)
	ans := make(map[string]map[string]string)
	if err == sql.ErrNoRows {
		return ans, ErrorEmptyResult

	} else if err != nil {
		return map[string]map[string]string{}, err
	}
	isEmpty := true
	for rows.Next() {
		isEmpty = isEmpty && false
		ansVals := make([]sql.NullString, len(selAttrs))
		ansPvals := make([]any, len(selAttrs))
		for i := range ansVals {
			ansPvals[i] = &ansVals[i]
		}
		if err := rows.Scan(ansPvals...); err != nil {
			return map[string]map[string]string{}, err
		}
		srchVal := ansVals[0].String
		ans[srchVal] = make(map[string]string)
		for i := 1; i < len(ansVals); i++ {
			if ansVals[i].Valid {
				ans[srchVal][selAttrs[i]] = ansVals[i].String
			}
		}
	}
	if isEmpty {
		return ans, ErrorEmptyResult
	}
	return ans, nil
}
