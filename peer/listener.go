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
	"fmt"
	ycfg	"ycp2p/config"
	sch		"ycp2p/scheduler"
	yclog	"ycp2p/logger"
	"sync"
)

//
// Listener manager
//
const PeerLsnMgrName = sch.PeerLsnMgrName

type listenerManager struct {
	name		string					// name
	tep			sch.SchUserTaskEp		// entry
	ptn			interface{}				// the listner task node pointer
	ptnPeerMgr	interface{}				// the peer manager task node pointer
	cfg			*ycfg.Cfg4PeerListener	// configuration
	listener	net.Listener			// listener of net
	listenAddr	*net.TCPAddr			// listen address
}

var lsnMgr = listenerManager{
	name:	PeerLsnMgrName,
	tep:	nil,
}


//
// To ecape the compiler "initialization loop" error
//
func init() {
	lsnMgr.tep = LsnMgrProc
}


//
// Listen manager entry
//
func LsnMgrProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	yclog.LogCallerFileLine("LsnMgrProc: scheduled, msg: %d", msg.Id)

	var eno sch.SchErrno

	switch msg.Id {
	case sch.EvSchPoweron:
		eno = lsnMgrPoweron(ptn)

	case sch.EvSchPoweroff:
		eno = lsnMgrPoweroff()

	case sch.EvPeLsnStartReq:
		eno = lsnMgrStart()

	case sch.EvPeLsnStopReq:
		eno = lsnMgrStop()

	default:
		yclog.LogCallerFileLine("LsnMgrProc: invalid message: %d", msg.Id)
		eno = sch.SchEnoParameter
	}

	if eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("LsnMgrProc: errors, eno: %d", eno)
	}

	return eno
}

//
// Poweron event handler
//
func lsnMgrPoweron(ptn interface{}) sch.SchErrno {

	yclog.LogCallerFileLine("lsnMgrPoweron: poweron, carry out task initilization")

	// Keep ourself task node pointer;
	// Get peer mamager task node pointer;
	lsnMgr.ptn = ptn
	_, lsnMgr.ptnPeerMgr = sch.SchinfGetTaskNodeByName(PeerMgrName)
	if lsnMgr.ptnPeerMgr == nil {
		yclog.LogCallerFileLine("lsnMgrPoweron: invalid peer manager task node pointer")
		return sch.SchEnoInternal
	}

	// Get configuration
	lsnMgr.cfg = ycfg.P2pConfig4PeerListener()
	if lsnMgr.cfg == nil {
		yclog.LogCallerFileLine("lsnMgrPoweron: invalid configuration pointer")
		return sch.SchEnoConfig
	}

	// Setup net lsitener
	var err error
	lsnAddr := fmt.Sprintf("%s:%d", lsnMgr.cfg.IP.String(), lsnMgr.cfg.Port)
	if lsnMgr.listener, err = net.Listen("tcp", lsnAddr); err != nil {
		yclog.LogCallerFileLine("lsnMgrPoweron: " +
			"listen failed, addr: %s, err: %s",
			lsnAddr, err.Error())
		return sch.SchEnoOS
	}
	lsnMgr.listenAddr = lsnMgr.listener.Addr().(*net.TCPAddr)

	yclog.LogCallerFileLine("lsnMgrPoweron: task inited ok")
	return sch.SchEnoNone
}

//
// Poweroff event handler
//
func lsnMgrPoweroff() sch.SchErrno {

	yclog.LogCallerFileLine("lsnMgrPoweroff: poweroff, done")

	// kill accepter task if needed
	if _, ptn := sch.SchinfGetTaskNodeByName(acceptProcName); ptn != nil {
		lsnMgrStop()
	}

	return sch.SchinfTaskDone(lsnMgr.ptn, sch.SchEnoKilled)
}

