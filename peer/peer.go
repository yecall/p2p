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


package peer

import (
	"net"
	"time"
	ycfg	"ycp2p/config"
	sch 	"ycp2p/scheduler"
	yclog	"ycp2p/logger"
)

//
// Peer manager errno
//
const (
	PeMgrEnoNone	= iota
	PeMgrEnoParameter
	PeMgrEnoScheduler
	PeMgrEnoConfig
	PeMgrEnoResource
	PeMgrEnoOs
	PeMgrEnoMessage
	PeMgrEnoUnsup
	PeMgrEnoInternal
)

type PeMgrErrno int

//
// Peer identity as string
//
type PeerId ycfg.NodeID

//
// Peer name as string
//
type PeerName string

//
// Peer capability as map
//
type PeerProtoType string
type PeerProtoVersion string
type PeerCap map[PeerProtoType] PeerProtoVersion

//
// Peer information
//
type PeerInfo struct {
	Id		PeerId		// identity
	Name	PeerName	// name
	Cap		PeerCap		// capability
}

//
// Peer manager configuration
//
const defaultConnectTimeout = 15 * time.Second	// default dial outbound timeout value, currently
												// it's a fixed value here than can be configurated
												// by other module.

const defaultHandshakeTimeout = 5 * time.Second	// default handshake timeout value, currently
												// it's a fixed value here than can be configurated
												// by other module.

const maxTcpmsgSize = 1024*1024*4				// max size of a tcpmsg package could be, currently
												// it's a fixed value here than can be configurated
												// by other module.

type peMgrConfig struct {
	maxPeers		int				// max peers would be
	maxOutbounds	int				// max concurrency outbounds
	maxInBounds		int				// max concurrency inbounds
	ip				net.IP			// ip address
	port			uint16			// port numbers
	nodeId			ycfg.NodeID		// the node's public key
	statics			[]*ycfg.Node	// statics nodes
	noDial			bool			// do not dial outbound
	bootstrapNode	bool			// local is a bootstrap node
	defaultCto		time.Duration	// default connect outbound timeout
	defaultHto		time.Duration	// default handshake timeout
	maxMsgSize		int				// max tcpmsg package size
	protoNum		uint32			// local protocol number
	protocols		[]Protocol		// local protocol table
}

//
// Statistics history
//
type peHistory struct {
	tmBegin		time.Time	// time begin to count
	cntOk		int			// counter for succeed to establish
	cntFailed	int			// counter for failed to establish
}

//
// Peer manager
//
const PeerMgrName = sch.PeerMgrName

type peerManager struct {
	name			string							// name
	tep				sch.SchUserTaskEp				// entry
	cfg				peMgrConfig						// configuration
	ptnMe			interface{}						// pointer to myself(peer manager task node)
	ptnTab			interface{}						// pointer to table task node
	ptnLsn			interface{}						// pointer to peer listener manager task node
	ptnAcp			interface{}						// pointer to peer acceptor manager task node
	ptnDcv			interface{}						// pointer to discover task node
	peers			map[interface{}]*peerInstance	// map peer instance's task node pointer to instance pointer
	nodes			map[ycfg.NodeID]*peerInstance	// map peer node identity to instance pointer
	workers			map[ycfg.NodeID]*peerInstance	// map peer node identity to pointer of instance in work
	wrkNum			int								// worker peer number
	ibpNum			int								// active inbound peer number
	obpNum			int								// active outbound peer number
	acceptPaused	bool							// if accept task paused
	randoms			[]*ycfg.Node					// random nodes found by discover
	stats			map[ycfg.NodeID]peHistory		// history for successful and failed
}

var peMgr = peerManager{
	name:			PeerMgrName,
	tep:			PeerMgrProc,
	cfg:			peMgrConfig{},
	ptnMe:			nil,
	ptnTab:			nil,
	ptnLsn:			nil,
	ptnAcp:			nil,
	peers:			map[interface{}]*peerInstance{},
	nodes:			map[ycfg.NodeID]*peerInstance{},
	workers:		map[ycfg.NodeID]*peerInstance{},
	wrkNum:			0,
	ibpNum:			0,
	obpNum:			0,
	acceptPaused:	false,
	randoms:		[]*ycfg.Node{},
	stats:			map[ycfg.NodeID]peHistory{},
}

//
// Peer manager entry
//
func PeerMgrProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	yclog.LogCallerFileLine("PeerMgrProc: scheduled, msg: %d", msg.Id)

	var schEno sch.SchErrno = sch.SchEnoNone
	var eno PeMgrErrno = PeMgrEnoNone

	switch msg.Id {
	case sch.EvSchPoweron:
		eno = peMgrPoweron(ptn)
	case sch.EvSchPoweroff:
		eno = peMgrPoweroff()
	case sch.EvPeMgrStartReq:
		eno = peMgrStartReq(msg.Body)
	case sch.EvDcvFindNodeRsp:
		eno = peMgrDcvFindNodeRsp(msg.Body)
	case sch.EvPeLsnConnAcceptedInd:
		eno = peMgrLsnConnAcceptedInd(msg.Body)
	case sch.EvPeOutboundReq:
		eno = peMgrOutboundReq(msg.Body)
	case sch.EvPeConnOutRsp:
		eno = peMgrConnOutRsp(msg.Body)
	case sch.EvPeHandshakeRsp:
		eno = peMgrHandshakeRsp(msg.Body)
	case sch.EvPePingpongRsp:
		eno = peMgrPingpongRsp(msg.Body)
	case sch.EvPeCloseReq:
		eno = peMgrCloseReq(msg.Body)
	case sch.EvPeCloseCfm:
		eno = peMgrConnCloseCfm(msg.Body)
	case sch.EvPeCloseInd:
		eno = peMgrConnCloseInd(msg.Body)
	default:
		yclog.LogCallerFileLine("PeerMgrProc: invalid message: %d", msg.Id)
		eno = PeMgrEnoParameter
	}

	if eno != PeMgrEnoNone {
		yclog.LogCallerFileLine("PeerMgrProc: errors, eno: %d", eno)
		schEno = sch.SchEnoUserTask
	}

	return schEno
}

