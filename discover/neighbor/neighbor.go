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
	"net"
	"sync"
	sch		"ycp2p/scheduler"
	um		"ycp2p/discover/udpmsg"
	ycfg	"ycp2p/config"
	yclog	"ycp2p/logger"
)


//
// errno
//
const (
	NgbMgrEnoNone	= iota
	NgbMgrEnoParameter
	NgbMgrEnoNotImpl
	NgbMgrEnoEncode
	NgbMgrEnoUdp
	NgbMgrEnoDuplicated
	NgbMgrEnoMismatched
	NgbMgrEnoScheduler
	NgbMgrEnoUnknown
)
type NgbMgrErrno int

//
// EvNblFindNodeRsp message
//
type NblFindNodeRsp struct {
	Result		int				// result, 0: ok, others: errno
	FindNode	*um.FindNode	// FindNode message from table task
}

//
// EvNblPingpongrRsp message
//
type NblPingRsp struct {
	Result		int			// result, 0: ok, others: errno
	Ping		*um.Ping	// Ping message from table task
}

//
// Neighbor task name: since this type of instance is created dynamic, no fixed name defined,
// instead, peer node id string is applied as the task name. Please see how this type of task
// instance is created for more.
//
const NgbProcName = ""

//
// Mailbox size of a ngighbor instance
//
const ngbProcMailboxSize = 8

//
// The control block of neighbor task instance
//
type neighborInst struct {
	ptn		interface{}	// task node pointer
	name	string			// task instance name
	msgType	um.UdpMsgType	// message type to inited this instance
	msgBody	interface{}	// message body
}

//
// Protocol handler errno
//
const (
	NgbProtoEnoNone	= iota
	NgbProtoEnoTimeout
	NgbProtoEnoParameter
)

type NgbProtoErrno int

//
// Neighbor task entry
//
func NgbProtoProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	yclog.LogCallerFileLine("NgbProtoProc: scheduled, msg: %d", msg.Id)

	var protoEno NgbProtoErrno
	var inst *neighborInst

	inst = sch.SchinfGetUserDataArea(ptn).(*neighborInst)

	switch msg.Id {
	case sch.EvNblFindNodeReq:
		protoEno = inst.NgbProtoFindNodeReq()
	case sch.EvNblPingpongReq:
		protoEno = inst.NgbProtoPingReq()
	case sch.EvNblFindNodeTimer:
		protoEno = inst.NgbProtoFindNodeTimeout()
	case sch.EvNblPingpongTimer:
		protoEno = inst.NgbProtoPingTimeout()
	default:
		yclog.LogCallerFileLine("NgbProtoProc: invalid message, msg.Id: %d", msg.Id)
		protoEno = NgbProtoEnoParameter
	}

	if protoEno != NgbProtoEnoNone {
		return sch.SchEnoUserTask
	}

	return sch.SchEnoNone
}

//
// FindNode request handler
//
func (inst *neighborInst) NgbProtoFindNodeReq() NgbProtoErrno {
	// check FindNode request
	if inst.msgType != um.UdpMsgTypeFindNode || inst.msgBody == nil {
		yclog.LogCallerFileLine("NgbProtoFindNodeReq: invalid find node request")
		return NgbProtoEnoParameter
	}

	// setup FindNode request

	// encode request

	// send encoded request

	// start timer for response

	return NgbProtoEnoNone
}

//
// Ping request handler
//
func (inst *neighborInst) NgbProtoPingReq() NgbProtoErrno {
	// check Ping request
	if inst.msgType != um.UdpMsgTypePing || inst.msgBody == nil {
		yclog.LogCallerFileLine("NgbProtoPingReq: invalid ping request")
		return NgbProtoEnoParameter
	}

	// setup request

	// encode request

	// send encoded request

	// start time for response

	return NgbProtoEnoNone
}

//
// FindNode timeout handler
//
func (inst *neighborInst) NgbProtoFindNodeTimeout() NgbProtoErrno {
	// response FindNode timeout to table task

	// done the neighbor task instance

	return NgbProtoEnoNone
}

