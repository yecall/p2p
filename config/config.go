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


package config

import (
	"strings"
	"strconv"
	"net"
	"crypto/ecdsa"
	"crypto/elliptic"
	"path/filepath"
	"os"
	"os/user"
	"runtime"
	"fmt"
	ethereum "github.com/ethereum/go-ethereum/crypto"
	yclog "ycp2p/logger"
)


//
// errno
//
type P2pCfgErrno int

const (
	PcfgEnoNone			= iota
	PcfgEnoParameter
	PcfgEnoPublicKye
	PcfgEnoPrivateKye
	PcfgEnoDataDir
	PcfgEnoDatabase
	PcfgEnoIpAddr
	PcfgEnoNodeId
)

//
// Some paths
//
const (
	PcfgEnoIpAddrivateKey	= "nodekey"	// Path within the datadir to the node's private key
	datadirNodeDatabase		= "nodes"	// Path within the datadir to store the node infos
)

//
// Bootstrap nodes, in a format like:
//
//	node-identity-hex-string@ip:udp-port:tcp-port
//
const P2pMaxBootstrapNodes = 32

var BootstrapNodeUrl = []string {
	"EF660C03932E84402507BCC29763756B1DDFCB636CE5F562FC92383ED180480019DD6A32EEE1F3CEB832A47EB7C13DC415A18E4B3F634FB951966BFB0292ADA8@192.168.2.102:30303:30303",
}

//
// Build "Node" struct for bootstraps
//
var BootstrapNodes = P2pSetupDefaultBootstrapNodes()

//
// Node ID length in bits
//
const NodeIDBits	= 512
const NodeIDBytes	= NodeIDBits / 8

//
// Node identity
//
type NodeID [NodeIDBytes]byte

//
// Max protocols
//
const MaxProtocols = 32

//
// Max peers
//
const MaxPeers = 32

//
// Max concurrecny inboudn and outbound
//
const MaxInbounds	= MaxPeers / 2
const MaxOutbounds	= MaxPeers / 2



//
// Node
//
type Node struct {
	IP			net.IP		// ip address
	UDP, TCP	uint16		// port numbers
	ID			NodeID		// the node's public key
}

//
// Node static Configuration parameters
//
type Config struct {
	Version			string				// p2p version
	PrivateKey		*ecdsa.PrivateKey	// node private key
	PublicKey		*ecdsa.PublicKey	// node public key
	MaxPeers		int					// max peers can be
	MaxInbounds		int					// max peers for inbound concurrency establishing can be
	MaxOutbounds	int					// max peers for outbound concurrency establishing can be
	Name			string				// node name
	BootstrapNodes	[]*Node				// bootstrap nodes
	StaticNodes		[]*Node				// static nodes
	NodeDataDir		string				// node data directory
	NodeDatabase	string				// node database
	ListenAddr		string				// address listened
	NoDial			bool				// outboundless flag
	BootstrapNode	bool				// bootstrap node flag
	Local			Node				// myself
}

//
// Configuration about neighbor listener on UDP
//
type Cfg4UdpListener struct {
	IP		net.IP		// ip address
	UDP		uint16		// udp port numbers
	TCP		uint16		// tcp port numbers
	ID		NodeID		// the node's public key
}

//
// Configuration about peer listener on TCP
//
type Cfg4PeerListener struct {
	IP			net.IP	// ip address
	Port		uint16	// port numbers
	ID			NodeID	// the node's public key
	MaxInBounds	int		// max concurrency inbounds
}

//
// Configuration about peer manager
//
type Cfg4PeerManager struct {
	IP				net.IP	// ip address
	Port			uint16	// port number
	ID				NodeID	// the node's public key
	MaxPeers		int		// max peers would be
	MaxOutbounds	int		// max concurrency outbounds
	MaxInBounds		int		// max concurrency inbounds
	Statics			[]*Node	// static nodes
	NoDial			bool	// do not dial outbound
	BootstrapNode	bool	// local is a bootstrap node
}

//
// Configuration about table manager
//
type Cfg4TabManager struct {
	Local			Node	// local node
	BootstrapNodes	[]*Node	// bootstrap nodes
	DataDir			string	// data directory
	NodeDB			string	// node database
	BootstrapNode	bool	// bootstrap node flag
}

//
// Default version string, formated as "M.m0.m1.m2"
//
const dftVersion = "0.1.0.0"

