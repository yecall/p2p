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
	"io"
	"time"
	ggio "github.com/gogo/protobuf/io"
	ycfg	"ycp2p/config"
	pb		"ycp2p/peer/pb"
	yclog	"ycp2p/logger"
)


//
// Max protocols supported
//
const MaxProtocols = ycfg.MaxProtocols

//
// Protocol identities
//
const (
	PID_P2P			= pb.ProtocolId_PID_P2P
	PID_EXT			= pb.ProtocolId_PID_EXT
)

//
// Message identities
//
const (
	MID_HANDSHAKE	= pb.MessageId_MID_HANDSHAKE
	MID_PING		= pb.MessageId_MID_PING
	MID_PONG		= pb.MessageId_MID_PONG
)

//
// Protocol
//
type Protocol struct {
	Pid		uint32	// protocol identity
	Ver		[4]byte	// protocol version: M.m0.m1.m2
}

//
// Handshake message
//
type Handshake struct {
	NodeId		ycfg.NodeID	// node identity
	ProtoNum	uint32		// number of protocols supported
	Protocols	[]Protocol	// version of protocol
}

//
// Package for TCP message
//
type P2pPackage struct {
	Pid				uint32	// protocol identity
	PayloadLength	uint32	// payload length
	Payload			[]byte	// payload
}

//
// Read handshake message from inbound peer
//
func (upkg *P2pPackage)getHandshakeInbound(inst *peerInstance) (*Handshake, PeMgrErrno) {

	//
	// read "package" message firstly
	//

	if inst.hto != 0 {
		inst.conn.SetReadDeadline(time.Now().Add(inst.hto))
	} else {
		inst.conn.SetReadDeadline(time.Time{})
	}

	r := inst.conn.(io.Reader)
	gr := ggio.NewDelimitedReader(r, inst.maxPkgSize)
	pkg := new(pb.P2PPackage)

	if err := gr.ReadMsg(pkg); err != nil {

		yclog.LogCallerFileLine("getHandshakeInbound: " +
			"ReadMsg faied, err: %s",
			err.Error())

		return nil, PeMgrEnoOs
	}

	//
	// check the package read
	//

	if *pkg.Pid != PID_P2P {

		yclog.LogCallerFileLine("getHandshakeInbound: " +
			"not a Hadnshake package, pid: %d",
			*pkg.Pid)

		return nil, PeMgrEnoMessage
	}

	if *pkg.PayloadLength <= 0 {

		yclog.LogCallerFileLine("getHandshakeInbound: " +
			"invalid payload length: %d",
			*pkg.PayloadLength)

		return nil, PeMgrEnoMessage
	}

	if len(pkg.Payload) != int(*pkg.PayloadLength) {

		yclog.LogCallerFileLine("getHandshakeInbound: " +
			"payload length mismatched, PlLen: %d, real: %d",
			*pkg.PayloadLength, len(pkg.Payload))

		return nil, PeMgrEnoMessage
	}

	//
	// decode the payload to get "handshake" message
	//

	pbMsg := new(pb.P2PMessage)

	if err := pbMsg.Unmarshal(pkg.Payload); err != nil {

		yclog.LogCallerFileLine("getHandshakeInbound:" +
			"Unmarshal failed, err: %s",
			err.Error())

		return nil, PeMgrEnoMessage
	}

	//
	// check the "handshake" message
	//
	
	if *pbMsg.Mid != MID_HANDSHAKE {

		yclog.LogCallerFileLine("getHandshakeInbound: " +
			"it's not a handshake message, mid: %d",
			*pbMsg.Mid)

		return nil, PeMgrEnoMessage
	}

	pbHS := pbMsg.Handshake

	if pbHS == nil {

		yclog.LogCallerFileLine("getHandshakeInbound: " +
			"invalid handshake message pointer: %p",
			pbHS)

		return nil, PeMgrEnoMessage
	}

	if len(pbHS.NodeId) != ycfg.NodeIDBytes {

		yclog.LogCallerFileLine("getHandshakeInbound:" +
			"invalid node identity length: %d",
				len(pbHS.NodeId))

		return nil, PeMgrEnoMessage
	}

	if *pbHS.ProtoNum > MaxProtocols {

		yclog.LogCallerFileLine("getHandshakeInbound:" +
			"too much protocols: %d",
			*pbHS.ProtoNum)

		return nil, PeMgrEnoMessage
	}

	if int(*pbHS.ProtoNum) != len(pbHS.Protocols) {

		yclog.LogCallerFileLine("getHandshakeInbound: " +
			"number of protocols mismathced, ProtoNum: %d, real: %d",
			int(*pbHS.ProtoNum), len(pbHS.Protocols))

		return nil, PeMgrEnoMessage
	}

	//
	// get handshake info to return
	//

	var ptrMsg = new(Handshake)
	copy(ptrMsg.NodeId[:], pbHS.NodeId)
	ptrMsg.ProtoNum = *pbHS.ProtoNum

	ptrMsg.Protocols = make([]Protocol, len(pbHS.Protocols))
	for i, p := range pbHS.Protocols {
		ptrMsg.Protocols[i].Pid = uint32(*p.Pid)
		copy(ptrMsg.Protocols[i].Ver[:], p.Ver)
	}

	return ptrMsg, PeMgrEnoNone
}

