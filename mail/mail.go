// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
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

package mail

import (
	"fmt"
	"strings"

	cncmail "github.com/czcorpus/cnc-gokit/mail"
)

var (
	NotFoundMsgPlaceholder = "??"
)

type EmailNotification struct {
	cncmail.NotificationConf
}

// LocalizedSignature returns a mail signature based on configuration
// and provided language. It is able to search for 2-character codes
// in case 5-ones are not matching.
// In case nothing is found, the returned message is NotFoundMsgPlaceholder
// (and an error is returned).
func (enConf EmailNotification) LocalizedSignature(lang string) (string, error) {
	if msg, ok := enConf.Signature[lang]; ok {
		return msg, nil
	}
	lang2 := strings.Split(lang, "-")[0]
	for k, msg := range enConf.Signature {
		if strings.Split(k, "-")[0] == lang2 {
			return msg, nil
		}
	}
	return NotFoundMsgPlaceholder, fmt.Errorf("e-mail signature for language %s not found", lang)
}

func (enConf EmailNotification) HasSignature() bool {
	return len(enConf.Signature) > 0
}

func (enConf EmailNotification) DefaultSignature(lang string) string {
	if lang == "cs" || lang == "cs-CZ" {
		return "Váš CNC-MASM"
	}
	return "Your CNC-MASM"
}