//
// Poweron event handler
//
func peMgrPoweron(ptn interface{}) PeMgrErrno {

	var eno = sch.SchEnoNone

	// backup pointers of related tasks
	peMgr.ptnMe	= ptn
	eno, peMgr.ptnTab = sch.SchinfGetTaskNodeByName(sch.TabMgrName)
	if eno != sch.SchEnoNone || peMgr.ptnTab == nil {
		yclog.LogCallerFileLine("peMgrPoweron: " +
			"SchinfGetTaskNodeByName failed, eno: %df, target: %s",
			eno, sch.TabMgrName)
		return PeMgrEnoScheduler
	}

	eno, peMgr.ptnLsn = sch.SchinfGetTaskNodeByName(PeerLsnMgrName)
	if eno != sch.SchEnoNone || peMgr.ptnTab == nil {
		yclog.LogCallerFileLine("peMgrPoweron: " +
			"SchinfGetTaskNodeByName failed, eno: %df, target: %s",
			eno, PeerLsnMgrName)
		return PeMgrEnoScheduler
	}

	eno, peMgr.ptnAcp = sch.SchinfGetTaskNodeByName(acceptProcName)
	if eno != sch.SchEnoNone || peMgr.ptnTab == nil {
		yclog.LogCallerFileLine("peMgrPoweron: " +
			"SchinfGetTaskNodeByName failed, eno: %df, target: %s",
			eno, acceptProcName)
		return PeMgrEnoScheduler
	}

	eno, peMgr.ptnDcv = sch.SchinfGetTaskNodeByName(sch.DcvMgrName)
	if eno != sch.SchEnoNone || peMgr.ptnDcv == nil {
		yclog.LogCallerFileLine("peMgrPoweron: " +
			"SchinfGetTaskNodeByName failed, eno: %d, target: %s",
			eno, sch.DcvMgrName)
		return PeMgrEnoScheduler
	}

	// fetch configration
	var cfg *ycfg.Cfg4PeerManager = nil
	if cfg := ycfg.P2pConfig4PeerManager(); cfg == nil {
		yclog.LogCallerFileLine("peMgrPoweron: P2pConfig4PeerManager failed")
		return PeMgrEnoConfig
	}
	peMgr.cfg = peMgrConfig {
		maxPeers:		cfg.MaxPeers,
		maxOutbounds:	cfg.MaxOutbounds,
		maxInBounds:	cfg.MaxInBounds,
		ip:				cfg.IP,
		port:			cfg.Port,
		nodeId:			cfg.ID,
		statics:		cfg.Statics,
		noDial:			cfg.NoDial,
		bootstrapNode:	cfg.BootstrapNode,
		defaultCto:		defaultConnectTimeout,
		defaultHto:		defaultHandshakeTimeout,
		maxMsgSize:		maxTcpmsgSize,
	}

	return PeMgrEnoNone
}

//
// Poweroff event handler
//
func peMgrPoweroff() PeMgrErrno {

	yclog.LogCallerFileLine("peMgrPoweroff: pwoeroff received, done the task")

	if eno := sch.SchinfTaskDone(peMgr.ptnMe, sch.SchEnoKilled); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrPoweroff: SchinfTaskDone failed, eno: %d", eno)
		return PeMgrEnoScheduler
	}

	return PeMgrEnoNone
}

//
// Peer manager start request handler
//
func peMgrStartReq(msg interface{}) PeMgrErrno {

	//
	// Notice: when this event received, we are required startup to deal with
	// peers in both inbound and outbound direction. For inbound, the manager
	// can control the inbound listener with event sch.EvPeLsnStartReq; while
	// for outbound, the event sch.EvPeOutboundReq, is for self-driven for the
	// manager. This is the basic, but when to start the inbound and outbound
	// might be considerable, since it's security issues related. Currently,
	// we simply start both as the "same time" here in this function, one can
	// start outbound firstly, and then counte the successful outbound peers,
	// at last, start inbound when the number of outbound peers reach a predefined
	// threshold, and son on.
	//

	_ = msg

	var schMsg = sch.SchMessage{}
	var eno = sch.SchEnoNone

	// start peer listener
	eno = sch.SchinfMakeMessage(&schMsg, peMgr.ptnMe, peMgr.ptnLsn, sch.EvPeLsnStartReq, nil)
	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrStartReq: " +
			"SchinfMakeMessage for EvPeLsnStartReq failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}

	eno = sch.SchinfSendMsg2Task(&schMsg)
	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrStartReq: " +
			"SchinfSendMsg2Task for EvPeLsnConnAcceptedInd failed, target: %s",
			sch.SchinfGetTaskName(peMgr.ptnLsn))
		return PeMgrEnoScheduler
	}

	// drive ourself to startup outbound
	eno = sch.SchinfMakeMessage(&schMsg, peMgr.ptnMe, peMgr.ptnMe, sch.EvPeOutboundReq, nil)
	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrStartReq: " +
			"SchinfMakeMessage for EvPeOutboundReq failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}

	eno = sch.SchinfSendMsg2Task(&schMsg)
	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrStartReq: " +
			"SchinfSendMsg2Task for EvPeOutboundReq failed, target: %s",
			sch.SchinfGetTaskName(peMgr.ptnMe))
		return PeMgrEnoScheduler
	}

	return PeMgrEnoNone
}

