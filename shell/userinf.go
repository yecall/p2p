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
	"ycp2p/peer"
)

//
// errno about this interface
//
type UserInfErrno	int
const (
	UserInfEnoNone		UserInfErrno = 0	// none of errors
	UserInfEnoParameter	UserInfErrno = 1	// invalid parameters
	UserInfoEnoNotImpl	UserInfErrno = 2	// not implemented
	UserInfEnoUnknown	UserInfErrno = 3	// unknown
	UserInfEnoMax		UserInfErrno = 4	// max, for bound checking
)

//
// Description about user interface errno
//
var UserInfErrnoDescription = []string {
	"none of errors",
	"invalid parameters",
	"unknown",
	"max, for bound checking",
}

//
// Stringz an errno with itself
//
func (eno UserInfErrno) UserInfErrnoString() string {
	if eno < UserInfEnoNone || eno >= UserInfEnoMax {
		return fmt.Sprintf("Can't be stringzed, invalid eno:%d", eno)
	}
	return UserInfErrnoDescription[eno]
}

//
// Stringz an errno with an eno parameter
//
func UserInfErrnoString(eno UserInfErrno) string {
	return eno.UserInfErrnoString()
}

//
// Messages' identity callbacked to user
//
const (
	CBMID_NULL		= iota	// it's null, means none
	CBMID_MAX				// for bound checking
)

//
// Message passed into user's callback
//
type UserCallbackMessageId	int			// message identity as integer
type UserCallbackMessage struct {
	MsgId		UserCallbackMessageId	// message identity
	PeerInfo	*peer.PeerInfo			// peer information
	MsgBody		interface{}				// message body
}

//
// Message from user
//
type UserMessage2Peer struct {
	IdList		[]peer.PeerId	// peer identity list
	PayloadLen	int				// payload length
	Payload		[]byte			// payload
	ExtraInfo	interface{}	// extra info: user this field to tell p2p more about this message,
								// for example, if broadcasting is wanted, then set IdList to nil
								// and setup thie extra info field.
}

//
// User callback function type
//
type UserInfCallback func(msg *UserCallbackMessage) interface{}

//
// Register user callback function to p2p
//
func UserInfRegisterCallback(ucb UserInfCallback) UserInfErrno {
	return UserInfEnoNone
}

//
// Send message to peer
//
func UserInfSendMessage(msg UserMessage2Peer) UserInfErrno {
	return UserInfEnoNone
}

//
// Disconnect peer
//
func UserInfDisconnectPeer(id peer.PeerId) UserInfErrno {
	return UserInfEnoNone
}

//
// Free total p2p all
//
func UserInfPoweroff() UserInfErrno {
	return UserInfEnoNone
}