//
// Startup event handler
//
func lsnMgrStart() sch.SchErrno {

	//
	// When startup signal rceived, we create task which would go into
	// a longlong loop to accept possible inbound connection. Notice that
	// this task would have no chance to receive any messages scheduled
	// to it since it's in a dead loop. To bring this task out, a stop
	// request must be sent to the manager(ourself), which would then try
	// to close the listener, so the task would get out.
	//

	yclog.LogCallerFileLine("lsnMgrStop: try to create accept task ...")

	var tskDesc = sch.SchTaskDescription{
		Name:		acceptProcName,
		MbSize:		0,
		Ep:			acceptProc,
		Wd:			nil,
		Flag:		sch.SchCreatedGo,
		DieCb:		nil,
		UserDa:		nil,
	}

	if eno, ptn := sch.SchinfCreateTask(&tskDesc); eno != sch.SchEnoNone || ptn == nil {
		yclog.LogCallerFileLine("lsnMgrStart: " +
			"SchinfCreateTask failed, eno: %d, ptn: %x",
			eno, ptn.(*interface{}))
		if ptn == nil {
			return eno
		}
		return sch.SchEnoInternal
	}

	yclog.LogCallerFileLine("lsnMgrStop: accept task created")
	return sch.SchEnoNone
}

//
// Stop event handler
//
func lsnMgrStop() sch.SchErrno {

	yclog.LogCallerFileLine("lsnMgrStop: listner will be closed")

	acceptTCB.lockTcb.Lock()
	defer acceptTCB.lockTcb.Unlock()

	// Close the listener to force the acceptor task out of the loop,
	// see function acceptProc for details please.
	acceptTCB.event = sch.SchEnoKilled
	if err := lsnMgr.listener.Close(); err != nil {
		yclog.LogCallerFileLine("lsnMgrStop: try to close listner fialed, err: %s", err.Error())
		return sch.SchEnoOS
	}

	yclog.LogCallerFileLine("lsnMgrStop: listner closed ok")
	lsnMgr.listener = nil

	return sch.SchEnoNone
}

//
// Accept task
//
const acceptProcName = "peerAcceptProc"

type acceptTskCtrlBlock struct {
	ptnPeMgr	interface{}		// pointer to peer manager task node
	ptnLsnMgr	interface{}		// pointer to listener manager task node
	listener	net.Listener	// the listener
	event		sch.SchErrno	// event fired
	curError	error			// current error fired
	lockTcb		sync.Locker		// lock to protect this control block
	lockAccept	sync.Locker		// lock to pause/resume acception
}

var acceptTCB = acceptTskCtrlBlock {
	ptnLsnMgr:	nil,
	listener:	nil,
	event:		sch.EvSchNull,
	curError:	nil,
}

//
// message for sch.EvPeLsnConnAcceptedInd
//
type msgConnAcceptedInd struct {
	conn		net.Conn
	localAddr	*net.TCPAddr
	remoteAddr	*net.TCPAddr
}

//
// Accept task entry
//
func acceptProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	_ = msg

	//
	// Go into a longlong loop to accept peer connections. Please see
	// comments in function lsnMgrStart for more.
	//

	_, acceptTCB.ptnLsnMgr = sch.SchinfGetTaskNodeByName(PeerLsnMgrName)
	if acceptTCB.ptnPeMgr == nil || acceptTCB.ptnLsnMgr == nil {
		yclog.LogCallerFileLine("acceptProc: invalid listener manager task pointer")
		sch.SchinfTaskDone(ptn, sch.SchEnoInternal)
		return sch.SchEnoInternal
	}

	_, acceptTCB.ptnPeMgr = sch.SchinfGetTaskNodeByName(PeerMgrName)
	if acceptTCB.ptnPeMgr == nil || acceptTCB.ptnLsnMgr == nil {
		yclog.LogCallerFileLine("acceptProc: invalid peer manager task pointer")
		sch.SchinfTaskDone(ptn, sch.SchEnoInternal)
		return sch.SchEnoInternal
	}

	acceptTCB.listener = lsnMgr.listener
	if acceptTCB.listener == nil {
		yclog.LogCallerFileLine("acceptProc: invalid listener")
		sch.SchinfTaskDone(ptn, sch.SchEnoInternal)
	}
	acceptTCB.event = sch.EvSchNull
	acceptTCB.curError = nil

	yclog.LogCallerFileLine("acceptProc: inited ok, tring to accept ...")