//
// FindNode response handler
//
func peMgrDcvFindNodeRsp(msg interface{}) PeMgrErrno {

	//
	// Here we got response about FindNode from discover task, which should contain
	// nodes could be try to connect to. We should check the number of the active
	// active outbound peer number currently to carry out action accordingly.
	//

	var rsp = msg.(*sch.MsgDcvFindNodeRsp)
	if rsp == nil {
		yclog.LogCallerFileLine("peMgrDcvFindNodeRsp: invalid FindNode response")
		return PeMgrEnoParameter
	}

	// Deal with each node responsed
	var appended = 0
	for _, n := range rsp.Nodes {

		// Check if duplicated instances
		if _, ok := peMgr.nodes[n.ID]; ok {
			continue
		}

		// Check if duplicated randoms
		for _, rn := range peMgr.randoms {
			if rn.ID == n.ID {
				continue
			}
		}

		// Check if duplicated statics
		for _, s := range peMgr.cfg.statics {
			if s.ID == n.ID {
				continue
			}
		}

		// backup node, max to the number of most peers can be
		peMgr.randoms = append(peMgr.randoms, n)
		if appended++; len(peMgr.randoms) >= peMgr.cfg.maxPeers {
			yclog.LogCallerFileLine("peMgrDcvFindNodeRsp: too much, some are discarded")
			break
		}
	}

	// drive ourself to startup outbound if some nodes appended
	if appended > 0 {

		var schMsg sch.SchMessage
		eno := sch.SchinfMakeMessage(&schMsg, peMgr.ptnMe, peMgr.ptnMe, sch.EvPeOutboundReq, nil)
		if eno != sch.SchEnoNone {
			yclog.LogCallerFileLine("peMgrDcvFindNodeRsp: " +
				"SchinfMakeMessage for EvPeOutboundReq failed, eno: %d",
				eno)
			return PeMgrEnoScheduler
		}

		eno = sch.SchinfSendMsg2Task(&schMsg)
		if eno != sch.SchEnoNone {
			yclog.LogCallerFileLine("peMgrDcvFindNodeRsp: " +
				"SchinfSendMsg2Task for EvPeOutboundReq failed, target: %s",
				sch.SchinfGetTaskName(peMgr.ptnMe))
			return PeMgrEnoScheduler
		}
	}

	return PeMgrEnoNone
}

//
// Peer connection accepted indication handler
//
func peMgrLsnConnAcceptedInd(msg interface{}) PeMgrErrno {

	//
	// Here we are indicated that an inbound connection had been accepted. We should
	// check the number of the active inbound peer number currently to carry out action
	// accordingly.
	//

	var eno = sch.SchEnoNone
	var ptnInst interface{} = nil

	// Check if more inbound allowed
	if peMgr.ibpNum >= peMgr.cfg.maxInBounds {
		yclog.LogCallerFileLine("peMgrLsnConnAcceptedInd: " +
			"no more resources, ibpNum: %d, max: %d",
			peMgr.ibpNum, peMgr.cfg.maxInBounds)
		return PeMgrEnoResource
	}

	//
	// Init peer instance control block
	//
	var ibInd = msg.(*msgConnAcceptedInd)
	var peInst = new(peerInstance)
	*peInst				= peerInstDefault
	peInst.ptnMgr		= peMgr.ptnMe
	peInst.state		= peInstStateAccepted
	peInst.cto			= peMgr.cfg.defaultCto
	peInst.hto			= peMgr.cfg.defaultHto
	peInst.maxPkgSize	= peMgr.cfg.maxMsgSize
	peInst.dialer		= nil
	peInst.conn			= ibInd.conn
	peInst.laddr		= ibInd.localAddr
	peInst.raddr		= ibInd.remoteAddr
	peInst.dir			= PeInstDirInbound

	//
	// Create peer instance task
	//
	var tskDesc  = sch.SchTaskDescription {
		Name:		acceptProcName,
		MbSize:		PeInstMailboxSize,
		Ep:			PeerInstProc,
		Wd:			nil,
		Flag:		sch.SchCreatedGo,
		DieCb:		nil,
		UserDa:		peInst,
	}

	if eno, ptnInst = sch.SchinfCreateTask(&tskDesc);
		eno != sch.SchEnoNone || ptnInst == nil {
		yclog.LogCallerFileLine("peMgrLsnConnAcceptedInd: " +
			"SchinfCreateTask failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}
	peInst.ptnMe = ptnInst

	//
	// Check the map
	//
	if _, dup := peMgr.peers[peInst]; dup {
		yclog.LogCallerFileLine("peMgrLsnConnAcceptedInd: " +
			"impossible duplicated peer instance")
		return PeMgrEnoInternal
	}

	//
	// Send handshake request to the instance created aboved
	//
	var schMsg = sch.SchMessage{}
	eno = sch.SchinfMakeMessage(&schMsg, peMgr.ptnMe, peInst.ptnMe, sch.EvPeHandshakeReq, nil)
	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrLsnConnAcceptedInd: " +
			"SchinfMakeMessage failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}

	if eno = sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrLsnConnAcceptedInd: " +
			"SchinfSendMsg2Task EvPeHandshakeReq failed, eno: %d, target: %s",
			eno, sch.SchinfGetTaskName(peInst.ptnMe))
		return PeMgrEnoScheduler
	}

	//
	// Map the instance, notice that we do not konw the node identity yet since
	// this is an inbound connection just accepted at this moment.
	//
	peMgr.peers[peInst.ptnMe] = peInst

	//
	// Check if the accept task needs to be paused
	//
	if peMgr.ibpNum += 1;  peMgr.ibpNum >= peMgr.cfg.maxInBounds {

		yclog.LogCallerFileLine("peMgrLsnConnAcceptedInd: " +
			"maxInbounds reached, try to pause accept task ...")

		peMgr.acceptPaused = PauseAccept()

		yclog.LogCallerFileLine("peMgrLsnConnAcceptedInd: " +
			"pause result: %d", peMgr.acceptPaused);
	}

	return PeMgrEnoNone
}

