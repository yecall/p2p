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
	PeMgrEnoInternal
	PeMgrEnoUnknown
)

type PeMgrErrno int

//
// Peer identity as string
//
type PeerId string

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
type peMgrConfig struct {
	maxPeers		int			// max peers would be
	maxOutbounds	int			// max concurrency outbounds
	maxInBounds		int			// max concurrency inbounds
	ip				net.IP		// ip address
	port			uint16		// port numbers
	nodeId			ycfg.NodeID	// the node's public key
}

//
// Peer manager
//
const PeerMgrName = sch.PeerMgrName

type peerManager struct {
	name			string							// name
	tep				sch.SchUserTaskEp				// entry
	ptnMe			interface{}						// pointer to myself(peer manager task node)
	ptnTab			interface{}						// pointer to table task node
	ptnLsn			interface{}						// pointer to peer listener manager task node
	ptnAcp			interface{}						// pointer to peer acceptor manager task node
	peers			map[interface{}]*peerInstance	// map peer instance's task node pointer to instance pointer
	cfg				peMgrConfig						// configuration
	ibpNum			int								// active inbound peer number
	obpNum			int								// active outbound peer number
	acceptPaused	bool							// if accept task paused
}

var peMgr = peerManager{
	name:	PeerMgrName,
	tep:	PeerMgrProc,
	ptnMe:	nil,
	ptnTab:	nil,
	ptnLsn:	nil,
	ptnAcp:	nil,
	peers:	map[interface{}]*peerInstance{},
	cfg:	nil,
	ibpNum:	0,
	obpNum:	0,
	acceptPaused: false,
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

	// fetch configration
	var cfg *ycfg.Cfg4PeerManager = nil
	if cfg := ycfg.P2pConfig4PeerManager(); cfg == nil {
		yclog.LogCallerFileLine("peMgrPoweron: P2pConfig4PeerManager failed")
		return PeMgrEnoConfig
	}
	peMgr.cfg = peMgrConfig{
		maxPeers:		cfg.MaxPeers,
		maxOutbounds:	cfg.MaxOutbounds,
		maxInBounds:	cfg.MaxInBounds,
		ip:				cfg.IP,
		port:			cfg.Port,
		nodeId:			cfg.ID,
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

	// drive ourself to startup
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
	*peInst			= peerInstDefault
	peInst.ptnMgr	= peMgr.ptnMe
	peInst.state	= peInstStateAccepted
	peInst.conn		= ibInd.conn
	peInst.laddr	= ibInd.localAddr
	peInst.raddr	= ibInd.remoteAddr
	peInst.dir		= PeInstDirInbound

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
		yclog.LogCallerFileLine("SchinfMakeMessage: " +
			"SchinfCreateTask failed, eno: %d",
			eno)
		return PeMgrEnoScheduler
	}

	if eno = sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("SchinfMakeMessage: " +
			"SchinfSendMsg2Task EvPeHandshakeReq failed, eno: %d, target: %s",
			eno, sch.SchinfGetTaskName(peInst.ptnMe))
		return PeMgrEnoScheduler
	}

	//
	// Map the instance
	//
	peMgr.peers[peInst.ptnMe] = peInst

	//
	// Check if the accept task needs to be paused
	//
	if peMgr.ibpNum += 1;  peMgr.ibpNum >= peMgr.cfg.maxInBounds {

		yclog.LogCallerFileLine("SchinfMakeMessage: maxInbounds reached, try to pause accept task ...")

		peMgr.acceptPaused = PauseAccept()

		yclog.LogCallerFileLine("SchinfMakeMessage: pause result: %d", peMgr.acceptPaused);
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

	return PeMgrEnoNone
}

//
// Dynamic peer task
//
const peInstTaskName = "peInstTsk"

const (
	peInstStateNull		= iota	// null
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
	name	string				// name
	tep		sch.SchUserTaskEp	// entry
	ptnMe	interface{}			// the instance task node pointer
	ptnMgr	interface{}			// the peer manager task node pointer
	state	peerInstState		// state
	conn	net.Conn			// connection
	laddr	*net.IPAddr			// local ip address
	raddr	*net.IPAddr			// remote ip address
	dir		int					// direction: outbound(+1) or inbound(-1)
	node	ycfg.Node			// peer "node" information
}

var peerInstDefault = peerInstance {
	name:	peInstTaskName,
	tep:	PeerInstProc,
	ptnMe:	nil,
	ptnMgr:	nil,
	state:	peInstStateNull,
	conn:	nil,
	laddr:	nil,
	raddr:	nil,
	dir:	PeInstDirNull,
	node:	ycfg.Node{},
}

//
// Peer instance entry
//
func PeerInstProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	yclog.LogCallerFileLine("PeerInstProc: scheduled, msg: %d", msg.Id)

	switch msg.Id {
	case sch.EvPeConnOutReq:
	case sch.EvPeHandshakeReq:
	case sch.EvPePingpongReq:
	case sch.EvPeCloseReq:
	case sch.EvPeEstablishedInd:
	default:
	}

	return sch.SchEnoNone
}
