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
//
// This struct tracks processing times in misc. parts of liveattrs
//

package db

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
)

var (
	ErrorEstimationNotAvail = errors.New("estimation not available")
)

func AddProcTimeEntry(
	db *sql.DB,
	procType string,
	dataSize, numItems int,
	procTime float64,
) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to add proc_times entry: %w", err)
	}
	_, err = tx.Exec(
		"INSERT INTO proc_times (data_size, proc_type, num_items, proc_time) "+
			"VALUES (?, ?, ?, ?)",
		dataSize, procType, numItems, procTime,
	)
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to add proc_times entry: %w", err)
	}
	return err
}

func EstimateProcTimeSecs(db *sql.DB, procType string, dataSize int) (int, error) {
	row := db.QueryRow(
		"SELECT SUM(t2.num_items) AS total_items, SUM(t2.proc_time) AS total_time "+
			"FROM "+
			"(SELECT MIN(ABS(t1.data_size - ?)) AS dist, data_size, proc_type "+
			"FROM proc_times t1 "+
			"WHERE t1.proc_type = ?"+
			") AS min_item "+
			"JOIN proc_times t2 "+
			"  ON t2.data_size = min_item.data_size AND t2.proc_type = min_item.proc_type ",
		dataSize, procType,
	)
	if row.Err() != nil {
		return -1, row.Err()
	}
	var totalItems sql.NullInt64
	var totalTime sql.NullFloat64
	err := row.Scan(&totalItems, &totalTime)
	if err != nil {
		return -1, err
	}
	if !totalItems.Valid || !totalTime.Valid {
		return -1, ErrorEstimationNotAvail
	}
	return int(math.RoundToEven(float64(totalItems.Int64) / totalTime.Float64)), nil
}
