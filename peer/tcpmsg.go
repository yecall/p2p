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
type TcpmsgHandshake struct {
	NodeId		ycfg.NodeID
	ProtoNum	uint32
	Protocols	[]Protocol
}

//
// Package for TCP message
//
type TcpmsgPackage struct {
	Pid		uint32	// protocol identity
	PlLen	uint32	// payload length
	Payload	[]byte	// payload
}

//
// Read handshake message from inbound peer
//
func (tp *TcpmsgPackage)getHandshakeInbound(inst *peerInstance) (*TcpmsgHandshake, PeMgrErrno) {

	inst.conn.SetDeadline(time.Now().Add(inst.hto))
	r := inst.conn.(io.Reader)
	gr := ggio.NewDelimitedReader(r, inst.maxPkgSize)

	pkg := new(pb.TcpmsgPackage)
	if err := gr.ReadMsg(pkg); err != nil {
		yclog.LogCallerFileLine("getHandshakeInbound: NewDelimitedReader faied, err: %s", err.Error())
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

	pbHS := new(pb.TcpmsgHandshake)
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

	var ptrMsg *TcpmsgHandshake = new(TcpmsgHandshake)
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
func (tp *TcpmsgPackage)putHandshakeOutbound(inst *peerInstance, hs *TcpmsgHandshake) PeMgrErrno {

	//
	// encode handshake message to package payload
	//

	pbMsg := new(pb.TcpmsgHandshake)

	for _, b := range hs.NodeId {
		pbMsg.NodeId = append(pbMsg.NodeId, b)
	}

	pbMsg.ProtoNum = &hs.ProtoNum

	for i, p := range hs.Protocols {

		pbMsg.Protocols[i].Pid = new(pb.ProtocolId)

		// we now treate Handshake as a protocol, but seems it needs not to be presented
		// in protocol table in handshake message?
		if p.Pid == uint32(pb.ProtocolId_PROTO_HANDSHAKE) {
			*pbMsg.Protocols[i].Pid = pb.ProtocolId_PROTO_HANDSHAKE
		} else if p.Pid == uint32(pb.ProtocolId_PROTO_EXTERNAL) {
			*pbMsg.Protocols[i].Pid = pb.ProtocolId_PROTO_EXTERNAL
		} else {
			yclog.LogCallerFileLine("putHandshakeOutbound: invalid pid: %d", p.Pid)
			return PeMgrEnoMessage
		}

		pbMsg.Protocols[i].Ver = append(pbMsg.Protocols[i].Ver, p.Ver[0], p.Ver[1], p.Ver[2], p.Ver[3])
	}

	var payload []byte
	var err error

	tp.Pid = uint32(pb.ProtocolId_PROTO_HANDSHAKE)
	if payload, err = pbMsg.Marshal(); err != nil {
		yclog.LogCallerFileLine("putHandshakeOutbound: Marshal failed, err: %s", err.Error())
		return PeMgrEnoMessage
	}

	if tp.PlLen = uint32(len(payload)); tp.PlLen <= 0 {
		yclog.LogCallerFileLine("putHandshakeOutbound: invalid payload length")
		return PeMgrEnoMessage
	}
	tp.Payload = payload

	//
	// encode the package
	//

	pbPkg := new(pb.TcpmsgPackage)
	pbPkg.Pid = new(pb.ProtocolId)
	*pbPkg.Pid = pb.ProtocolId_PROTO_HANDSHAKE
	*pbPkg.PlLen = tp.PlLen
	pbPkg.Payload = tp.Payload

	var sendBuf []byte
	if sendBuf, err = pbPkg.Marshal(); err != nil {
		yclog.LogCallerFileLine("putHandshakeOutbound: Marshal failed, err: %s", err.Error())
		return PeMgrEnoMessage
	}

	if len(sendBuf) <= 0 {
		yclog.LogCallerFileLine("putHandshakeOutbound: invalid send buffer");
		return PeMgrEnoMessage
	}

	//
	// send encoded package to peer
	//

	inst.conn.SetDeadline(time.Now().Add(inst.hto))
	w := inst.conn.(io.Writer)
	if n, _ := w.Write(sendBuf); n != len(sendBuf) {
		yclog.LogCallerFileLine("putHandshakeOutbound: Write failed, err: %s", err.Error())
		return PeMgrEnoOs
	}

	return PeMgrEnoNone
}