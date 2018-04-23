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


package table

import (
	"net"
	"time"
	sch		"ycp2p/scheduler"
	ycfg	"ycp2p/config"
	yclog	"ycp2p/logger"
)



//
// Hash type
//
const HashLength = 32				// 32 bytes(256 bits) hash applied
const HashBits = HashLength * 8		// bits number of hash
type Hash [HashLength]byte

//
// Some constants
//
const (
	bucketSize			= 16					// max nodes can be held for one bucket
	nBuckets			= HashBits + 1			// total number of buckets
	maxBonding			= 16					// max concurrency bondings
	maxFindnodeFailures	= 5						// max FindNode failures to remove a node
	autoRefreshCycle	= 1 * time.Hour			// period to auto refresh
	seedCount           = 30					// wanted number of seeds
	seedMaxAge          = 5 * 24 * time.Hour	// max age can seeds be
	nodeExpiration		= 24 * time.Hour		// Time after which an unseen node should be dropped.
	nodeCleanupCycle	= time.Hour				// Time period for running the expiration task.

)

//
// Bucket entry
//
type bucketEntry struct {
	ip		net.IP		// ip address
	udp		uint16		// UDP port number
	tcp		uint16		// TCP port number
	id		ycfg.NodeID // node identity: the public key
	sha		Hash		// hash of id
}

//
// bucket type
//
type bucket struct {
	nodes	[]*bucketEntry	// node table for a bucket
}

//
// Table task configuration
//
type tabConfig struct {
	ip		net.IP		// ip address
	udp		uint16		// UDP port number
	tcp		uint16		// TCP port number
	id		ycfg.NodeID // node identity: the public key
}

//
// Table manager
//
const TabMgrName = sch.TabMgrName

type tableManager struct {
	name		string				// name
	tep			sch.SchUserTaskEp	// entry
	cfg			tabConfig			// configuration
	shaLocal	Hash				// hash of local node identity
	buckets		[nBuckets]*bucket	// buckets
}

var dcvMgr = tableManager{
	name:	TabMgrName,
	tep:	TabMgrProc,
}

//
// Table manager entry
//
func TabMgrProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {
	yclog.LogCallerFileLine("TabMgrProc: scheduled, msg: %d", msg.Id)

	switch msg.Id {
	case sch.EvSchPoweron:
	case sch.EvSchPoweroff:
	case sch.EvTabRefreshTimer:
	case sch.EvTabRefreshReq:
	case sch.EvNblFindNodeRsp:
	case sch.EvNblPingpongRsp:
	default:
	}
	return sch.SchEnoNone
}

//
// Static task to clean the node database
//
const NdbcName = "ndbCleaner"

type nodeDbCleaner struct {
	name	string
	tep		sch.SchUserTaskEp
}

var ndbCleaner = nodeDbCleaner{
	name:	NdbcName,
	tep:	NdbcProc,
}

//
// NodeDb cleaner entry
//
func NdbcProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	yclog.LogCallerFileLine("NdbcProc: scheduled, msg id: %d", msg.Id)

	var eno sch.SchErrno

	switch msg.Id {
	case sch.EvSchPoweron:
	case sch.EvSchPoweroff:
	case sch.EvNdbCleanerTimer:
	default:
		yclog.LogCallerFileLine("NdbcProc: invalid message: %d", msg.Id)
		return sch.SchEnoInternal
	}

	return eno
}