acceptLoop:

	for {

		// Check if had been kill by manager
		if acceptTCB.listener == nil {
			yclog.LogCallerFileLine("acceptProc: broken for nil listener, we might have been killed")
			break acceptLoop
		}

		// Get lock to accept: unlock it at once since we just want to know if we
		// are allowed to accept.
		acceptTCB.lockAccept.Lock()
		acceptTCB.lockAccept.Unlock()

		// Try to accept: can we be blocked in Accpet()?
		// provide it would, so the listen manager might have to close the
		// listener to get this task out.
		conn, err := acceptTCB.listener.Accept()

		// Lock the control block since following statements need to access it
		acceptTCB.lockTcb.Lock()

		// Check errors
		if err != nil && !err.(net.Error).Temporary() {
			yclog.LogCallerFileLine("acceptProc: " +
				"break loop for non-temporary error while accepting, err: %s", err.Error())
			acceptTCB.curError = err
			acceptTCB.lockTcb.Unlock()
			break acceptLoop
		}

		// Check connection accepted
		if conn == nil {
			yclog.LogCallerFileLine("acceptProc: " +
				"break loop for null connection accepted without errors")
			acceptTCB.event = sch.EvSchException
			acceptTCB.lockTcb.Unlock()
			break acceptLoop
		}

		// Connection got, hand it up to peer manager task
		var msg = sch.SchMessage{}
		var msgBody = msgConnAcceptedInd {
			conn: 		conn,
			localAddr:	conn.LocalAddr().(*net.TCPAddr),
			remoteAddr:	conn.RemoteAddr().(*net.TCPAddr),
		}

		eno := sch.SchinfMakeMessage(&msg, ptn, acceptTCB.ptnPeMgr, sch.EvPeLsnConnAcceptedInd, &msgBody)
		if eno != sch.SchEnoNone {
			yclog.LogCallerFileLine("acceptProc: " +
				"SchinfMakeMessage for EvPeLsnConnAcceptedInd failed, eno: %d",
				eno)
			acceptTCB.lockTcb.Unlock()
			continue
		}

		eno = sch.SchinfSendMsg2Task(&msg)
		if eno != sch.SchEnoNone {
			yclog.LogCallerFileLine("acceptProc: " +
				"SchinfSendMsg2Task for EvPeLsnConnAcceptedInd failed, target: %s",
				sch.SchinfGetTaskName(acceptTCB.ptnPeMgr))
			acceptTCB.lockTcb.Unlock()
			continue
		}
	}

	// Lock the control block since following statements need to access it
	acceptTCB.lockTcb.Lock()

	//
	// Here we get out! We should check what had happened to break the loop
	// for accepting aboved.
	//

	if acceptTCB.curError != nil && acceptTCB.event != sch.EvSchNull {

		//
		// This is the normal case: the loop is broken by manager task, or
		// errors fired from underlaying network.
		//

		yclog.LogCallerFileLine("acceptProc: broken for event: %d", acceptTCB.event)
		sch.SchinfTaskDone(ptn, acceptTCB.event)
		acceptTCB.lockTcb.Unlock()
		return acceptTCB.event
	}

	//
	// Abnormal case, we should never come here
	//

	if acceptTCB.curError != nil {
		yclog.LogCallerFileLine("acceptProc: abnormal exit, event: %d, err: %s",
			acceptTCB.event, acceptTCB.curError.Error())
	} else {
		yclog.LogCallerFileLine("acceptProc: abnormal exit, event: %d, err: nil",
			acceptTCB.event)
	}

	acceptTCB.lockTcb.Unlock()
	sch.SchinfTaskDone(ptn, sch.SchEnoUnknown)
	return sch.SchEnoUnknown
}

//
// Pause accept
//
func PauseAccept() bool {
	acceptTCB.lockAccept.Lock()
	return true
}

//
// Resume accept
//
func ResumeAccept() bool {
	acceptTCB.lockAccept.Unlock()
	return true
}