//
// Outbound request handler
//
func peMgrOutboundReq(msg interface{}) PeMgrErrno {

	//
	// Notice: the event sch.EvPeOutboundReq, which is designed to drive the manager
	// to carry out the outbound action, when received, the manager should do its best
	// to start as many as possible outbound tasks, if the possible nodes are not
	// enougth, it then ask the discover task to find more.
	//
	// When event sch.EvPeMgrStartReq received, the manager should send itself a message
	// with event sch.EvPeOutboundReq, and while some other events recevied, the manager
	// should also send itself event sch.EvPeOutboundReq too.
	//

	_ = msg

	// Check workers number
	if peMgr.wrkNum >= peMgr.cfg.maxPeers {
		yclog.LogCallerFileLine("peMgrOutboundReq: it's good, peers full")
		return PeMgrEnoNone
	}

	// Check outbounds number
	if peMgr.obpNum >= peMgr.cfg.maxOutbounds {
		yclog.LogCallerFileLine("peMgrOutboundReq: it's good, outbounds full")
		return PeMgrEnoNone
	}

	// Collect all possible candidates
	var candidates []*ycfg.Node
	var count = 0

	for _, n := range peMgr.cfg.statics {
		if _, ok := peMgr.nodes[n.ID]; !ok {
			candidates[count] = n
			count++
		}
	}

	var rdCnt = 0
	for _, n := range peMgr.randoms {
		if _, ok := peMgr.nodes[n.ID]; !ok {
			candidates[count] = n
			count++
		}
		rdCnt++
	}
	peMgr.randoms = peMgr.randoms[rdCnt:]

	yclog.LogCallerFileLine("peMgrOutboundReq: " +
		"total number of candidates: %d", len(candidates))

	// it might be excceeded, truncate candidates if needed
	if len(candidates) > peMgr.cfg.maxOutbounds - peMgr.obpNum {
		candidates = candidates[:peMgr.cfg.maxOutbounds - peMgr.obpNum]
	}

	// create outbound instances for candidates if any
	var failed = 0
	var ok = 0

	for _, n := range candidates {
		if eno := peMgrCreateOutboundInst(n); eno != PeMgrEnoNone {
			yclog.LogCallerFileLine("peMgrOutboundReq: " +
				"create outbound instance failed, eno: %d", eno)
			failed++
			continue
		}
		ok++
	}

	yclog.LogCallerFileLine("peMgrOutboundReq: " +
		"create outbound intance failed: %d, ok: %d",
		failed, ok)

	// if outbounds are not enougth, ask discover to find more
	if peMgr.obpNum < peMgr.cfg.maxOutbounds {
		if eno := peMgrAsk4More(); eno != PeMgrEnoNone {
			yclog.LogCallerFileLine("peMgrOutboundReq: " +
				"ask discover for more nodes failed, eno: %d", eno)
			return eno
		}
	}

	return PeMgrEnoNone
}

//
// Outbound response handler
//
func peMgrConnOutRsp(msg interface{}) PeMgrErrno {

	//
	// This is an event from an instance task of outbound peer, telling the result
	// about action "connect to".
	//

	var rsp = msg.(*msgConnOutRsp)

	// Check result
	if rsp.result != PeMgrEnoNone {

		// failed, kill instance
		yclog.LogCallerFileLine("peMgrConnOutRsp: " +
			"outbound failed, result: %d, node: %s",
			rsp.result, ycfg.P2pNodeId2HexString(rsp.peNode.ID))

		if eno := peMgrKillInst(rsp.ptn, rsp.peNode); eno != PeMgrEnoNone {
			yclog.LogCallerFileLine("")
		}
		return PeMgrEnoNone
	}

	// Send EvPeHandshakeReq to instance
	var schMsg = sch.SchMessage{}
	var eno sch.SchErrno

	eno = sch.SchinfMakeMessage(&schMsg, peMgr.ptnMe, rsp.ptn, sch.EvPeHandshakeReq, nil)
	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrConnOutRsp: " +
			"SchinfMakeMessage failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}

	if eno = sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrConnOutRsp: " +
			"SchinfSendMsg2Task EvPeHandshakeReq failed, eno: %d, target: %s",
			eno, sch.SchinfGetTaskName(rsp.ptn))
		return PeMgrEnoScheduler
	}

	return PeMgrEnoNone
}

//
// Handshake response handler
//
func peMgrHandshakeRsp(msg interface{}) PeMgrErrno {

	//
	// This is an event from an instance task of outbound or inbound peer, telling
	// the result about the handshake procedure between a pair of peers.
	//

	var rsp = msg.(*msgHandshakeRsp)

	// Check result
	if rsp.result != PeMgrEnoNone {

		// failed, kill instance
		yclog.LogCallerFileLine("peMgrHandshakeRsp: " +
			"outbound failed, result: %d, node: %s",
			rsp.result, ycfg.P2pNodeId2HexString(rsp.peNode.ID))

		if eno := peMgrKillInst(rsp.ptn, rsp.peNode); eno != PeMgrEnoNone {
			yclog.LogCallerFileLine("")
		}
		return PeMgrEnoNone
	}

	// Send EvPeEstablishedInd to instance
	var schMsg = sch.SchMessage{}
	var eno sch.SchErrno

	eno = sch.SchinfMakeMessage(&schMsg, peMgr.ptnMe, rsp.ptn, sch.EvPeEstablishedInd, nil)
	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrHandshakeRsp: " +
			"SchinfMakeMessage failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}

	if eno = sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrHandshakeRsp: " +
			"SchinfSendMsg2Task EvPeHandshakeReq failed, eno: %d, target: %s",
			eno, sch.SchinfGetTaskName(rsp.ptn))
		return PeMgrEnoScheduler
	}

	// Map the instance, notice that, only in this moment we can know the node
	// identity for a inbound connection.
	var inst *peerInstance = peMgr.peers[rsp.ptn]
	peMgr.workers[rsp.peNode.ID] = inst
	peMgr.wrkNum++
	if inst.dir == PeInstDirInbound {
		peMgr.nodes[inst.node.ID] = inst
	}

	return PeMgrEnoNone
}