//
// Default p2p instance name
//
const dftName = "test"

//
// Default configuration(notice that it's not a validated configuration and
// could never be applied), most of those defaults must be overided by higher
// lever module of system.
//
const (
	dftUdpPort = 30303
	dftTcpPort = 30303
)

var dftLocal = Node {
	IP:		net.IPv4(192,168,2,178),
	UDP:	dftUdpPort,
	TCP:	dftTcpPort,
	ID:		NodeID{0},
}

var config = Config {
	Version:			dftVersion,
	PrivateKey:			nil,
	PublicKey:			nil,
	MaxPeers:			MaxPeers,
	MaxInbounds:		MaxInbounds,
	MaxOutbounds:		MaxOutbounds,
	Name:				dftName,
	BootstrapNodes:		BootstrapNodes,
	StaticNodes:		nil,
	NodeDataDir:		P2pDefaultDataDir(true),
	NodeDatabase:		datadirNodeDatabase,
	NoDial:				true,
	BootstrapNode:		true,
	Local:				dftLocal,
}

var PtrConfig = &config

//
// Get default config
//
func P2pDefaultConfig() *Config {
	return &config
}

//
// P2pConfig
//
func P2pConfig(cfg *Config) P2pCfgErrno {

	//
	// Update, one SHOULD first call P2pDefaultConfig to get default value, modify
	// some fields if necessary, and then call this function, since the configuration
	// is overlapped directly here in this function.
	//

	if cfg == nil {
		yclog.LogCallerFileLine("P2pConfig: invalid configuration")
		return PcfgEnoParameter
	}
	config = *cfg

	//
	// Check configuration. Notice that we do not need a private key in current
	// implement, only public key needed to build the local node identity, so one
	// can leave private to be nil while give a not nil public key. If both are
	// nils, key pair will be built, see bellow pls.
	//

	if config.PrivateKey == nil {
		yclog.LogCallerFileLine("P2pConfig: private key is empty")
	}

	if config.PublicKey == nil {
		yclog.LogCallerFileLine("P2pConfig: public key is empty")
	}

	if config.MaxPeers == 0 ||
		config.MaxOutbounds == 0 ||
		config.MaxInbounds == 0	||
		config.MaxPeers < config.MaxInbounds + config.MaxOutbounds {
		yclog.LogCallerFileLine("P2pConfig: " +
			"invalid peer number constraint, MaxPeers: %d, MaxOutbounds: %d, MaxInbounds: %d",
			config.MaxPeers, config.MaxOutbounds, config.MaxInbounds)
		return PcfgEnoParameter
	}

	if len(config.Name) == 0 {
		yclog.LogCallerFileLine("P2pConfig: node name is empty")
	}

	if cap(config.BootstrapNodes) == 0 {
		yclog.LogCallerFileLine("P2pConfig: BootstrapNodes is empty")
	}

	if cap(config.StaticNodes) == 0 {
		yclog.LogCallerFileLine("P2pConfig: StaticNodes is empty")
	}

	//
	// Seems path.IsAbs does not work under Windows
	//

	if len(config.NodeDataDir) == 0 /*|| path.IsAbs(config.NodeDataDir) == false*/ {
		yclog.LogCallerFileLine("P2pConfig: invaid data directory")
		return PcfgEnoDataDir
	}

	if len(config.NodeDatabase) == 0 {
		yclog.LogCallerFileLine("P2pConfig: invalid database name")
		return PcfgEnoDatabase
	}

	if config.Local.IP == nil {
		yclog.LogCallerFileLine("P2pConfig: invalid ip address")
		return PcfgEnoIpAddr
	}

	//
	// setup local node identity from key
	//

	if p2pSetupLocalNodeId() != PcfgEnoNone {
		yclog.LogCallerFileLine("P2pConfig: invalid ip address")
		return PcfgEnoNodeId
	}

	return PcfgEnoNone
}

//
// Node identity to hex string
//
func P2pNodeId2HexString(id NodeID) string {
	return fmt.Sprintf("%X", id[:])
}

