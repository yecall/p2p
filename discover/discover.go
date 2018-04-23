/*
 *  Copyright (C) 2017 gyee authors
 *
 *  This file is part of the gyee library.
 *
 *  the gyee library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  the gyee library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License
 *  along with the gyee library.  If not, see <http://www.gnu.org/licenses/>.
 *
 */


package discover

import (
	sch 	"ycp2p/scheduler"
	yclog	"ycp2p/logger"
)

//
// Discover manager
//
const DcvMgrName = sch.DcvMgrName

type discoverManager struct {
	name	string				// name
	tep		sch.SchUserTaskEp	// entry
}

var dcvMgr = discoverManager{
	name:	DcvMgrName,
	tep:	DcvMgrProc,
}

//
// Discover manager entry
//
func DcvMgrProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {
	yclog.LogCallerFileLine("DcvMgrProc: scheduled, msg: %d", msg.Id)

	switch msg.Id {
	case sch.EvSchPoweron:
	case sch.EvSchPoweroff:
	case sch.EvDcvFindNodeReq:
	case sch.EvTabRefreshRsp:
	default:
	}
	return sch.SchEnoNone
}
