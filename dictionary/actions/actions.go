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
	"context"
	"database/sql"
	"masm/v3/cncdb"
	"masm/v3/corpus"
	"masm/v3/general"
	"masm/v3/jobs"
	"masm/v3/liveattrs/laconf"
)

type Actions struct {
	corpConf *corpus.CorporaSetup

	laConfCache *laconf.LiveAttrsBuildConfProvider

	// ctx controls cancellation
	ctx context.Context

	// jobStopChannel receives job ID based on user interaction with job HTTP API in
	// case users asks for stopping the vte process
	jobStopChannel <-chan string

	jobActions *jobs.Actions

	// laDB is a live-attributes-specific database where masm needs full privileges
	laDB *sql.DB

	// cncDB is CNC's main database
	cncDB *cncdb.CNCMySQLHandler
}

// NewActions is the default factory for Actions
func NewActions(
	ctx context.Context,
	corpConf *corpus.CorporaSetup,
	jobStopChannel <-chan string,
	jobActions *jobs.Actions,
	cncDB *cncdb.CNCMySQLHandler,
	laDB *sql.DB,
	laConfRegistry *laconf.LiveAttrsBuildConfProvider,
	version general.VersionInfo,
) *Actions {
	actions := &Actions{
		ctx:            ctx,
		corpConf:       corpConf,
		jobActions:     jobActions,
		jobStopChannel: jobStopChannel,
		laConfCache:    laConfRegistry,
		cncDB:          cncDB,
		laDB:           laDB,
	}
	return actions
}