//
// Pingpong response handler
//
func peMgrPingpongRsp(msg interface{}) PeMgrErrno {

	//
	// This is an event from an instance task of outbound or inbound peer, telling
	// the result about pingpong procedure between a pair of peers.
	//

	var rsp = msg.(*msgPingpongRsp)

	// Check result
	if rsp.result != PeMgrEnoNone {

		// failed, kill instance
		yclog.LogCallerFileLine("peMgrPingpongRsp: " +
			"outbound failed, result: %d, node: %s",
			rsp.result, ycfg.P2pNodeId2HexString(rsp.peNode.ID))

		if eno := peMgrKillInst(rsp.ptn, rsp.peNode); eno != PeMgrEnoNone {
			yclog.LogCallerFileLine("peMgrPingpongRsp: " +
				"kill instance failed, inst: %s, node: %s",
				sch.SchinfGetTaskName(rsp.ptn),
				ycfg.P2pNodeId2HexString(rsp.peNode.ID)	)
		}
	}

	return PeMgrEnoNone
}

//
// Event request to close peer handler
//
func peMgrCloseReq(msg interface{}) PeMgrErrno {

	//
	// This is an event from other module requests to close a peer connection,
	// the peer to be closed should be included in the message passed in.
	//

	var req = msg.(*sch.MsgPeCloseReq)

	// Send close-request to instance
	var schMsg = sch.SchMessage{}
	var eno sch.SchErrno

	eno = sch.SchinfMakeMessage(&schMsg, peMgr.ptnMe, req.Ptn, sch.EvPeCloseReq, &req)
	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrCloseReq: " +
			"SchinfMakeMessage failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}

	if eno = sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrCloseReq: " +
			"SchinfSendMsg2Task EvPeHandshakeReq failed, eno: %d, target: %s",
			eno, sch.SchinfGetTaskName(req.Ptn))
		return PeMgrEnoScheduler
	}

	return PeMgrEnoNone
}

//
// Peer connection closed confirm handler
//
func peMgrConnCloseCfm(msg interface{}) PeMgrErrno {

	//
	// This is an event from an instance task of outbound or inbound peer whom
	// is required to be closed by the peer manager, confiming that the connection
	// had been closed.
	//

	var eno PeMgrErrno
	var cfm = msg.(*MsgCloseCfm)

	// Do not care the result, kill always
	if cfm.result != PeMgrEnoNone {
		yclog.LogCallerFileLine("peMgrConnCloseCfm, " +
			"result: %d, node: %s",
			cfm.result, ycfg.P2pNodeId2HexString(cfm.peNode.ID)	)
	}

	if eno = peMgrKillInst(cfm.ptn, cfm.peNode); eno != PeMgrEnoNone {
		yclog.LogCallerFileLine("peMgrConnCloseCfm: " +
			"kill instance failed, inst: %s, node: %s",
			sch.SchinfGetTaskName(cfm.ptn),
			ycfg.P2pNodeId2HexString(cfm.peNode.ID)	)
		return PeMgrEnoScheduler
	}

	//
	// since we had lost a peer, we need to drive ourself to startup outbound
	//

	var schEno sch.SchErrno
	var schMsg = sch.SchMessage{}

	schEno = sch.SchinfMakeMessage(&schMsg, peMgr.ptnMe, peMgr.ptnMe, sch.EvPeOutboundReq, nil)
	if schEno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrConnCloseCfm: " +
			"SchinfMakeMessage for EvPeOutboundReq failed, eno: %d",
			schEno)
		return PeMgrEnoScheduler
	}

	schEno = sch.SchinfSendMsg2Task(&schMsg)
	if schEno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrConnCloseCfm: " +
			"SchinfSendMsg2Task for EvPeOutboundReq failed, target: %s",
			sch.SchinfGetTaskName(peMgr.ptnMe))
		return PeMgrEnoScheduler
	}

	return PeMgrEnoNone
}

//
// Peer connection closed indication handler
//
func peMgrConnCloseInd(msg interface{}) PeMgrErrno {

	//
	// This is an event from an instance task of outbound or inbound peer whom
	// is not required to be closed by the peer manager, but the connection had
	// been closed for some other reasons.
	//

	var ind = msg.(*MsgCloseInd)

	// Do not care the result, kill always
	yclog.LogCallerFileLine("peMgrConnCloseInd, " +
		"cause: %d, node: %s",
		ind.cause, ycfg.P2pNodeId2HexString(ind.peNode.ID)	)

	if eno := peMgrKillInst(ind.ptn, ind.peNode); eno != PeMgrEnoNone {
		yclog.LogCallerFileLine("peMgrConnCloseInd: " +
			"kill instance failed, inst: %s, node: %s",
			sch.SchinfGetTaskName(ind.ptn),
			ycfg.P2pNodeId2HexString(ind.peNode.ID)	)
		return PeMgrEnoScheduler
	}

	//
	// since we had lost a peer, we need to drive ourself to startup outbound
	//

	var schEno sch.SchErrno
	var schMsg = sch.SchMessage{}

	schEno = sch.SchinfMakeMessage(&schMsg, peMgr.ptnMe, peMgr.ptnMe, sch.EvPeOutboundReq, nil)
	if schEno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrConnCloseInd: " +
			"SchinfMakeMessage for EvPeOutboundReq failed, eno: %d",
			schEno)
		return PeMgrEnoScheduler
	}

	schEno = sch.SchinfSendMsg2Task(&schMsg)
	if schEno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrConnCloseInd: " +
			"SchinfSendMsg2Task for EvPeOutboundReq failed, target: %s",
			sch.SchinfGetTaskName(peMgr.ptnMe))
		return PeMgrEnoScheduler
	}

	return PeMgrEnoNone
}

