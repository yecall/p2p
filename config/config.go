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
	"net"
	"crypto/ecdsa"
	"path/filepath"
	"os"
	"os/user"
	"runtime"
	"fmt"
	"path"
	yclog "ycp2p/logger"
)


//
// errno
//
type P2pCfgErrno int

const (
	PcfgEnoNone		= iota
	PcfgEnoParameter
	PcfgEnoPrivateKye
	PcfgEnoDataDir
	PcfgEnoDatabase
	PcfgEnoDataListenAddr
	PcfgEnoIpAddr
	PcfgEnoUnknown
)

//
// Some paths
//
const (
	datadirPrivateKey      = "nodekey"            // Path within the datadir to the node's private key
	datadirDefaultKeyStore = "keystore"           // Path within the datadir to the keystore
	datadirStaticNodes     = "static-nodes.json"  // Path within the datadir to the static node list
	datadirTrustedNodes    = "trusted-nodes.json" // Path within the datadir to the trusted node list
	datadirNodeDatabase    = "nodes"              // Path within the datadir to store the node infos
)

//
// Boot nodes
//
var MainnetBootnodes = []string{
	"ynode://xxx...@127.0.0.1:30303",
}

//
// Node ID length in bits
//
const NodeIDBits	= 512
const NodeIDBytes	= NodeIDBits/8

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
const MaxInbounds	= MaxPeers/2
const MaxOutbounds	= MaxPeers/2



//
// Node
//
type Node struct {
	IP			net.IP // ip address
	UDP, TCP	uint16 // port numbers
	ID			NodeID // the node's public key
}

//
// Node static Configuration parameters
//
type Config struct {
	Version			string				// p2p version
	PrivateKey		*ecdsa.PrivateKey	// node private key
	MaxPeers		int					// max peers can be
	MaxInbounds		int					// max peers for inbound concurrency establishing can be
	MaxOutbounds	int					// max peers for outbound concurrency establishing can be
	Name			string				// node name
	BootstrapNodes	[]*Node				// bootstrap nodes
	StaticNodes		[]*Node				// static nodes
	TrustedNodes	[]*Node				// trusted nodes
	NodeDataDir		string				// node data directory
	NodeDatabase	string				// node database
	ListenAddr		string				// address listent
	NoDial			bool				// outboundless flag
	Local			Node				// myself
}

//
// Configuration about neighbor listener on UDP
//
type Cfg4UdpListener struct {
	IP		net.IP	// ip address
	Port	uint16	// port numbers
	ID		NodeID	// the node's public key
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
	IP				net.IP	// ip address
	UdpPort			uint16	// udp port number
	TcpPort			uint16	// tcp port number
	ID				NodeID	// the node's public key
	DataDir			string	// data directory
	NodeDB			string	// node database
}

//
// Default configuration(notice that it's not a validated configuration and
// could never be applied), most of those defaults must be overided by higher
// lever module of system.
//
var dftLocal = Node {
	IP:		nil,
	UDP:	0,
	TCP:	0,
	ID:		nil,
}

var config = Config {
	PrivateKey:			nil,
	MaxPeers:			MaxPeers,
	MaxInbounds:		MaxInbounds,
	MaxOutbounds:		MaxOutbounds,
	Name:				"yeeco.node",
	BootstrapNodes:		nil,
	StaticNodes:		nil,
	NodeDataDir:		P2pDefaultDataDir(),
	NodeDatabase:		"node.db",
	ListenAddr:			"*:0",
	Local:				dftLocal,
}

//
// Setup local node identity
//
func p2pSetupLocalNodeId() P2pCfgErrno {
	return PcfgEnoNone
}

//
// P2pConfig
//
func P2pConfig(cfg *Config) P2pCfgErrno {

	//
	// update
	//

	if cfg == nil {
		yclog.LogCallerFileLine("P2pConfig: invalid configuration")
		return PcfgEnoParameter
	}
	config = *cfg

	//
	// check configuration
	//

	if config.PrivateKey == nil {
		yclog.LogCallerFileLine("P2pConfig: invalid private key")
		return PcfgEnoPrivateKye
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

	if len(config.NodeDataDir) == 0 || path.IsAbs(config.NodeDataDir) == false {
		yclog.LogCallerFileLine("P2pConfig: invaid data directory")
		return PcfgEnoDataDir
	}

	if len(config.NodeDatabase) == 0 {
		yclog.LogCallerFileLine("P2pConfig: invalid database name")
		return PcfgEnoDatabase
	}

	if len(config.ListenAddr) == 0 {
		yclog.LogCallerFileLine("P2pConfig: invalid listen address")
		return PcfgEnoDataListenAddr
	}

	if config.Local.IP == nil {
		yclog.LogCallerFileLine("P2pConfig: invalid ip address")
		return PcfgEnoIpAddr
	}

	return PcfgEnoNone
}

//
// Node identity to string
//
func P2pNodeId2HexString(id NodeID) string {
	var str = ""
	for _, b := range id {
		str = str + fmt.Sprintf("%02x", b)
	}
	return str
}

//
// Get default data directory
//
func P2pDefaultDataDir() string {
	home := P2pGetUserHomeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "yee")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "Yee")
		} else {
			return filepath.Join(home, ".yee")
		}
	}
	return ""
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

//
// Get configuration of neighbor discovering listener
//
func P2pConfig4UdpListener() *Cfg4UdpListener {

	//
	// local configuration must be completed firstly before
	// calling this function
	//

	return &Cfg4UdpListener {
		IP:		config.Local.IP,
		Port:	config.Local.UDP,
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
		IP:				config.Local.IP,
		UdpPort:		config.Local.UDP,
		TcpPort:		config.Local.TCP,
		ID:				config.Local.ID,
		DataDir:		config.NodeDataDir,
		NodeDB:			config.NodeDatabase,
	}
}

