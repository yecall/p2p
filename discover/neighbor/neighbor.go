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


package neighbor

import (
	sch		"ycp2p/scheduler"
	yclog	"ycp2p/logger"
)

//
// Neighbor task name (prefix): when created, the father must append something more
// to strcat this "name".
//
const NgbProcName = "ngbproc_"

//
// The control block of neighbor task instance
//
type ngbCtrlBlock struct {
	name	string
}

//
// Neighbor task entry
//
func NgbProtoProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {
	yclog.LogCallerFileLine("NgbProtoProc: scheduled, msg: %d", msg.Id)
	return sch.SchEnoNone
}

//
// Neighbor manager task name
//
const NgbMgrName = "ngbmgr"

//
// Control block of neighbor manager task
//
type ngbMgrCtrlBlock struct {
	name	string
}

//
// Neighbor manager task entry
//
func NgbMgrProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {
	yclog.LogCallerFileLine("NgbMgrProc: scheduled, msg: %d", msg.Id)
	return sch.SchEnoNone
}