//
// Create outbound instance
//
func peMgrCreateOutboundInst(node *ycfg.Node) PeMgrErrno {

	// Create outbound task instance for specific node
	var eno = sch.SchEnoNone
	var ptnInst interface{} = nil

	//
	// Init peer instance control block
	//
	var peInst = new(peerInstance)
	*peInst				= peerInstDefault
	peInst.ptnMgr		= peMgr.ptnMe
	peInst.state		= peInstStateConnOut
	peInst.cto			= peMgr.cfg.defaultCto
	peInst.hto			= peMgr.cfg.defaultHto
	peInst.maxPkgSize	= peMgr.cfg.maxMsgSize
	peInst.dialer		= &net.Dialer{Timeout: peMgr.cfg.defaultCto}
	peInst.conn			= nil
	peInst.laddr		= nil
	peInst.raddr		= nil
	peInst.dir			= PeInstDirOutbound
	peInst.node			= *node

	//
	// Create peer instance task
	//
	var tskDesc  = sch.SchTaskDescription {
		Name:		acceptProcName,
		MbSize:		PeInstMailboxSize,
		Ep:			PeerInstProc,
		Wd:			nil,
		Flag:		sch.SchCreatedGo,
		DieCb:		nil,
		UserDa:		peInst,
	}

	if eno, ptnInst = sch.SchinfCreateTask(&tskDesc);
		eno != sch.SchEnoNone || ptnInst == nil {
		yclog.LogCallerFileLine("peMgrCreateOutboundInst: " +
			"SchinfCreateTask failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}
	peInst.ptnMe = ptnInst

	//
	// Check the map
	//
	if _, dup := peMgr.peers[peInst]; dup {
		yclog.LogCallerFileLine("peMgrCreateOutboundInst: " +
			"impossible duplicated peer instance")
		return PeMgrEnoInternal
	}

	//
	// Send EvPeConnOutReq request to the instance created aboved
	//
	var schMsg = sch.SchMessage{}
	eno = sch.SchinfMakeMessage(&schMsg, peMgr.ptnMe, peInst.ptnMe, sch.EvPeConnOutReq, nil)
	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrCreateOutboundInst: " +
			"SchinfMakeMessage failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}

	if eno = sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrCreateOutboundInst: " +
			"SchinfSendMsg2Task EvPeHandshakeReq failed, eno: %d, target: %s",
			eno, sch.SchinfGetTaskName(peInst.ptnMe))
		return PeMgrEnoScheduler
	}

	//
	// Map the instance
	//
	peMgr.peers[peInst.ptnMe] = peInst
	peMgr.nodes[peInst.node.ID] = peInst
	peMgr.obpNum++

	return PeMgrEnoNone
}

//
// Kill specific instance
//
func peMgrKillInst(ptn interface{}, node *ycfg.Node) PeMgrErrno {

	// Get task node pointer
	if ptn == nil && node == nil {
		yclog.LogCallerFileLine("peMgrKillInst: invalid parameter(s)")
		return PeMgrEnoParameter
	}

	if ptn == nil {
		if ptn = peMgr.nodes[node.ID].ptnMe; ptn == nil {
			yclog.LogCallerFileLine("peMgrKillInst: " +
				"instance not found, node: %s",
				ycfg.P2pNodeId2HexString(node.ID))
			return PeMgrEnoScheduler
		}
	}

	// Get instance data area pointer, and if the connection is not null
	// we close it so the instance would get out event it's blocked in
	// action on its' connection.
	var peInst = peMgr.peers[ptn]
	if peInst.conn != nil {
		peInst.conn.Close()
	}

	// Stop instance task
	if eno := sch.SchinfStopTask(ptn); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrKillInst: " +
			"SchinfStopTask failed, eno: %d, task: %s",
			eno, sch.SchinfGetTaskName(ptn))
		return PeMgrEnoScheduler
	}

	// Remove maps for the node
	if peInst.state == peInstStateActivated {
		delete(peMgr.workers, peInst.node.ID)
		peMgr.wrkNum--
	}

	if peInst.dir == PeInstDirOutbound {
		peMgr.obpNum--
	} else if peInst.dir == PeInstDirInbound {
		peMgr.ibpNum--
	} else {
		yclog.LogCallerFileLine("peMgrKillInst: " +
			"invalid peer instance direction: %d",
			peInst.dir)
	}

	delete(peMgr.nodes, peInst.node.ID)
	delete(peMgr.peers, ptn)

	// Check if the accepter task paused, resume it if necessary
	if peMgr.acceptPaused == true {
		ResumeAccept()
	}

	return PeMgrEnoNone
}

//
// Request the discover task to findout more node for outbound
//
func peMgrAsk4More() PeMgrErrno {

	// Send EvDcvFindNodeReq to discover task. The filters “include" and
	// "exclude" are not applied currently.

	var eno sch.SchErrno
	var schMsg = sch.SchMessage{}
	var req = sch.MsgDcvFindNodeReq {
		Include: nil,
		Exclude: nil,
	}

	eno = sch.SchinfMakeMessage(&schMsg, peMgr.ptnMe, peMgr.ptnDcv, sch.EvDcvFindNodeReq, &req)
	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrAsk4More: " +
			"SchinfMakeMessage failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}

	if eno = sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("peMgrAsk4More: " +
			"SchinfSendMsg2Task EvPeHandshakeReq failed, eno: %d, target: %s",
			eno, sch.SchinfGetTaskName(peMgr.ptnDcv))
		return PeMgrEnoScheduler
	}

	return PeMgrEnoNone
}

//
// Dynamic peer task
//
const peInstTaskName = "peInstTsk"

const (
	peInstStateNull		= iota	// null
	peInstStateConnOut			// outbound connection inited
	peInstStateAccepted			// inbound accepted, need handshake
	peInstStateConnected		// outbound connected, need handshake
	peInstStateHandshook		// handshook
	peInstStateActivated		// actived in working
	peInstStateClosed			// closed
)

type peerInstState int	// instance state type

const PeInstDirNull		= 0		// null, so connection should be nil
const PeInstDirOutbound	= +1	// outbound connection
const PeInstDirInbound	= -1	// inbound connection

const PeInstMailboxSize = 32	// mailbox size

type peerInstance struct {
	name		string				// name
	tep			sch.SchUserTaskEp	// entry
	ptnMe		interface{}			// the instance task node pointer
	ptnMgr		interface{}			// the peer manager task node pointer
	state		peerInstState		// state
	cto			time.Duration		// connect timeout value
	hto			time.Duration		// handshake timeout value
	dialer		*net.Dialer			// dialer to make outbound connection
	conn		net.Conn			// connection
	laddr		*net.TCPAddr		// local ip address
	raddr		*net.TCPAddr		// remote ip address
	dir			int					// direction: outbound(+1) or inbound(-1)
	node		ycfg.Node			// peer "node" information
	maxPkgSize	int					// max size of tcpmsg package
	protoNum	uint32				// peer protocol number
	protocols	[]Protocol			// peer protocol table

}

