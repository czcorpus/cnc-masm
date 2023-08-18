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

package laquery

import (
	"fmt"
	"masm/v3/liveattrs/db/qbuilder"
	"masm/v3/liveattrs/request/query"
	"masm/v3/liveattrs/utils"
	"strings"

	"github.com/rs/zerolog/log"
)

type PredicateArgs struct {
	data                query.Attrs
	bibID               string
	bibLabel            string
	autocompleteAttr    string
	emptyValPlaceholder string
}

func (args *PredicateArgs) Len() int {
	return len(args.data)
}

func (args *PredicateArgs) importValue(value string) string {
	if value == args.emptyValPlaceholder {
		return ""
	}
	return value
}

func (args *PredicateArgs) ExportSQL(itemPrefix, corpusID string) (string, []string) {
	where := make([]string, 0, 20)
	sqlValues := make([]string, 0, 20)
	for dkey, values := range args.data {
		key := utils.ImportKey(dkey)
		if args.autocompleteAttr == args.bibLabel && key == args.bibID {
			continue
		}
		cnfItem := make([]string, 0, 20)
		switch tValues := values.(type) {
		case []any:
			for _, value := range tValues {
				tValue, ok := value.(string)
				if !ok {
					continue
				}
				if len(tValue) == 0 || tValue[0] != '@' {
					cnfItem = append(
						cnfItem,
						fmt.Sprintf(
							"%s.%s %s ?",
							itemPrefix, key, qbuilder.CmpOperator(tValue),
						),
					)
					sqlValues = append(sqlValues, args.importValue(tValue))

				} else {
					cnfItem = append(
						cnfItem,
						fmt.Sprintf(
							"%s.%s %s ?",
							itemPrefix, args.bibLabel,
							qbuilder.CmpOperator(tValue[1:]),
						),
					)
					sqlValues = append(sqlValues, args.importValue(tValue[1:]))
				}
			}
		case string:
			cnfItem = append(
				cnfItem,
				fmt.Sprintf(
					"%s.%s LIKE ?",
					itemPrefix, key),
			)
			sqlValues = append(sqlValues, args.importValue(tValues))
		case map[string]any:
			regexpVal, ok := args.data.GetRegexpAttrVal(dkey)
			if ok {
				cnfItem = append(cnfItem, fmt.Sprintf("%s.%s REGEXP ?", itemPrefix, key))
				sqlValues = append(sqlValues, args.importValue(regexpVal))

				// TODO add support for this
			} else {
				// TODO handle in a better way
				log.Error().Msgf(
					"failed to determine type of liveattrs attribute %s (corpus %s)", key, corpusID)
			}
		default: // TODO can this even happen???
			cnfItem = append(
				cnfItem,
				fmt.Sprintf(
					"LOWER(%s.%s) %s LOWER(?)",
					itemPrefix, key, qbuilder.CmpOperator(fmt.Sprintf("%v", tValues)),
				),
			)
			sqlValues = append(sqlValues, args.importValue(fmt.Sprintf("%v", tValues)))
		}

		if len(cnfItem) > 0 {
			where = append(where, fmt.Sprintf("(%s)", strings.Join(cnfItem, " OR ")))
		}
	}
	where = append(where, fmt.Sprintf("%s.corpus_id = ?", itemPrefix))
	sqlValues = append(sqlValues, corpusID)
	return strings.Join(where, " AND "), sqlValues
}

type QueryComponents struct {
	sqlTemplate   string
	selectedAttrs []string
	hiddenAttrs   []string
	whereValues   []string
}
