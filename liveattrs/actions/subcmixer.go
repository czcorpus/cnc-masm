// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Institute of the Czech National Corpus,
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

package actions

import (
	"encoding/json"
	"fmt"
	"masm/v3/api"
	"masm/v3/general/collections"
	"masm/v3/liveattrs/subcmixer"
	"net/http"
	"strings"
)

const (
	corpusMaxSize = 500000000
)

type subcmixerRatio struct {
	AttrName  string  `json:"attrName"`
	AttrValue string  `json:"attrValue"`
	Ratio     float64 `json:"ratio"`
}

type subcmixerArgs struct {
	Corpora   []string         `json:"corpora"`
	TextTypes []subcmixerRatio `json:"textTypes"`
}

func (sa *subcmixerArgs) validate() error {
	currStruct := ""
	for _, tt := range sa.TextTypes {
		strc := strings.Split(tt.AttrName, ".")
		if currStruct != "" && currStruct != strc[0] {
			return fmt.Errorf("the ratio rules for subcmixer may contain only attributes of a single structure")
		}
		currStruct = strc[0]
	}
	return nil
}

func importTaskArgs(args subcmixerArgs) ([]subcmixer.TaskArgs, error) {
	ans := [][]subcmixer.TaskArgs{
		{
			{
				NodeID:     0,
				ParentID:   subcmixer.NewEmptyMaybe[int](),
				Ratio:      1,
				Expression: &subcmixer.CategoryExpression{},
			},
		},
	}
	groupedRatios := collections.NewMultidict[subcmixerRatio]()
	for _, item := range args.TextTypes {
		groupedRatios.Add(item.AttrName, item)
	}
	counter := 1
	err := groupedRatios.ForEach(func(k string, expressions []subcmixerRatio) error {
		tmp := []subcmixer.TaskArgs{}
		for _, pg := range ans[len(ans)-1] {
			for _, item := range expressions {
				sm, err := subcmixer.NewCategoryExpression(item.AttrName, "==", item.AttrValue)
				if err != nil {
					return err
				}
				tmp = append(
					tmp,
					subcmixer.TaskArgs{
						NodeID:     counter,
						ParentID:   subcmixer.NewMaybe(pg.NodeID),
						Ratio:      item.Ratio / 100.0,
						Expression: sm,
					},
				)
				counter++
			}
		}
		ans = append(ans, tmp)
		return nil
	})
	if err != nil && err != collections.ErrorStopIteration {
		return []subcmixer.TaskArgs{}, err
	}
	ret := []subcmixer.TaskArgs{}
	for _, item := range ans {
		for _, subitem := range item {
			ret = append(ret, subitem)
		}
	}
	return ret, nil
}

func (a *Actions) MixSubcorpus(w http.ResponseWriter, req *http.Request) {
	var args subcmixerArgs
	err := json.NewDecoder(req.Body).Decode(&args)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusBadRequest)
		return
	}
	err = args.validate()
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusUnprocessableEntity)
	}
	conditions, err := importTaskArgs(args)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
	}
	laTableName := fmt.Sprintf("%s_liveattrs_entry", args.Corpora[0])
	catTree, err := subcmixer.NewCategoryTree(
		conditions,
		a.laDB,
		args.Corpora[0],
		args.Corpora[1:],
		laTableName,
		corpusMaxSize,
	)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	corpusDBInfo, err := a.cncDB.LoadInfo(args.Corpora[0])
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	mm, err := subcmixer.NewMetadataModel(
		a.laDB,
		laTableName,
		catTree,
		corpusDBInfo.BibIDAttr,
	)
	if err != nil {
		api.WriteJSONErrorResponse(w, api.NewActionErrorFrom(err), http.StatusInternalServerError)
		return
	}
	ans := mm.Solve()
	api.WriteJSONResponse(w, ans)
}
