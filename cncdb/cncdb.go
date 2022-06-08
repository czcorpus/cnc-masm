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
package cncdb

import (
	"database/sql"
	"masm/v3/corpus"
	"time"

	"github.com/go-sql-driver/mysql"
)

type CNCMySQLHandler struct {
	conn *sql.DB
}

func (c *CNCMySQLHandler) UpdateSize(transact *sql.Tx, corpus string, size int64) error {
	_, err := transact.Exec("UPDATE corpora SET size = ? WHERE name = ?", size, corpus)
	return err
}

func (c *CNCMySQLHandler) UpdateDescription(transact *sql.Tx, corpus, descCs, descEn string) error {
	var err error
	if descCs != "" {
		_, err = transact.Exec("UPDATE corpora SET description_cs = ? WHERE name = ?", descCs, corpus)
	}
	if err != nil {
		return err
	}
	if descEn != "" {
		_, err = transact.Exec("UPDATE corpora SET description_en = ? WHERE name = ?", descEn, corpus)
	}
	return err
}

func (c *CNCMySQLHandler) LoadInfo(corpusID string) (*corpus.DBInfo, error) {
	var bibLabelStruct, bibLabelAttr, bibIDStruct, bibIDAttr sql.NullString
	row := c.conn.QueryRow(
		`SELECT name, active, bib_label_struct, bib_label_attr,
		   bib_id_struct, bib_id_attr, bib_group_duplicates, locale
		FROM corpora WHERE name = ?`, corpusID)
	var ans corpus.DBInfo
	err := row.Scan(
		&ans.Name,
		&ans.Active,
		&bibLabelStruct,
		&bibLabelAttr,
		&bibIDStruct,
		&bibIDAttr,
		&ans.BibGroupDuplicates,
		&ans.Locale,
	)
	if err != nil {
		return nil, err
	}
	if bibLabelStruct.Valid && bibLabelAttr.Valid {
		ans.BibLabelAttr = bibLabelStruct.String + "." + bibLabelAttr.String
	}
	if bibIDStruct.Valid && bibIDAttr.Valid {
		ans.BibIDAttr = bibIDStruct.String + "." + bibIDAttr.String
	}
	return &ans, nil

}

func (c *CNCMySQLHandler) StartTx() (*sql.Tx, error) {
	return c.conn.Begin()
}

func (c *CNCMySQLHandler) CommitTx(transact *sql.Tx) error {
	return transact.Commit()
}

func (c *CNCMySQLHandler) RollbackTx(transact *sql.Tx) error {
	return transact.Rollback()
}

func (c *CNCMySQLHandler) Conn() *sql.DB {
	return c.conn
}

func NewCNCMySQLHandler(host, user, pass, dbName string) (*CNCMySQLHandler, error) {
	conf := mysql.NewConfig()
	conf.Net = "tcp"
	conf.Addr = host
	conf.User = user
	conf.Passwd = pass
	conf.DBName = dbName
	conf.ParseTime = true
	conf.Loc = time.Local
	db, err := sql.Open("mysql", conf.FormatDSN())
	if err != nil {
		return nil, err
	}
	return &CNCMySQLHandler{conn: db}, nil
}