//
// Write handshake message to peer
//
func (upkg *P2pPackage)putHandshakeOutbound(inst *peerInstance, hs *Handshake) PeMgrErrno {

	//
	// encode "handshake" message as the payload of "package" message
	//

	pbHandshakeMsg := new(pb.P2PMessage_Handshake)
	pbHandshakeMsg.NodeId = append(pbHandshakeMsg.NodeId, hs.NodeId[:] ...)
	pbHandshakeMsg.ProtoNum = &hs.ProtoNum
	pbHandshakeMsg.Protocols = make([]*pb.P2PMessage_Protocol, *pbHandshakeMsg.ProtoNum)

	for i, p := range hs.Protocols {
		var pbProto = new(pb.P2PMessage_Protocol)
		pbHandshakeMsg.Protocols[i] = pbProto
		pbProto.Pid = new(pb.ProtocolId)
		*pbProto.Pid = pb.ProtocolId_PID_P2P
		pbProto.Ver = append(pbProto.Ver, p.Ver[:]...)
	}

	pbMsg := new(pb.P2PMessage)
	pbMsg.Mid = new(pb.MessageId)
	*pbMsg.Mid = pb.MessageId_MID_HANDSHAKE
	pbMsg.Handshake = pbHandshakeMsg

	payload, err1 := pbMsg.Marshal()
	if err1 != nil {

		yclog.LogCallerFileLine("putHandshakeOutbound:" +
			"Marshal failed, err: %s",
			err1.Error())

		return PeMgrEnoMessage
	}

	//
	// setup the "package"
	//

	pbPkg := new(pb.P2PPackage)
	pbPkg.Pid = new(pb.ProtocolId)
	*pbPkg.Pid = pb.ProtocolId_PID_P2P
	pbPkg.PayloadLength = new(uint32)
	*pbPkg.PayloadLength = uint32(len(payload))
	pbPkg.Payload = append(pbPkg.Payload, payload...)

	//
	// send package to peer, notice need to encode here directly, it would be
	// done in calling of gw.WriteMsg, see bellow.
	//

	if inst.hto != 0 {
		inst.conn.SetWriteDeadline(time.Now().Add(inst.hto))
	} else {
		inst.conn.SetWriteDeadline(time.Time{})
	}

	w := inst.conn.(io.Writer)
	gw := ggio.NewDelimitedWriter(w)

	if err := gw.WriteMsg(pbPkg); err != nil {

		yclog.LogCallerFileLine("putHandshakeOutbound:" +
			"Write failed, err: %s",
			err.Error())

		return PeMgrEnoOs
	}

	return PeMgrEnoNone
}

//
// Send user packege
//

func (upkg *P2pPackage)SendPackage(inst *peerInstance) PeMgrErrno {

	if inst == nil {
		yclog.LogCallerFileLine("SendPackage: invalid parameter")
		return PeMgrEnoParameter
	}

	//
	// Setup the protobuf "package"
	//

	pbPkg := new(pb.P2PPackage)
	pbPkg.Pid = new(pb.ProtocolId)
	*pbPkg.Pid = pb.ProtocolId(upkg.Pid)
	pbPkg.PayloadLength = new(uint32)
	*pbPkg.PayloadLength = uint32(upkg.PayloadLength)
	pbPkg.Payload = append(pbPkg.Payload, upkg.Payload...)

	//
	// Set deadline
	//

	if inst.hto != 0 {
		inst.conn.SetWriteDeadline(time.Now().Add(inst.hto))
	} else {
		inst.conn.SetWriteDeadline(time.Time{})
	}

	//
	// Create writer and then write package to peer
	//

	w := inst.conn.(io.Writer)
	gw := ggio.NewDelimitedWriter(w)

	if err := gw.WriteMsg(pbPkg); err != nil {

		yclog.LogCallerFileLine("SendPackage:" +
			"Write failed, err: %s",
			err.Error())

		return PeMgrEnoOs
	}

	return PeMgrEnoNone
}

//
// Receive user package
//
func (upkg *P2pPackage)RecvPackage(inst *peerInstance) PeMgrErrno {

	if inst == nil {
		yclog.LogCallerFileLine("RecvPackage: invalid parameter")
		return PeMgrEnoParameter
	}

	//
	// Setup the reader
	//

	if inst.hto != 0 {
		inst.conn.SetReadDeadline(time.Now().Add(inst.hto))
	} else {
		inst.conn.SetReadDeadline(time.Time{})
	}

	r := inst.conn.(io.Reader)
	gr := ggio.NewDelimitedReader(r, inst.maxPkgSize)

	//
	// New protobuf "package" and read peer into it
	//

	pkg := new(pb.P2PPackage)

	if err := gr.ReadMsg(pkg); err != nil {

		yclog.LogCallerFileLine("RecvPackage: " +
			"ReadMsg faied, err: %s",
			err.Error())

		return PeMgrEnoOs
	}

	return PeMgrEnoNone
}