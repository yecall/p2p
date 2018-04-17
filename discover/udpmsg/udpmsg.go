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

package udpmsg

import (
	"net"
	yclog	"ycp2p/logger"
	ycfg	"ycp2p/config"
	pb		"ycp2p/discover/udpmsg/pb"
)

//
// UDP messages for discovering protocol tasks
//
const (
	UdpMsgTypePing		= iota
	UdpMsgTypePong
	UdpMsgTypeFindNode
	UdpMsgTypeNeighbors
	UdpMsgTypeUnknown
)

type UdpMsgType int

type (
	Endpoint struct {
		IP			net.IP
		UDP			uint16
		TCP 		uint16
	}

	Node struct {
		IP			net.IP
		UDP			uint16
		TCP			uint16
		NodeId		ycfg.NodeID
	}

	Ping struct {
		From		Endpoint
		To			Endpoint
		Expiration	uint64
		Extra		[]byte
	}

	Pong struct {
		From		Endpoint
		To			Endpoint
		Expiration	uint64
		Extra		[]byte
	}

	FindNode struct {
		From		Endpoint
		To			Endpoint
		Target		Node
		Expiration	uint64
		Extra		[]byte
	}

	Neighbors struct {
		From		Endpoint
		To			Endpoint
		nodes		Node
		Expiration	uint64
		Extra		[]byte
	}
)

//
// UDP message: tow parts, the first is the raw bytes ... the seconde is
// protobuf message. for decoding, protobuf message will be extract from
// the raw one; for encoding, bytes will be wriiten into raw buffer.
//
// Notice: since we would only one UDP reader for descovering, we can put
// an UdpMsg instance here.
//
type UdpMsg struct {
	Buf		[]byte
	Len		int
	From	*net.UDPAddr
	Msg		pb.UdpMessage
	Eno		UdpMsgErrno
}

var udpMsg = UdpMsg {
	Buf:	nil,
	Len:	0,
	From:	nil,
	Msg:	nil,
	Eno:	UdpMsgEnoUnknown,
}

var PtrUdpMsg = &udpMsg

const (
	UdpMsgEnoNone 		= iota
	UdpMsgEnoParameter
	UdpMsgEnoEncodeFailed
	UdpMsgEnoDecodeFailed
	UdpMsgEnoUnknown
)

type UdpMsgErrno int

//
// Set raw message
//
func (pum *UdpMsg) SetRawMessage(buf []byte, len int, from *net.UDPAddr) UdpMsgErrno {
	if buf == nil || len == 0 || from == nil {
		yclog.LogCallerFileLine("SetRawMessage: invalid parameter(s)")
		return UdpMsgEnoParameter
	}
	pum.Eno = UdpMsgEnoNone
	pum.Buf = buf
	pum.Len = len
	pum.From = from
	return UdpMsgEnoNone
}

//
// Decoding
//
func (pum *UdpMsg) Decode() UdpMsgErrno {
	if err := (&pum.Msg).Unmarshal(pum.Buf); err != nil {
		yclog.LogCallerFileLine("Decode: Unmarshal failed, err: %s", err.Error())
		return UdpMsgEnoDecodeFailed
	}
	return UdpMsgEnoNone
}

//
// Get decoded message
//
func (pum *UdpMsg) GetPbMessage() *pb.UdpMessage {
	return &pum.Msg
}

//
// Get decoded message
//
func (pum *UdpMsg) GetDecodedMsg() interface{} {

	// get type
	mt := pum.GetDecodedMsgType()
	if mt == UdpMsgTypeUnknown {
		yclog.LogCallerFileLine("GetDecodedMsg: GetDecodedMsgType failed, mt: %d", mt)
		return nil
	}

	// map type to function and the get
	var funcMap = map[UdpMsgType]interface{} {
		UdpMsgTypePing: pum.GetPing,
		UdpMsgTypePong: pum.GetPong,
		UdpMsgTypeFindNode: pum.GetFindNode,
		UdpMsgTypeNeighbors: pum.GetNeighbors,
	}

	var f interface{}
	var ok bool
	if f, ok = funcMap[mt]; !ok {
		yclog.LogCallerFileLine("GetDecodedMsg: invalid message type: %d", mt)
		return nil
	}

	return f.(func()interface{})()
}

//
// Get deocded message type
//
func (pum *UdpMsg) GetDecodedMsgType() UdpMsgType {
	var pbMap = map[pb.UdpMessage_MessageType]UdpMsgType {
		pb.UdpMessage_PING:			UdpMsgTypePing,
		pb.UdpMessage_PONG:			UdpMsgTypePong,
		pb.UdpMessage_FINDNODE:		UdpMsgTypePong,
		pb.UdpMessage_NEIGHBORS:	UdpMsgTypePing,
	}

	var key pb.UdpMessage_MessageType
	var val UdpMsgType
	var ok bool
	key = pum.Msg.GetMsgType()
	if val, ok = pbMap[key]; !ok {
		yclog.LogCallerFileLine("GetDecodedMsgType: invalid message type")
		return UdpMsgTypeUnknown
	}
	return val
}

//
// Get decoded Ping
//
func (pum *UdpMsg) GetPing() *Ping {
	return nil
}

//
// Get decoded Pong
//
func (pum *UdpMsg) GetPong() *Pong {
	return nil
}

//
// Get decoded FindNode
//
func (pum *UdpMsg) GetFindNode() *FindNode {
	return nil
}

//
// Get decoded Neighbors
//
func (pum *UdpMsg) GetNeighbors() *Neighbors {
	return nil
}

//
// Encode directly from protobuf message
//
func (pum *UdpMsg) EncodePbMsg() UdpMsgErrno {
	var err error
	if pum.Buf, err = (&pum.Msg).Marshal(); err != nil {
		yclog.LogCallerFileLine("Encode: Marshal failed, err: %s", err.Error())
		pum.Eno = UdpMsgEnoEncodeFailed
		return pum.Eno
	}
	pum.Eno = UdpMsgEnoNone
	return pum.Eno
}

//
// Encode from UDP messages
//
func (pum *UdpMsg) Encode(t int, msg interface{}) UdpMsgErrno {

	var eno UdpMsgErrno

	switch t {
	case UdpMsgTypePing:
		eno = pum.EncodePing(msg.(*Ping))
		break
	case UdpMsgTypePong:
		eno = pum.EncodePong(msg.(*Pong))
		break
	case UdpMsgTypeFindNode:
		eno = pum.EncodeFindNode(msg.(*FindNode))
		break
	case UdpMsgTypeNeighbors:
		eno = pum.EncodeNeighbors(msg.(*Neighbors))
		break
	default:
		eno = UdpMsgEnoParameter
	}

	if eno != UdpMsgEnoNone {
		yclog.LogCallerFileLine("Encode: failed, type: %d", t)
	}

	pum.Eno = eno
	return eno
}

//
// Encode Ping
//
func (pum *UdpMsg) EncodePing(ping *Ping) UdpMsgErrno {
	return UdpMsgEnoNone
}

//
// Encode Pong
//
func (pum *UdpMsg) EncodePong(pong *Pong) UdpMsgErrno {
	return UdpMsgEnoNone
}

//
// Encode FindNode
//
func (pum *UdpMsg) EncodeFindNode(fn *FindNode) UdpMsgErrno {
	return UdpMsgEnoNone
}

//
// Encode Neighbors
//
func (pum *UdpMsg) EncodeNeighbors(nb *Neighbors) UdpMsgErrno {
	return UdpMsgEnoNone
}




