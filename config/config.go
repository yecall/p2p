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
)


//
// errno
//
type P2pCfgErrno int

const (
	PcfgEnoNone		= iota
	PcfgEnoUnknown
	PcfgEnoMax
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
const NodeIDBits = 512

//
// Node identity
//
type NodeID [NodeIDBits/8]byte

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
	BootstrapNodes	[]*NodeID			// bootstrap nodes
	StaticNodes		[]*NodeID			// static nodes
	TrustedNodes	[]*NodeID			// trusted nodes
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
	Port			uint16	// port numbers
	ID				NodeID	// the node's public key
	MaxPeers		int		// max peers would be
	MaxOutbounds	int		// max concurrency outbounds
	MaxInBounds		int		// max concurrency inbounds
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
	TrustedNodes:		nil,
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
	}
}
