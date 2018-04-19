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
	um		"ycp2p/discover/udpmsg"
	yclog	"ycp2p/logger"
)


//
// errno
//
const (
	NgbMgrEnoNone	= iota
	NgbMgrEnoParameter
	NgbMgrEnoNotImpl
	NgbMgrEnoUnknown
	NgbMgrEnoMax
)
type NgbMgrErrno int


//
// Neighbor task name (prefix): when created, the father must append something more
// to strcat this "name".
//
const NgbProcName = "ngbproc_"

//
// The control block of neighbor task instance
//
type neighborInstance struct {
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
type neighborManager struct {
	name		string					// name
	tep			sch.SchUserTaskEp		// entry
}

//
// It's a static task, only one instance would be
//
var ngbMgr = &neighborManager {
	name:	NgbMgrName,
	tep:	NgbMgrProc,
}

//
// Neighbor manager task entry
//
func NgbMgrProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	yclog.LogCallerFileLine("NgbMgrProc: scheduled, msg: %d", msg.Id)

	//
	// Messages are from udp listener task or table task. The former would dispatch udpmsgs
	// (which are decoded from raw protobuf messages received by UDP); the later, would request
	// us to init Ping procedure or FindNode procedure. By checking the sch.SchMessage.Id we
	// can determine what the current message for.
	//

	var eno NgbMgrErrno

	switch msg.Id {

	case sch.EvSchPoweron:
		return ngbMgr.PoweronHandler()

	case sch.EvSchPoweroff:
		return ngbMgr.PoweroffHandler()

	// udpmsg from udp listener task
	case sch.EvNblMsgInd:
		eno = ngbMgr.UdpMsgHandler(msg.Body.(*UdpMsgInd))

	// request to find node from table task
	case sch.EvNblFindNodeReq:
		eno = ngbMgr.FindNodeReq(msg.Body.(*um.FindNode))

	// request to ping from table task
	case sch.EvNblPingpongReq:
		eno = ngbMgr.PingpongReq(msg.Body.(*um.Ping))

	default:
		yclog.LogCallerFileLine("NgbMgrProc: invalid message id: %d", msg.Id)
		eno = NgbMgrEnoParameter
	}

	if eno != NgbMgrEnoNone {
		yclog.LogCallerFileLine("NgbMgrProc: errors, eno: %d", eno)
		return sch.SchEnoUserTask
	}

	return sch.SchEnoNone
}

//
// Poweron handler
//
func (ngbMgr *neighborManager)PoweronHandler() sch.SchErrno {
	return sch.SchEnoNone
}

//
// Poweroff handler
//
func (ngbMgr *neighborManager)PoweroffHandler() sch.SchErrno {
	return sch.SchEnoNotImpl
}

//
// udpmsg handler
//
func (ngbMgr *neighborManager)UdpMsgHandler(msg *UdpMsgInd) NgbMgrErrno {

	var eno NgbMgrErrno

	switch msg.msgType {

	case um.UdpMsgTypePing:
		eno = ngbMgr.PingHandler(msg.msgBody.(*um.Ping))

	case um.UdpMsgTypePong:
		eno = ngbMgr.PongHandler(msg.msgBody.(*um.Pong))

	case um.UdpMsgTypeFindNode:
		eno = ngbMgr.FindNodeHandler(msg.msgBody.(*um.FindNode))

	case um.UdpMsgTypeNeighbors:
		eno = ngbMgr.NeighborsHandler(msg.msgBody.(*um.Neighbors))

	default:
		yclog.LogCallerFileLine("NgbMgrUdpMsgHandler: invalid udp message type: %d", msg.msgType)
		eno = NgbMgrEnoParameter
	}

	if eno != NgbMgrEnoNone {
		yclog.LogCallerFileLine("NgbMgrProc: errors, eno: %d", eno)
	}
	return eno
}

//
// Ping handler
//
func (ngbMgr *neighborManager)PingHandler(ping *um.Ping) NgbMgrErrno {

	//
	// Here we are pinged by another node
	//

	return NgbMgrEnoNotImpl
}

//
// Pong handler
//
func (ngbMgr *neighborManager)PongHandler(pong *um.Pong) NgbMgrErrno {

	//
	// Here we got pong from another node
	//

	return NgbMgrEnoNotImpl
}

//
// FindNode handler
//
func (ngbMgr *neighborManager)FindNodeHandler(findNode *um.FindNode) NgbMgrErrno {

	//
	// Here we are requested to FindNode by another node
	//

	return NgbMgrEnoNotImpl
}

//
// Neighbors handler
//
func (ngbMgr *neighborManager)NeighborsHandler(nbs *um.Neighbors) NgbMgrErrno {

	//
	// Here we got Neighbors from another node
	//

	return NgbMgrEnoNotImpl
}

//
// FindNode request handler
//
func (ngbMgr *neighborManager)FindNodeReq(findNode *um.FindNode) NgbMgrErrno {

	//
	// Here we are requested to FindNode by local table task
	//

	return NgbMgrEnoNotImpl
}
//
// Pingpong(ping) request handler
//
func (ngbMgr *neighborManager)PingpongReq(ping *um.Ping) NgbMgrErrno {

	//
	// Here we are requested to Ping another node by local table task
	//

	return NgbMgrEnoNotImpl
}