//
// Hex-string to node identity
//
func P2pHexString2NodeId(hex string) *NodeID {

	var nid = NodeID{byte(0)}

	if len(hex) != NodeIDBytes * 2 {
		yclog.LogCallerFileLine("P2pHexString2NodeId: " +
			"invalid length: %d",
			len(hex))
		return nil
	}

	for cidx, c := range hex {

		if c >= '0' && c <= '9' {
			c = c - '0'
		} else if c >= 'a' && c <= 'f' {
			c = c - 'a' + 10
		} else if c >= 'A' && c <= 'F' {
			c = c - 'A' + 10
		} else {
			yclog.LogCallerFileLine("P2pHexString2NodeId: " +
				"invalid string: %s",
				hex)
			return nil
		}

		bidx := cidx >> 1
		if cidx & 0x01 == 0 {
			nid[bidx] = byte(c << 4)
		} else {
			nid[bidx] += byte(c)
		}
	}

	return &nid
}

//
// Get default data directory
//
func P2pDefaultDataDir(flag bool) string {

	//
	// get home and setup default dir
	//

	home := P2pGetUserHomeDir()

	if home != "" {
		if runtime.GOOS == "darwin" {
			home = filepath.Join(home, "Library", "yee")
		} else if runtime.GOOS == "windows" {
			home = filepath.Join(home, "AppData", "Roaming", "Yee")
		} else {
			home = filepath.Join(home, ".yee")
		}
	}

	//
	// check flag to create default directory if it's not exit
	//

	if flag {
		_, err := os.Stat(home)
		if err == nil {
			return home
		}
		if os.IsNotExist(err) {
			err := os.MkdirAll(home, 0700)
			if err != nil {
				return ""
			}
		}
	}

	return home
}

//
// Get user directory
//
func P2pGetUserHomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

func p2pBuildPrivateKey() *ecdsa.PrivateKey {

	//
	// Here we apply the Ethereum crypto package to build node private key:
	//
	// 1) if no data directory specified, try to generate key, but do no save to file;
	// 2) if data directory presented, try to load key from file;
	// 3) if load failed, try to generate key and the save it to file;
	//
	// See bellow please, also see Ethereum function node.NodeKey pls.
	//

	if config.NodeDataDir == "" {
		key, err := ethereum.GenerateKey()
		if err != nil {
			yclog.LogCallerFileLine("p2pBuildPrivateKey: " +
				"GenerateKey failed, err: %s",
				err.Error())
			return nil
		}
		return key
	}

	keyfile := filepath.Join(config.NodeDataDir, config.Name, PcfgEnoIpAddrivateKey)
	if key, err := ethereum.LoadECDSA(keyfile); err == nil {
		yclog.LogCallerFileLine("p2pBuildPrivateKey: " +
			"private key loaded ok from file: %s",
			keyfile)
		return key
	}

	key, err := ethereum.GenerateKey()
	if err != nil {
		yclog.LogCallerFileLine("p2pBuildPrivateKey: " +
			"GenerateKey failed, err: %s",
			err.Error())
		return nil
	}

	instanceDir := filepath.Join(config.NodeDataDir, config.Name)
	if _, err := os.Stat(instanceDir); os.IsNotExist(err) {
		if err := os.MkdirAll(instanceDir, 0700); err != nil {
			yclog.LogCallerFileLine("p2pBuildPrivateKey: " +
				"MkdirAll failed, err: %s, path: %s",
				err.Error(), instanceDir)
			return key
		}
	}

	if err := ethereum.SaveECDSA(keyfile, key); err != nil {
		yclog.LogCallerFileLine("p2pBuildPrivateKey: " +
			"SaveECDSA failed, err: %s",
			err.Error())
	}

	yclog.LogCallerFileLine("p2pBuildPrivateKey: " +
		"key save ok to file: %s",
		keyfile)

	return key
}

//
// Trans public key to node identity
//
func p2pPubkey2NodeId(pub *ecdsa.PublicKey) *NodeID {

	var id NodeID

	pbytes := elliptic.Marshal(pub.Curve, pub.X, pub.Y)

	if len(pbytes)-1 != len(id) {
		yclog.LogCallerFileLine("p2pPubkey2NodeId: " +
			"invalid public key for node identity")
		return nil
	}
	copy(id[:], pbytes[1:])

	return &id
}