var peerInstDefault = peerInstance {
	name:	peInstTaskName,
	tep:	PeerInstProc,
	ptnMe:	nil,
	ptnMgr:	nil,
	state:	peInstStateNull,
	cto:	0,
	dialer:	nil,
	conn:	nil,
	laddr:	nil,
	raddr:	nil,
	dir:	PeInstDirNull,
	node:	ycfg.Node{},
}

//
// EvPeConnOutRsp message
//
type msgConnOutRsp struct {
	result	PeMgrErrno	// result of outbound connect action
	peNode 	*ycfg.Node	// target node
	ptn		interface{}	// pointer to task instance node of sender
}

//
// EvPeHandshakeRsp message
//
type msgHandshakeRsp struct {
	result	PeMgrErrno	// result of handshake action
	peNode 	*ycfg.Node	// target node
	ptn		interface{}	// pointer to task instance node of sender
}

//
// EvPePingpongRsp message
//
type msgPingpongRsp struct {
	result	PeMgrErrno	// result of pingpong action
	peNode 	*ycfg.Node	// target node
	ptn		interface{}	// pointer to task instance node of sender
}

//
// EvPeCloseCfm message
//
type MsgCloseCfm struct {
	result	PeMgrErrno	// result of pingpong action
	peNode 	*ycfg.Node	// target node
	ptn		interface{}	// pointer to task instance node of sender
}

//
// EvPeCloseInd message
//
type MsgCloseInd struct {
	cause	PeMgrErrno	// tell why it's closed
	peNode 	*ycfg.Node	// target node
	ptn		interface{}	// pointer to task instance node of sender
}

//
// Peer instance entry
//
func PeerInstProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	yclog.LogCallerFileLine("PeerInstProc: scheduled, msg: %d", msg.Id)

	var eno PeMgrErrno


	inst := sch.SchinfGetUserDataArea(ptn).(*peerInstance)

	switch msg.Id {
	case sch.EvPeConnOutReq:
		eno = piConnOutReq(inst, msg.Body)
	case sch.EvPeHandshakeReq:
		eno = piHandshakeReq(inst, msg.Body)
	case sch.EvPePingpongReq:
		eno = piPingpongReq(inst, msg.Body)
	case sch.EvPeCloseReq:
		eno = piCloseReq(inst, msg.Body)
	case sch.EvPeEstablishedInd:
		eno = piEstablishedInd(inst, msg.Body)
	case sch.EvPePingpongTimer:
		eno = piPingpongTimerHandler(inst)
	case sch.EvPeDataReq:
		eno = piDataReq(inst, msg.Body)
	default:
		yclog.LogCallerFileLine("PeerInstProc: invalid message: %d", msg.Id)
		eno = PeMgrEnoParameter
	}

	if eno != PeMgrEnoNone {
		yclog.LogCallerFileLine("PeerInstProc: instance errors, eno: %d", eno)
		return sch.SchEnoUserTask
	}

	return sch.SchEnoNone
}