//
// Ping timeout handler
//
func (inst *neighborInst) NgbProtoPingTimeout() NgbProtoErrno {
	// response Ping timeout to table task

	// done the neighbor task instance

	return NgbProtoEnoNone
}

//
// Callbacked when died
//
func NgbProtoDieCb(ptn interface{}) sch.SchErrno {

	// here we are called while task is exiting, we need to free resources had been
	// allocated to the instance
	var inst *neighborInst

	// get user data pointer which points to our instance
	if inst = sch.SchinfGetUserDataArea(ptn).(*neighborInst); inst == nil {

		yclog.LogCallerFileLine("NgbProtoDieCb: " +
			"invalid user data area, name: %s",
			sch.SchinfGetTaskName(ptn))

		return sch.SchEnoInternal
	}

	// clean the map
	ngbMgr.cleanMap(inst.name)

	// More ... ?

	return sch.SchEnoNone
}

//
// Neighbor manager task name
//
const NgbMgrName = sch.NgbMgrName

//
// Control block of neighbor manager task
//
type neighborManager struct {
	lock		sync.Locker					// lock for protection
	name		string						// name
	tep			sch.SchUserTaskEp			// entry
	ptnMe		interface{}				// pointer to task node of myself
	ptnTab		interface{}				// pointer to task node of table task
	ngbMap		map[string]*neighborInst	// map neighbor node id to task node pointer
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
	// can determine what the current message for. Here, we play game like following:
	//
	// 1) for reqest from table task, the manager create a neighbor task to handle it, and
	// backups the peer node identity in a map;
	// 2) for udpmsg indication messages which could is not in the map, the manager response
	// them at once;
	// 3) for udpmsg indication messages which are from those peer nodes in the map, the manager
	// dispatch them to the neighbor task instances created in step 1);
	// 4) the neighbor task should install a callback to clean itself from the map when its'
	// instance done;
	//

	var eno NgbMgrErrno

	switch msg.Id {

	case sch.EvSchPoweron:
		return ngbMgr.PoweronHandler(ptn)

	case sch.EvSchPoweroff:
		return ngbMgr.PoweroffHandler(ptn)

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
func (ngbMgr *neighborManager)PoweronHandler(ptn interface{}) sch.SchErrno {
	ngbMgr.ptnMe = ptn
	eno, ptnTab := sch.SchinfGetTaskNodeByName(sch.TabMgrName)

	if 	eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("PoweronHandler: " +
			"SchinfGetTaskNodeByName failed, eno: %d, name: %s",
			eno, sch.TabMgrName)
		return eno
	}

	if ptnTab == nil {
		yclog.LogCallerFileLine("PoweronHandler: invalid table task node pointer")
		return sch.SchEnoUnknown
	}

	ngbMgr.ptnMe = ptn
	ngbMgr.ptnTab = ptnTab
	return sch.SchEnoNone
}

//
// Poweroff handler
//
func (ngbMgr *neighborManager)PoweroffHandler(ptn interface{}) sch.SchErrno {
	yclog.LogCallerFileLine("PoweroffHandler: done for poweroff event")
	return sch.SchinfTaskDone(ptn, sch.SchEnoKilled)
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
	// currently relay is not supported, check if we are the target, if false, we
	// just discard this ping message.
	//
	if ping.To.NodeId != lsnMgr.cfg.ID {
		yclog.LogCallerFileLine("PingHandler: not the target: %s", ycfg.P2pNodeId2HexString(lsnMgr.cfg.ID))
		return NgbMgrEnoParameter
	}

	// some procedures about the peer node might be running, but we do not care this
	// to check the neighbor task map, we just send pong simply.
	pong := um.Pong{
		From:		ping.To,
		To:			ping.From,
		Expiration:	0,
		Extra:		nil,
	}

	toAddr := net.UDPAddr {
		IP: 	ping.From.IP,
		Port:	int(ping.From.UDP),
		Zone:	"",
	}

	pum := new(um.UdpMsg)
	if eno := pum.Encode(um.UdpMsgTypePong, &pong); eno != NgbMgrEnoNone {
		yclog.LogCallerFileLine("PingHandler: Encode failed, eno: %d", eno)
		return NgbMgrEnoEncode
	}

	if eno := sendUdpMsg(pum.Buf, &toAddr); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("PingHandler: sendUdpMsg failed, eno: %d", eno)
		return NgbMgrEnoUdp
	}

	return NgbMgrEnoNone
}

