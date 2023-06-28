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

package kontext

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

func SendSoftReset(conf *Conf) error {
	if len(conf.SoftResetURL) == 0 {
		log.Warn().Msgf("The kontextSoftResetURL configuration not set - ignoring the action")
		return nil
	}
	for _, instance := range conf.SoftResetURL {
		resp, err := http.Post(instance, "application/json", nil)
		if err != nil {
			return err
		}
		if resp.StatusCode >= 300 {
			return fmt.Errorf("kontext instance `%s` soft reset failed - unexpected status code %d", instance, resp.StatusCode)
		}
	}
	return nil
}
