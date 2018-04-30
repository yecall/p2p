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
// errno
//
const (
	DcvMgrEnoNone		= iota
	DcvMgrEnoParameter
	DcvMgrEnoScheduler
)

type DcvMgrErrno int

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

	var eno DcvMgrErrno = DcvMgrEnoNone

	switch msg.Id {
	case sch.EvSchPoweron:
		eno = DcvMgrPoweron()
	case sch.EvSchPoweroff:
		eno = DcvMgrPoweroff(ptn)
	case sch.EvDcvFindNodeReq:
		eno = DcvMgrFindNodeReq(msg.Body.(*sch.MsgDcvFindNodeReq))
	case sch.EvTabRefreshRsp:
		eno = DcvMgrTabRefreshRsp(msg.Body.(*sch.MsgTabRefreshRsp))
	default:
		yclog.LogCallerFileLine("DcvMgrProc: invalid message: %d", msg.Id)
		return sch.SchEnoUserTask
	}

	if eno != DcvMgrEnoNone {
		yclog.LogCallerFileLine("DcvMgrProc: errors, eno: %d", eno)
		return sch.SchEnoUserTask
	}

	return sch.SchEnoNone
}

//
// Poweron handler
//
func DcvMgrPoweron() DcvMgrErrno {
	return DcvMgrEnoNone
}


//
// Poweroff handler
//
func DcvMgrPoweroff(ptn interface{}) DcvMgrErrno {
	return DcvMgrEnoNone
}

//
// FindNode request handler
//
func DcvMgrFindNodeReq(req *sch.MsgDcvFindNodeReq) DcvMgrErrno {

	//
	// When peer manager task considers that more peers needed, it then send FindNode
	// request to here the discover task to ask for more, see function peMgrAsk4More
	// for details about please.
	//
	// When EvDcvFindNodeReq received, we should requtst the table manager task to
	// refresh itself to get more by sending sch.EvTabRefreshReq to it, and we would
	// responsed by sch.EvTabRefreshRsp, with message type as sch.MsgTabRefreshRsp.
	// And then, we can response the peer manager task with sch.EvDcvFindNodeRsp event
	// with sch.MsgDcvFindNodeRsp message.
	//

	return DcvMgrEnoNone
}

//
// Table refreshed response handler
//
func DcvMgrTabRefreshRsp(rsp *sch.MsgTabRefreshRsp) DcvMgrErrno {

	//
	// We receive the response about event sch.EvTabRefreshReq we hand sent to table
	// manager task. For more, see comments aboved in function DcvMgrFindNodeReq pls.
	//

	return DcvMgrEnoNone
}




