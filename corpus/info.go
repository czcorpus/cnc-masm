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

package corpus

type DBInfo struct {
	Name   string
	Active int
	Locale string

	ParallelCorpus string

	// BibLabelAttr contains both structure and attribute (e.g. 'doc.id')
	BibLabelAttr string

	// BibIDAttr contains both structure and attribute (e.g. 'doc.id')
	BibIDAttr          string
	BibGroupDuplicates int
}

// LiveattrsTableName returns actual table name used for live-attrs querying.
// For non-aligned corpora, this is just [corpus name]_item. For aligned ones,
// we use data from parallel_corpus table and create [parallel corpus name]_item
func (info *DBInfo) LiveattrsTableName() string {
	if info.ParallelCorpus != "" {
		return info.ParallelCorpus + "_item"
	}
	return info.Name + "_item"
}
