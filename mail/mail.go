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
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

var (
	NotFoundMsgPlaceholder = "??"
)

type EmailNotification struct {
	Sender       string `json:"sender"`
	SMTPServer   string `json:"smtpServer"`
	SMTPUsername string `json:"smtpUsername"`
	SMTPPassword string `json:"smtpPassword"`
	// Signature defines multi-language signature for notification e-mails
	Signature map[string]string `json:"signature"`
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

func dialSmtpServer(conf *EmailNotification) (*smtp.Client, error) {
	host, port, err := net.SplitHostPort(conf.SMTPServer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SMTP server info: %w", err)
	}
	if port == "25" {
		ans, err := smtp.Dial(conf.SMTPServer)
		if err != nil {
			return nil, fmt.Errorf("failed to dial: %w", err)
		}
		return ans, err
	}
	auth := smtp.PlainAuth("", conf.SMTPUsername, conf.SMTPPassword, host)
	client, err := smtp.Dial(conf.SMTPServer)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	client.StartTLS(&tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to StartTLS: %w", err)
	}
	err = client.Auth(auth)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate client: %w", err)
	}
	return client, nil
}

// SendNotification sends a general e-mail notification based on
// a respective monitoring configuration. The 'alarmToken' argument
// can be nil - in such case the 'turn of the alarm' text won't be
// part of the message.
func SendNotification(
	conf *EmailNotification,
	receivers []string,
	subject string,
	msgParagraphs ...string,
) error {
	client, err := dialSmtpServer(conf)
	if err != nil {
		return err
	}
	defer client.Close()

	client.Mail(conf.Sender)
	for _, rcpt := range receivers {
		client.Rcpt(rcpt)
	}

	wc, err := client.Data()
	if err != nil {
		return err
	}
	defer wc.Close()

	headers := make(map[string]string)
	headers["From"] = conf.Sender
	headers["To"] = strings.Join(receivers, ",")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	body := ""
	for k, v := range headers {
		body += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	for _, par := range msgParagraphs {
		body += "<p>" + par + "</p>\r\n\r\n"
	}

	buf := bytes.NewBufferString(body)
	_, err = buf.WriteTo(wc)
	return err
}