//
// Pong handler
//
func (ngbMgr *neighborManager)PongHandler(pong *um.Pong) NgbMgrErrno {

	//
	// Here we got pong from another node
	//
	// currently relay is not supported, check if we are the target, if false, we
	// just discard this ping message.
	//

	if pong.To.NodeId != lsnMgr.cfg.ID {
		yclog.LogCallerFileLine("PongHandler: not the target: %s", ycfg.P2pNodeId2HexString(lsnMgr.cfg.ID))
		return NgbMgrEnoParameter
	}

	strPeerNodeId := ycfg.P2pNodeId2HexString(pong.From.NodeId)
	if ngbMgr.checkMap(strPeerNodeId) == false {
		yclog.LogCallerFileLine("PongHandler: neighbor isntance not exist: %s", strPeerNodeId)
		return NgbMgrEnoMismatched
	}
	ptnNgb := ngbMgr.getMap(strPeerNodeId).ptn

	var schMsg = sch.SchMessage{}
	if eno := sch.SchinfMakeMessage(&schMsg, ngbMgr.ptnMe, ptnNgb, sch.EvNblPingpongRsp, pong); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("PongHandler: SchinfMakeMessage failed, eno: %d", eno)
		return NgbMgrEnoScheduler
	}

	if eno := sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("PongHandler: SchinfSendMsg2Task failed, eno: %d", eno)
		return NgbMgrEnoScheduler
	}

	return NgbMgrEnoNone
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
	// currently relay is not supported, check if we are the target, if false, we
	// just discard this ping message.
	//

	if nbs.To.NodeId != lsnMgr.cfg.ID {
		yclog.LogCallerFileLine("NeighborsHandler: not the target: %s", ycfg.P2pNodeId2HexString(lsnMgr.cfg.ID))
		return NgbMgrEnoParameter
	}

	strPeerNodeId := ycfg.P2pNodeId2HexString(nbs.From.NodeId)
	if ngbMgr.checkMap(strPeerNodeId) == false {
		yclog.LogCallerFileLine("NeighborsHandler: neighbor isntance not exist: %s", strPeerNodeId)
		return NgbMgrEnoMismatched
	}
	ptnNgb := ngbMgr.getMap(strPeerNodeId).ptn

	var schMsg = sch.SchMessage{}
	if eno := sch.SchinfMakeMessage(&schMsg, ngbMgr.ptnMe, ptnNgb, sch.EvNblFindNodeRsp, nbs); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("NeighborsHandler: SchinfMakeMessage failed, eno: %d", eno)
		return NgbMgrEnoScheduler
	}

	if eno := sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("NeighborsHandler: SchinfSendMsg2Task failed, eno: %d", eno)
		return NgbMgrEnoScheduler
	}

	return NgbMgrEnoNone
}