//
// Setup local node identity
//
func p2pSetupLocalNodeId() P2pCfgErrno {

	if config.PrivateKey != nil {

		config.PublicKey = &config.PrivateKey.PublicKey

	} else if config.PublicKey == nil {

		config.PrivateKey = p2pBuildPrivateKey()

		if config.PrivateKey == nil {
			yclog.LogCallerFileLine("p2pSetupLocalNodeId: " +
				"p2pBuildPrivateKey failed")
			return PcfgEnoPrivateKye
		}

		config.PublicKey = &config.PrivateKey.PublicKey
	}

	pnid := p2pPubkey2NodeId(config.PublicKey)

	if pnid == nil {

		yclog.LogCallerFileLine("p2pSetupLocalNodeId: " +
			"p2pPubkey2NodeId failed")
		return PcfgEnoPublicKye
	}
	config.Local.ID = *pnid

	yclog.LogCallerFileLine("p2pSetupLocalNodeId: " +
		"local node identity: %s",
		P2pNodeId2HexString(config.Local.ID))

	return PcfgEnoNone
}

//
// Setup default bootstrap nodes
//
func P2pSetupDefaultBootstrapNodes() []*Node {

	var bsn = make([]*Node, 0, P2pMaxBootstrapNodes)

	for idx, url := range BootstrapNodeUrl {

		strs := strings.Split(url,"@")
		if len(strs) != 2 {
			yclog.LogCallerFileLine("P2pSetupDefaultBootstrapNodes: " +
				"invalid bootstrap url: %s",
				url)
			return nil
		}

		strNodeId := strs[0]
		strs = strings.Split(strs[1],":")
		if len(strs) != 3 {
			yclog.LogCallerFileLine("P2pSetupDefaultBootstrapNodes: " +
				"invalid bootstrap url: %s",
				url)
			return nil
		}

		strIp := strs[0]
		strUdpPort := strs[1]
		strTcpPort := strs[2]

		pid := P2pHexString2NodeId(strNodeId)
		if pid == nil {
			yclog.LogCallerFileLine("P2pSetupDefaultBootstrapNodes: " +
				"P2pHexString2NodeId failed, strNodeId: %s",
				strNodeId)
			return nil
		}

		bsn = append(bsn, new(Node))
		copy(bsn[idx].ID[:], (*pid)[:])
		bsn[idx].IP = net.ParseIP(strIp)

		if port, err := strconv.Atoi(strUdpPort); err != nil {
			yclog.LogCallerFileLine("P2pSetupDefaultBootstrapNodes: " +
				"Atoi for UDP port failed, err: %s",
				err.Error())
			return nil
		} else {
			bsn[idx].UDP = uint16(port)
		}

		if port, err := strconv.Atoi(strTcpPort); err != nil {
			yclog.LogCallerFileLine("P2pSetupDefaultBootstrapNodes: " +
				"Atoi for TCP port failed, err: %s",
				err.Error())
			return nil
		} else {
			bsn[idx].TCP = uint16(port)
		}
	}

	return  bsn
}

//
// Get configuration of neighbor discovering listener
//
func P2pConfig4UdpListener() *Cfg4UdpListener {
	return &Cfg4UdpListener {
		IP:		config.Local.IP,
		UDP:	config.Local.UDP,
		TCP:	config.Local.TCP,
		ID:		config.Local.ID,
	}
}

//
// Get configuration of peer listener
//
func P2pConfig4PeerListener() *Cfg4PeerListener {
	return &Cfg4PeerListener {
		IP:			config.Local.IP,
		Port:		config.Local.TCP,
		ID:			config.Local.ID,
		MaxInBounds:config.MaxInbounds,
	}
}

//
// Get configuration of peer manager
//
func P2pConfig4PeerManager() *Cfg4PeerManager {
	return &Cfg4PeerManager {
		IP:				config.Local.IP,
		Port:			config.Local.TCP,
		ID:				config.Local.ID,
		MaxPeers:		config.MaxPeers,
		MaxOutbounds:	config.MaxOutbounds,
		MaxInBounds:	config.MaxInbounds,
		Statics:		config.StaticNodes,
		NoDial:			config.NoDial,
	}
}

//
// Get configuration op table manager
//
func P2pConfig4TabManager() *Cfg4TabManager {
	return &Cfg4TabManager {
		Local:			config.Local,
		BootstrapNodes:	config.BootstrapNodes,
		DataDir:		config.NodeDataDir,
		NodeDB:			config.NodeDatabase,
		BootstrapNode:	config.BootstrapNode,
	}
}

