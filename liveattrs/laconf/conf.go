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

package laconf

import (
	"fmt"

	vteconf "github.com/czcorpus/vert-tagextract/v3/cnf"
)

func GetSubcorpAttrs(vteConf *vteconf.VTEConf) []string {
	ans := make([]string, 0, 50)
	for strct, attrs := range vteConf.Structures {
		for _, attr := range attrs {
			ans = append(ans, fmt.Sprintf("%s.%s", strct, attr))
		}
	}
	return ans
}

func LoadConf(path string) (*vteconf.VTEConf, error) {
	return vteconf.LoadConf(path)
}