//
// FindNode request handler
//
func (ngbMgr *neighborManager)FindNodeReq(findNode *um.FindNode) NgbMgrErrno {

	//
	// Here we are requested to FindNode by local table task: we encode and send the message to
	// destination node, and the create a neighbor task to deal with the findnode procedure. See
	// comments in NgbMgrProc for more pls.
	//

	var rsp = NblFindNodeRsp{}
	var schMsg  = sch.SchMessage{}

	var funcRsp2Tab = func () NgbMgrErrno {
		if eno := sch.SchinfMakeMessage(&schMsg, ngbMgr.ptnMe, ngbMgr.ptnTab, sch.EvNblFindNodeRsp, &rsp); eno != sch.SchEnoNone {
			yclog.LogCallerFileLine("FindNodeReq: SchinfMakeMessage failed, eno: %d", eno)
			return NgbMgrEnoScheduler
		}

		if eno := sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
			yclog.LogCallerFileLine("FindNodeReq: "+
				"SchinfSendMsg2Task failed, eno: %d, sender: %s, recver: %s",
				eno,
				sch.SchinfGetMessageSender(&schMsg),
				sch.SchinfGetMessageRecver(&schMsg))
			return NgbMgrEnoScheduler
		}
		return NgbMgrEnoNone
	}

	// check if duplicated: if true, tell table task it's duplicated by event EvNblFindNodeRsp
	strPeerNodeId := ycfg.P2pNodeId2HexString(findNode.To.NodeId)

	if ngbMgr.checkMap(strPeerNodeId) {

		yclog.LogCallerFileLine("FindNodeReq: duplicated neighbor instance: %s", strPeerNodeId)

		rsp.Result = NgbMgrEnoDuplicated
		rsp.FindNode = ngbMgr.getMap(strPeerNodeId).msgBody.(*um.FindNode)
		return funcRsp2Tab()
	}

	// send findnode message
	to := &net.UDPAddr{
		IP:		findNode.To.IP,
		Port:	int(findNode.To.UDP),
		Zone:	"",
	}
	pum := new(um.UdpMsg)

	if eno := pum.Encode(um.UdpMsgTypeFindNode, findNode); eno != um.UdpMsgEnoNone {
		yclog.LogCallerFileLine("FindNodeReq: Encode failed, eno: %d", eno)
		return NgbMgrEnoEncode
	}

	if eno := sendUdpMsg(pum.Buf, to); eno != um.UdpMsgEnoNone {
		yclog.LogCallerFileLine("FindNodeReq: sendUdpMsg failed, eno: %d", eno)
		return NgbMgrEnoUdp
	}

	// create a neighbor instance and setup the map
	var ngbInst = neighborInst {
		ptn:		nil,
		name:		strPeerNodeId,
		msgType:	um.UdpMsgTypeFindNode,
		msgBody:	findNode,
	}

	var noDog = sch.SchWatchDog {
		HaveDog:false,
	}

	var dc = sch.SchTaskDescription {
		Name:	strPeerNodeId,
		MbSize:	ngbProcMailboxSize,
		Ep:		NgbProtoProc,
		Wd:		noDog,
		Flag:	sch.SchCreatedGo,
		DieCb:	NgbProtoDieCb,
		UserDa: &ngbInst,
	}

	eno, ptn := sch.SchinfCreateTask(&dc)
	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("FindNodeReq: SchinfCreateTask failed, eno: %d", eno)
		return NgbMgrEnoScheduler
	}
	ngbInst.ptn = ptn
	ngbMgr.setupMap(strPeerNodeId, &ngbInst)

	rsp.Result = NgbMgrEnoNone
	rsp.FindNode = ngbMgr.getMap(strPeerNodeId).msgBody.(*um.FindNode)

	if eno := sch.SchinfMakeMessage(&schMsg, ngbMgr.ptnMe, ngbInst.ptn, sch.EvNblFindNodeReq, nil); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("FindNodeReq: SchinfMakeMessage failed, eno: %d", eno)
		return NgbMgrEnoScheduler
	}

	if eno := sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("FindNodeReq: SchinfSendMsg2Task failed, eno: %d", eno)
		return NgbMgrEnoScheduler
	}

	return funcRsp2Tab()
}

