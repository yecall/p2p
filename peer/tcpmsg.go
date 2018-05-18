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
	PROTO_HANDSHAKE	= pb.ProtocolId_PROTO_HANDSHAKE	// handshake
	PROTO_EXTERNAL	= pb.ProtocolId_PROTO_EXTERNAL	// external
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
	NodeId		ycfg.NodeID
	ProtoNum	uint32
	Protocols	[]Protocol
}

//
// Package for TCP message
//
type P2pPackage struct {
	Pid		uint32	// protocol identity
	PlLen	uint32	// payload length
	Payload	[]byte	// payload
}

//
// Read handshake message from inbound peer
//
func (tp *P2pPackage)getHandshakeInbound(inst *peerInstance) (*Handshake, PeMgrErrno) {

	if inst.hto != 0 {
		inst.conn.SetReadDeadline(time.Now().Add(inst.hto))
	} else {
		inst.conn.SetReadDeadline(time.Time{})
	}

	r := inst.conn.(io.Reader)
	gr := ggio.NewDelimitedReader(r, inst.maxPkgSize)

	pkg := new(pb.P2pPackage)
	if err := gr.ReadMsg(pkg); err != nil {
		yclog.LogCallerFileLine("getHandshakeInbound: ReadMsg faied, err: %s", err.Error())
		return nil, PeMgrEnoOs
	}

	if *pkg.Pid != PROTO_HANDSHAKE {
		yclog.LogCallerFileLine("getHandshakeInbound: not a Hadnshake package, pid: %d", *pkg.Pid)
		return nil, PeMgrEnoMessage
	}

	if *pkg.PlLen <= 0 {
		yclog.LogCallerFileLine("getHandshakeInbound: invalid payload length: %d", *pkg.PlLen)
		return nil, PeMgrEnoMessage
	}

	if len(pkg.Payload) != int(*pkg.PlLen) {
		yclog.LogCallerFileLine("getHandshakeInbound: " +
			"payload length mismatched, PlLen: %d, real: %d",
			*pkg.PlLen, len(pkg.Payload))
		return nil, PeMgrEnoMessage
	}

	pbHS := new(pb.Handshake)
	pbHS.Unmarshal(pkg.Payload)

	if len(pbHS.NodeId) != ycfg.NodeIDBytes {
		yclog.LogCallerFileLine("getHandshakeInbound: invalid node identity length: %d", len(pbHS.NodeId))
		return nil, PeMgrEnoMessage
	}

	if *pbHS.ProtoNum > MaxProtocols {
		yclog.LogCallerFileLine("getHandshakeInbound: too much protocols: %d", *pbHS.ProtoNum)
		return nil, PeMgrEnoMessage
	}

	if int(*pbHS.ProtoNum) != len(pbHS.Protocols) {
		yclog.LogCallerFileLine("getHandshakeInbound: " +
			"number of protocols mismathced, ProtoNum: %d, real: %d",
			int(*pbHS.ProtoNum), len(pbHS.Protocols))
		return nil, PeMgrEnoMessage
	}

	var ptrMsg = new(Handshake)
	for i, b := range pbHS.NodeId {
		ptrMsg.NodeId[i] = b
	}

	ptrMsg.ProtoNum = *pbHS.ProtoNum

	for i, p := range pbHS.Protocols {
		ptrMsg.Protocols[i].Pid = uint32(*p.Pid)
		for j, b := range p.Ver {
			ptrMsg.Protocols[i].Ver[j] = b
		}
	}

	return ptrMsg, PeMgrEnoNone
}

//
// Write handshake message to peer
//
func (tp *P2pPackage)putHandshakeOutbound(inst *peerInstance, hs *Handshake) PeMgrErrno {

	//
	// encode handshake message to package payload
	//

	pbMsg := new(pb.Handshake)

	pbMsg.NodeId = append(pbMsg.NodeId, hs.NodeId[:] ...)
	pbMsg.ProtoNum = &hs.ProtoNum
	pbMsg.Protocols = make([]*pb.TcpmsgHandshake_Protocol, *pbMsg.ProtoNum)

	for i, p := range hs.Protocols {

		var pbProto = new(pb.TcpmsgHandshake_Protocol)
		pbMsg.Protocols[i] = pbProto

		pbProto.Pid = new(pb.ProtocolId)
		if p.Pid == uint32(pb.ProtocolId_PROTO_HANDSHAKE) {
			*pbProto.Pid = pb.ProtocolId_PROTO_HANDSHAKE
		} else if p.Pid == uint32(pb.ProtocolId_PROTO_EXTERNAL) {
			*pbProto.Pid = pb.ProtocolId_PROTO_EXTERNAL
		} else {
			yclog.LogCallerFileLine("putHandshakeOutbound: invalid pid: %d", p.Pid)
			return PeMgrEnoMessage
		}

		pbProto.Ver = append(pbProto.Ver, p.Ver[:]...)

	}

	payload, err1 := pbMsg.Marshal()

	if err1 != nil {
		yclog.LogCallerFileLine("putHandshakeOutbound: Marshal failed, err: %s", err1.Error())
		return PeMgrEnoMessage
	}

	//
	// encode the package
	//

	pbPkg := new(pb.P2pPackage)
	pbPkg.Pid = new(pb.ProtocolId)
	*pbPkg.Pid = pb.ProtocolId_PROTO_HANDSHAKE
	pbPkg.PlLen = new(uint32)
	*pbPkg.PlLen = uint32(len(payload))
	pbPkg.Payload = append(pbPkg.Payload, payload...)

	sendBuf, err2 := pbPkg.Marshal()
	if err2 != nil {
		yclog.LogCallerFileLine("putHandshakeOutbound: Marshal failed, err: %s", err2.Error())
		return PeMgrEnoMessage
	}

	if len(sendBuf) <= 0 {
		yclog.LogCallerFileLine("putHandshakeOutbound: invalid send buffer")
		return PeMgrEnoMessage
	}

	//
	// send encoded package to peer
	//

	if inst.hto != 0 {
		inst.conn.SetWriteDeadline(time.Now().Add(inst.hto))
	} else {
		inst.conn.SetWriteDeadline(time.Time{})
	}

	w := inst.conn.(io.Writer)
	if n, err3 := w.Write(sendBuf); n != len(sendBuf) {
		yclog.LogCallerFileLine("putHandshakeOutbound: Write failed, err: %s", err3.Error())
		return PeMgrEnoOs
	}

	return PeMgrEnoNone
}