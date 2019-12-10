// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
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

package mail

import (
	"bytes"
	"fmt"
	"masm/cnf"
	"net/smtp"
	"strings"

	"github.com/google/uuid"
)

// SendNotification sends a general e-mail notification based on
// a respective monitoring configuration. The 'alarmToken' argument
// can be nil - in such case the 'turn of the alarm' text won't be
// part of the message.
func SendNotification(conf *cnf.KontextMonitoringSetup, subject, message string, alarmToken *uuid.UUID) error {
	client, err := smtp.Dial(conf.SMTPServer)
	if err != nil {
		return err
	}
	defer client.Close()

	client.Mail(conf.Sender)
	for _, rcpt := range conf.NotificationEmails {
		client.Rcpt(rcpt)
	}

	wc, err := client.Data()
	if err != nil {
		return err
	}
	defer wc.Close()

	headers := make(map[string]string)
	headers["From"] = conf.Sender
	headers["To"] = strings.Join(conf.NotificationEmails, ",")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	body := ""
	for k, v := range headers {
		body += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	body += "<p>" + message + "</p>\r\n\r\n"

	if alarmToken != nil {
		body += fmt.Sprintf("<p>Click <a href=\"%s/%s\">here</a> to stop the alarm</p>\r\n",
			conf.AlarmResetURL, alarmToken)
	}
	buf := bytes.NewBufferString(body)
	_, err = buf.WriteTo(wc)
	return err
}
