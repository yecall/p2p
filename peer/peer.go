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
	sch 	"ycp2p/scheduler"
	yclog	"ycp2p/logger"
)

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
// Peer manager
//
const PeerMgrName = sch.PeerMgrName

type peerManager struct {
	name	string				// name
	tep		sch.SchUserTaskEp	// entry
}

var dcvMgr = peerManager{
	name:	PeerMgrName,
	tep:	PeerMgrProc,
}

//
// Peer manager entry
//
func PeerMgrProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	yclog.LogCallerFileLine("PeerMgrProc: scheduled, msg: %d", msg.Id)

	switch msg.Id {
	case sch.EvSchPoweron:
	case sch.EvSchPoweroff:
	case sch.EvDcvFindNodeRsp:
	case sch.EvPeerConnAcceptedInd:
	case sch.EvPeOutboundReq:
	case sch.EvPeConnOutRsp:
	case sch.EvPeHandshakeRsp:
	case sch.EvPePingpongRsp:
	case sch.EvPeCloseCfm:
	case sch.EvPeCloseInd:
	default:
	}

	return sch.SchEnoNone
}

//
// Dynamic peer task
//
type peerInstState int

type peerInstance struct {
	name	string				// name
	tep		sch.SchUserTaskEp	// entry
	state	peerInstState		// state
	conn	net.Conn			// connection
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
