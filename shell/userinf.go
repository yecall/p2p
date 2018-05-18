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


package shell

import (
	"fmt"
	"sync"
	"ycp2p/peer"
	yclog "ycp2p/logger"
)


//
// errno about this interface
//
type P2pInfErrno	int

const (
	P2pInfEnoNone		P2pInfErrno = 0	// none of errors
	P2pInfEnoParameter	P2pInfErrno = 1	// invalid parameters
	P2pInfEnoScheduler	P2pInfErrno	= 2	// shceduler
	P2pInfEnoNotImpl	P2pInfErrno = 3	// not implemented
	P2pInfEnoInternal	P2pInfErrno	= 4	// internal
	P2pInfEnoUnknown	P2pInfErrno = 5	// unknown
	P2pInfEnoMax		P2pInfErrno = 6	// max, for bound checking
)

//
// Description about user interface errno
//
var P2pInfErrnoDescription = []string {
	"none of errors",
	"invalid parameters",
	"unknown",
	"max value can errno be",
}

//
// Stringz an errno with itself
//
func (eno P2pInfErrno) P2pInfErrnoString() string {
	if eno < P2pInfEnoNone || eno >= P2pInfEnoMax {
		return fmt.Sprintf("Can't be stringzed, invalid eno:%d", eno)
	}
	return P2pInfErrnoDescription[eno]
}

//
// Stringz an errno with an eno parameter
//
func P2pInfErrnoString(eno P2pInfErrno) string {
	return eno.P2pInfErrnoString()
}

//
// Message passed into user's callback
//
type P2pPackageCallback struct {
	PeerInfo		*peer.PeerInfo	// peer information
	ProtoId			int				// protocol identity
	PayloadLength	int				// bytes in payload buffer
	Payload			[]byte			// payload buffer
}

//
// Message from user
//
type P2pPackage2Peer struct {
	IdList			[]peer.PeerId	// peer identity list
	ProtoId			int				// protocol identity
	PayloadLength	int				// payload length
	Payload			[]byte			// payload
	Extra			interface{}		// extra info: user this field to tell p2p more about this message,
									// for example, if broadcasting is wanted, then set IdList to nil
									// and setup thie extra info field.
}

//
// callback type
//
const (
	P2pInfIndCb	= iota
	P2pInfPkgCb
)

//
// P2p peer status indication callback type
//
const (
	P2pIndPeerActivated	= iota	// peer activated
	P2pIndConnStatus			// connection status changed
	P2pIndPeerClosed			// connection closed
)

type P2pIndPeerActivatedPara struct {
	Inst		interface{}
	PeerInfo	*peer.Handshake
}

type P2pIndConnStatusPara int

type P2pIndPeerClosedPara int

type P2pInfIndCallback func(what int, para interface{}) interface{}

//
// P2p callback function type for package incoming
//
type P2pInfPkgCallback func(msg *P2pPackageCallback) interface{}

//
// Register user callback function to p2p
//
var P2pIndHandler P2pInfIndCallback = nil
var Lock4Cb sync.Mutex

func P2pInfRegisterCallback(what int, cb interface{}, ptn interface{}) P2pInfErrno {

	if what != P2pInfIndCb && what != P2pInfPkgCb {
		yclog.LogCallerFileLine("P2pInfRegisterCallback: " +
			"invalid callback type: %d",
			what)
		return P2pInfEnoParameter
	}

	if what == P2pInfIndCb {
		if P2pIndHandler != nil {
			yclog.LogCallerFileLine("P2pInfRegisterCallback: old handler will be overlapped")
		}
		Lock4Cb.Lock()
		P2pIndHandler = cb.(P2pInfIndCallback)
		Lock4Cb.Unlock()
		return P2pInfEnoNone
	}

	if ptn == nil {
		yclog.LogCallerFileLine("P2pInfRegisterCallback: nil task node pointer")
		return P2pInfEnoParameter
	}

	if eno := peer.SetP2pkgCallback(cb, ptn); eno != peer.PeMgrEnoNone {
		yclog.LogCallerFileLine("P2pInfRegisterCallback: " +
			"SetP2pkgCallback failed, eno: %d",
			eno)
		return P2pInfEnoInternal
	}

	return P2pInfEnoNone
}

//
// Send message to peer
//
func P2pInfSendPackage(pkg *P2pPackage2Peer) P2pInfErrno {

	if eno, failed := peer.SendPackage(pkg); eno != peer.PeMgrEnoNone {

		yclog.LogCallerFileLine("P2pInfSendPackage: " +
			"SendPackage failed, eno: %d, pkg: %s",
			eno,
			fmt.Sprintf("%+v", *pkg))

		var str = ""

		for _, f := range failed {
			str = str + fmt.Sprintf("%x", *f)
		}

		yclog.LogCallerFileLine("P2pInfSendPackage: " +
			"failed list: %s",
			str)

		return P2pInfEnoInternal
	}

	return P2pInfEnoNone
}

//
// Disconnect peer
//
func P2pInfClosePeer(id *peer.PeerId) P2pInfErrno {
	if eno := peer.ClosePeer(id); eno != peer.PeMgrEnoNone {
		yclog.LogCallerFileLine("P2pInfSendPackage: " +
			"ClosePeer failed, eno: %d, peer: %s",
			eno,
			fmt.Sprintf("%+v", *id))
		return P2pInfEnoInternal
	}
	return P2pInfEnoNone
}

//
// Free total p2p all
//
func P2pInfPoweroff() P2pInfErrno {
	yclog.LogCallerFileLine("P2pInfPoweroff: not supported yet")
	return P2pInfEnoNotImpl
}