//
// Pingpong(ping) request handler
//
func (ngbMgr *neighborManager)PingpongReq(ping *um.Ping) NgbMgrErrno {

	//
	// Here we are requested to Ping another node by local table task
	//

	var rsp = NblPingRsp{}
	var schMsg  = sch.SchMessage{}

	var funcRsp2Tab = func () NgbMgrErrno {
		if eno := sch.SchinfMakeMessage(&schMsg, ngbMgr.ptnMe, ngbMgr.ptnTab, sch.EvNblPingpongRsp, &rsp); eno != sch.SchEnoNone {
			yclog.LogCallerFileLine("PingpongReq: SchinfMakeMessage failed, eno: %d", eno)
			return NgbMgrEnoScheduler
		}

		if eno := sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
			yclog.LogCallerFileLine("PingpongReq: "+
				"SchinfSendMsg2Task failed, eno: %d, sender: %s, recver: %s",
				eno,
				sch.SchinfGetMessageSender(&schMsg),
				sch.SchinfGetMessageRecver(&schMsg))
			return NgbMgrEnoScheduler
		}
		return NgbMgrEnoNone
	}

	// check if duplicated: if true, tell table task it's duplicated by event EvNblFindNodeRsp
	strPeerNodeId := ycfg.P2pNodeId2HexString(ping.To.NodeId)

	if ngbMgr.checkMap(strPeerNodeId) {

		yclog.LogCallerFileLine("PingpongReq: duplicated neighbor instance: %s", strPeerNodeId)

		rsp.Result = NgbMgrEnoDuplicated
		rsp.Ping = ngbMgr.getMap(strPeerNodeId).msgBody.(*um.Ping)
		return funcRsp2Tab()
	}

	// send findnode message
	to := &net.UDPAddr{
		IP:		ping.To.IP,
		Port:	int(ping.To.UDP),
		Zone:	"",
	}
	pum := new(um.UdpMsg)

	if eno := pum.Encode(um.UdpMsgTypeFindNode, ping); eno != um.UdpMsgEnoNone {
		yclog.LogCallerFileLine("PingpongReq: Encode failed, eno: %d", eno)
		return NgbMgrEnoEncode
	}

	if eno := sendUdpMsg(pum.Buf, to); eno != um.UdpMsgEnoNone {
		yclog.LogCallerFileLine("PingpongReq: sendUdpMsg failed, eno: %d", eno)
		return NgbMgrEnoUdp
	}

	// create a neighbor instance and setup the map
	var ngbInst = neighborInst {
		ptn:		nil,
		name:		strPeerNodeId,
		msgType:	um.UdpMsgTypePing,
		msgBody:	ping,
	}

	var noDog = sch.SchWatchDog {
		HaveDog:false,
	}

	var dc = sch.SchTaskDescription {
		Name:	strPeerNodeId,
		MbSize:	ngbProcMailboxSize,
		Ep:		NgbProtoProc,
		Wd:		noDog,
		Flag:	sch.SchCreatedGo,
		DieCb:	NgbProtoDieCb,
		UserDa: &ngbInst,
	}

	eno, ptn := sch.SchinfCreateTask(&dc)
	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("PingpongReq: SchinfCreateTask failed, eno: %d", eno)
		return NgbMgrEnoScheduler
	}
	ngbInst.ptn = ptn
	ngbMgr.setupMap(strPeerNodeId, &ngbInst)

	rsp.Result = NgbMgrEnoNone
	rsp.Ping = ngbMgr.getMap(strPeerNodeId).msgBody.(*um.Ping)
	return funcRsp2Tab()
}

//
// Setup map for neighbor instance
//
func (ngbMgr *neighborManager) setupMap(name string, inst *neighborInst) {
	ngbMgr.lock.Lock()
	defer ngbMgr.lock.Unlock()
	ngbMgr.ngbMap[name] = inst
}

//
// Clean map for neighbor instance
//
func (ngbMgr *neighborManager) cleanMap(name string) {
	ngbMgr.lock.Lock()
	defer ngbMgr.lock.Unlock()
	delete(ngbMgr.ngbMap, name)
}

//
// Check map for neighbor instance
//
func (ngbMgr *neighborManager) checkMap(name string) bool {
	ngbMgr.lock.Lock()
	defer ngbMgr.lock.Unlock()
	 _, ok := ngbMgr.ngbMap[name]
	 return ok
}

//
// Get instance from map
//
func (ngbMgr *neighborManager) getMap(name string) *neighborInst {
	ngbMgr.lock.Lock()
	defer ngbMgr.lock.Unlock()
	return ngbMgr.ngbMap[name]
}