//
// Outbound connect to peer request handler
//
func piConnOutReq(inst *peerInstance, msg interface{}) PeMgrErrno {

	_ = msg

	//
	// Check instance
	//

	if inst == nil {
		yclog.LogCallerFileLine("piConnOutReq: invalid instance")
		return PeMgrEnoParameter
	}

	if inst.dialer == nil ||
		inst.dir != PeInstDirOutbound  ||
		inst.state != peInstStateConnOut {
		yclog.LogCallerFileLine("piConnOutReq: instance mismatched")
		return PeMgrEnoInternal
	}

	//
	// Dial to peer node
	//

	var addr = &net.TCPAddr{IP: inst.node.IP, Port: int(inst.node.TCP)}
	var conn net.Conn = nil
	var err error
	var eno PeMgrErrno = PeMgrEnoNone

	if conn, err = inst.dialer.Dial("tcp", addr.String()); err != nil {
		yclog.LogCallerFileLine("piConnOutReq: " +
			"dial failed, to: %s, err: %s",
			addr.String(), err.Error())
		eno = PeMgrEnoOs
	} else {
		// Backup connection and update instance state
		yclog.LogCallerFileLine("piConnOutReq: " +
			"dial ok, to: %s, err: %s",
			addr.String(), err.Error())
		inst.conn = conn
		inst.laddr = conn.LocalAddr().(*net.TCPAddr)
		inst.raddr = conn.RemoteAddr().(*net.TCPAddr)
		inst.state = peInstStateConnected
	}

	//
	// Response to peer manager task
	//

	var schEno sch.SchErrno
	var schMsg = sch.SchMessage{}
	var rsp = msgConnOutRsp {
		result:eno,
		peNode:&inst.node,
		ptn: inst.ptnMe,
	}

	schEno = sch.SchinfMakeMessage(&schMsg, inst.ptnMe, inst.ptnMgr, sch.EvPeConnOutRsp, &rsp)
	if schEno != sch.SchEnoNone {
		yclog.LogCallerFileLine("piConnOutReq: " +
			"SchinfMakeMessage failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}

	if schEno = sch.SchinfSendMsg2Task(&schMsg); schEno != sch.SchEnoNone {
		yclog.LogCallerFileLine("piConnOutReq: " +
			"SchinfSendMsg2Task EvPeConnOutRsp failed, eno: %d, target: %s",
			schEno, sch.SchinfGetTaskName(inst.ptnMgr))
		return PeMgrEnoScheduler
	}

	return PeMgrEnoNone
}

//
// Handshake request handler
//
func piHandshakeReq(inst *peerInstance, msg interface{}) PeMgrErrno {

	_ = msg

	//
	// Check instance
	//

	if inst == nil {
		yclog.LogCallerFileLine("piHandshakeReq: invalid instance")
		return PeMgrEnoParameter
	}

	if inst.state != peInstStateConnected && inst.state != peInstStateAccepted {
		yclog.LogCallerFileLine("piHandshakeReq: instance mismatched")
		return PeMgrEnoInternal
	}

	if inst.conn == nil {
		yclog.LogCallerFileLine("piHandshakeReq: invalid instance")
		return PeMgrEnoInternal
	}

	//
	// Carry out action according to the direction of current peer instance
	// connection.
	//

	var eno PeMgrErrno

	if inst.dir == PeInstDirInbound {
		eno = piHandshakeInbound(inst)
	} else if inst.dir == PeInstDirOutbound {
		eno = piHandshakeOutbound(inst)
	} else {
		yclog.LogCallerFileLine("piHandshakeReq: " +
			"invalid instance direction: %d",
			inst.dir)
		eno = PeMgrEnoInternal
	}

	if eno != PeMgrEnoNone {
		yclog.LogCallerFileLine("piHandshakeReq: " +
			"handshake fialed, dir: %d, eno: %d",
			inst.dir, eno)
	}

	//
	// response to peer manager
	//

	var schEno sch.SchErrno
	var schMsg = sch.SchMessage{}
	var rsp = msgHandshakeRsp {
		result:eno,
		peNode:&inst.node,
		ptn: inst.ptnMe,
	}

	schEno = sch.SchinfMakeMessage(&schMsg, inst.ptnMe, inst.ptnMgr, sch.EvPeHandshakeRsp, &rsp)
	if schEno != sch.SchEnoNone {
		yclog.LogCallerFileLine("piHandshakeReq: " +
			"SchinfMakeMessage failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}

	if schEno = sch.SchinfSendMsg2Task(&schMsg); schEno != sch.SchEnoNone {
		yclog.LogCallerFileLine("piHandshakeReq: " +
			"SchinfSendMsg2Task EvPeConnOutRsp failed, eno: %d, target: %s",
			schEno, sch.SchinfGetTaskName(inst.ptnMgr))
		return PeMgrEnoScheduler
	}

	return eno
}

//
// Ping-Request handler
//
func piPingpongReq(inst *peerInstance, msg interface{}) PeMgrErrno {

	//
	// The ping procedure is inted by a timer internal the peer task
	// instance, and seems no need to init this kind of procedure outside
	// the peer instance.
	//

	yclog.LogCallerFileLine("piPingpongReq: not supported")
	return PeMgrEnoUnsup
}

//
// Close-Request handler
//
func piCloseReq(inst *peerInstance, msg interface{}) PeMgrErrno {
	return PeMgrEnoNone
}

//
// Peer-Established indication handler
//
func piEstablishedInd(inst *peerInstance, msg interface{}) PeMgrErrno {
	return PeMgrEnoNone
}

//
// Pingpong timer handler
//
func piPingpongTimerHandler(inst *peerInstance) PeMgrErrno {
	return PeMgrEnoNone
}

//
// Data-Request(send data) handler
//
func piDataReq(inst *peerInstance, msg interface{}) PeMgrErrno {
	return PeMgrEnoNone
}

//
// Handshake for inbound
//
func piHandshakeInbound(inst *peerInstance) PeMgrErrno {

	var eno PeMgrErrno = PeMgrEnoNone
	var pkg = new(TcpmsgPackage)
	var hs *TcpmsgHandshake

	// read inbound handshake from remote peer
	if hs, eno = pkg.getHandshakeInbound(inst); hs == nil || eno != PeMgrEnoNone {
		yclog.LogCallerFileLine("piHandshakeInbound: " +
			"read inbound Handshake message failed, eno: %d",
			eno)
		return eno
	}

	// backup info about protocols supported by peer
	inst.protoNum = hs.ProtoNum
	inst.protocols = hs.Protocols
	inst.node.ID = hs.NodeId

	// write outbound handshake to remote peer
	hs.NodeId = peMgr.cfg.nodeId
	hs.ProtoNum = peMgr.cfg.protoNum
	hs.Protocols = peMgr.cfg.protocols

	if eno = pkg.putHandshakeOutbound(inst, hs); eno != PeMgrEnoNone {
		yclog.LogCallerFileLine("piHandshakeInbound: " +
			"write outbound Handshake message failed, eno: %d",
			eno)
		return eno
	}

	// update instance state
	inst.state = peInstStateHandshook

	return PeMgrEnoNone
}

//
// Handshake for outbound
//
func piHandshakeOutbound(inst *peerInstance) PeMgrErrno {

	var eno PeMgrErrno = PeMgrEnoNone
	var pkg = new(TcpmsgPackage)
	var hs = new(TcpmsgHandshake)

	// write outbound handshake to remote peer
	hs.NodeId = peMgr.cfg.nodeId
	hs.ProtoNum = peMgr.cfg.protoNum
	hs.Protocols = peMgr.cfg.protocols

	if eno = pkg.putHandshakeOutbound(inst, hs); eno != PeMgrEnoNone {
		yclog.LogCallerFileLine("piHandshakeOutbound: " +
			"write outbound Handshake message failed, eno: %d",
			eno)
		return eno
	}

	// read inbound handshake from remote peer
	if hs, eno = pkg.getHandshakeInbound(inst); hs == nil || eno != PeMgrEnoNone {
		yclog.LogCallerFileLine("piHandshakeOutbound: " +
			"read inbound Handshake message failed, eno: %d",
			eno)
		return eno
	}

	// since it's an outbound peer, the peer node id is known before this
	// handshake procedure carried out, we can check against these twos.
	if hs.NodeId != inst.node.ID {
		yclog.LogCallerFileLine("piHandshakeOutbound: node identity mismathced")
		return PeMgrEnoMessage
	}

	// backup info about protocols supported by peer;
	// update instance state;
	inst.protoNum = hs.ProtoNum
	inst.protocols = hs.Protocols
	inst.state = peInstStateHandshook

	return PeMgrEnoNone
}
