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


package scheduler

//
// Null event: nothing;
// Poweron: scheduler just started;
// Poweroff: scheduler will be stopped.
//
const (
	EvSchNull		= 0
	EvSchPoweron	= EvSchNull
	EvSchPoweroff	= EvSchNull + 1
)

//
// Scheduler internal event
//
const (
	EvSchBase			= 10
	EvSchTaskCreated	= EvSchBase + 1
)


//
// Timer event: for an user task, it could hold most timer number as schMaxTaskTimer,
// and then, when timer n which in [0,schMaxTaskTimer-1] is expired, message with event
// id as n would be sent to user task, means schMessage.id would be set to n.
//
const EvTimerBase = 1000

//
// Shell event
//
const EvShellBase = 1100

//
// Table manager event
//
const TabRefreshTimerId = 0
const (
	EvTabMgrBase 		= 1200
	EvTabRefreshTimer	= EvTimerBase	+ TabRefreshTimerId
	EvTabRefreshReq		= EvTabMgrBase	+ 1
	EvTabRefreshRsp		= EvTabMgrBase	+ 2
)

//
// Discover manager event
//
const (
	EvDcvMgrBase		= 1300
	EvDcvFindNodeReq	= EvDcvMgrBase	+ 1
	EvDcvFindNodeRsp	= EvDcvMgrBase	+ 2
)

//
// Neighbor lookup on Udp event
//
const NblFindNodeTimerId	= 0
const NblPingpongTimerId	= 1
const (
	EvNblUdpBase		= 1400
	EvNblFindNodeTimer	= EvTimerBase	+ NblFindNodeTimerId
	EvNblPingpongTimer	= EvTimerBase	+ NblPingpongTimerId
	EvNblFindNodeReq	= EvNblUdpBase	+ 1
	EvNblFindNodeRsp	= EvNblUdpBase	+ 2
	EvNblPingpongReq	= EvNblUdpBase	+ 3
	EvNblPingpongRsp	= EvNblUdpBase	+ 4
)

//
// Neighbor listenner event
//
const (
	EvNblListennerBase	= 1500
	EvNblMsgInd			= EvNblListennerBase + 1
	EvNblStop			= EvNblListennerBase + 2
	EvNblStart			= EvNblListennerBase + 3
)

//
// Peer manager event
//
const (
	EvPeerMgrBase = 1600
)


//
// Peer listerner event
//
const (
	EvPeerLsnBase = 1700
)

//
// Peer connection establishment event
//
const (
	EvPeerEstBase		= 1800
	EvPeConnOutReq		= EvPeerEstBase	+ 1
	EvPeConnOutRsp		= EvPeerEstBase	+ 2
	EvPeHandshakeReq	= EvPeerEstBase	+ 3
	EvPeHandshakeRsp	= EvPeerEstBase	+ 4
	EvPePingpongReq		= EvPeerEstBase	+ 5
	EvPePingpongRsp		= EvPeerEstBase	+ 6
	EvPeCloseReq		= EvPeerEstBase	+ 7
	EvPeCloseCfm		= EvPeerEstBase	+ 8
	EvPeCloseInd		= EvPeerEstBase + 9
)

//
// DHT manager event
//
const EvDhtMgrBase = 1900

//
// DHT peer lookup on Tcp event
//
const EvDhtPeerLkBase = 2000

//
// DHT provider event
//
const EvDhtPrdBase = 2100
