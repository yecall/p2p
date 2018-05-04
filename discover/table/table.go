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
	"time"
	"path"
	"math/rand"
	"crypto/sha256"
	sch		"ycp2p/scheduler"
	ycfg	"ycp2p/config"
	um		"ycp2p/discover/udpmsg"
	yclog	"ycp2p/logger"
)


//
// errno
//
const (
	TabMgrEnoNone		= iota
	TabMgrEnoConfig
	TabMgrEnoParameter
	TabMgrEnoScheduler
	TabMgrEnoDatabase
	TabMgrEnoNotFound
	TabMgrEnoInternal
	TabMgrEnoFindNodeFailed
	TabMgrEnoPingpongFailed
	TabMgrEnoTimeout
)

type TabMgrErrno int

//
// Hash type
//
const HashLength = 32				// 32 bytes(256 bits) hash applied
const HashBits = HashLength * 8		// bits number of hash
type Hash [HashLength]byte

//
// Some constants about database(levelDb)
//
const (
	ndbVersion = 4
)

//
// Some constants about buckets, timers, ...
//
const (
	bucketSize			= 16					// max nodes can be held for one bucket
	nBuckets			= HashBits + 1			// total number of buckets
	maxBonding			= 16					// max concurrency bondings
	maxFindnodeFailures	= 5						// max FindNode failures to remove a node
	autoRefreshCycle	= 1 * time.Hour			// period to auto refresh
	findNodeExpiration	= 21 * time.Second		// should be (NgbProtoFindNodeResponseTimeout + delta)
	pingpongExpiration	= 21 * time.Second		// should be (NgbProtoPingResponseTimeout + delta)
	seedCount           = 32					// wanted number of seeds
	seedMaxAge          = 5 * 24 * time.Hour	// max age can seeds be
	nodeExpiration		= 24 * time.Hour		// Time after which an unseen node should be dropped.
	nodeCleanupCycle	= time.Hour				// Time period for running the expiration task.
)

//
// Bucket entry
//
type NodeID ycfg.NodeID

type Node struct {
	ycfg.Node			// our Node type
	sha			Hash	// hash from node identity
}

type bucketEntry struct {
	ycfg.Node			// node
	sha			Hash	// hash of id
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
	local			ycfg.Node	// local node identity
	bootstrapNodes	[]*Node		// bootstrap nodes
	dataDir			string		// data directory
	nodeDb			string		// node database
	bootstratNode	bool		// bootstrap node flag
}

//
// Instance control block
//
const (
	TabInstStateNull	= iota	// null instance state
	TabInstStateQuering			// FindNode sent but had not been responsed yet
	TabInstStateBonding			// Ping sent but hand not been responsed yet
	TabInstStateQTimeout		// query timeout
	TabInstStateBTimeout		// bound timeout
)

const (
	TabInstQPendingMax	= 16		// max nodes in pending for quering
	TabInstBPendingMax	= 128		// max nodes in pending for bounding
	TabInstQueringMax	= 8			// max concurrency quering instances
	TabInstBondingMax	= 64		// max concurrency bonding instances
)

type instCtrlBlock struct {
	state	int				// instance state, see aboved consts about state pls
	req		interface{}		// request message pointer which inited this instance
	rsp		interface{}		// pointer to response message received
	tid		int				// identity of timer for response
}

//
// Table manager
//
const TabMgrName = sch.TabMgrName

type tableManager struct {
	name			string						// name
	tep				sch.SchUserTaskEp			// entry
	cfg				tabConfig					// configuration
	ptnMe			interface{}					// pointer to task node of myself
	ptnNgbMgr		interface{}					// pointer to neighbor manager task node
	ptnDcvMgr		interface{}					// pointer to discover manager task node
	shaLocal		Hash						// hash of local node identity
	buckets			[nBuckets]*bucket			// buckets
	queryIcb		[]*instCtrlBlock			// active query instance table
	boundIcb		[]*instCtrlBlock			// active bound instance table
	queryPending	[]*Node						// pending to be queried
	boundPending	[]*Node						// pending to be bound
	dlkTab			[]int						// log2 distance lookup table for a byte
	refreshing		bool						// busy in refreshing now
	dataDir			string						// data directory
	nodeDb			*nodeDB						// node database object pointer
}

var tabMgr = tableManager{
	name:	TabMgrName,
	tep:	TabMgrProc,
}

//
// Table manager entry
//
func TabMgrProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	yclog.LogCallerFileLine("TabMgrProc: scheduled, msg: %d", msg.Id)

	var eno TabMgrErrno = TabMgrEnoNone

	switch msg.Id {

	case sch.EvSchPoweron:
		eno = TabMgrPoweron(ptn)

	case sch.EvSchPoweroff:
		eno = TabMgrPoweroff(ptn)

	case sch.EvTabRefreshTimer:
		eno = TabMgrRefreshTimerHandler()

	case sch.EvTabPingpongTimer:
		eno = TabMgrPingpongTimerHandler(msg.Body.(*instCtrlBlock))

	case sch.EvTabFindNodeTimer:
		eno = TabMgrFindNodeTimerHandler(msg.Body.(*instCtrlBlock))

	case sch.EvTabRefreshReq:
		eno = TabMgrRefreshReq(msg.Body.(*sch.MsgTabRefreshReq))

	case sch.EvNblFindNodeRsp:
		eno = TabMgrFindNodeRsp(msg.Body.(*sch.NblFindNodeRsp))

	case sch.EvNblPingpongRsp:
		eno = TabMgrPingpongRsp(msg.Body.(*sch.NblPingRsp))

	default:
		yclog.LogCallerFileLine("TabMgrProc: invalid message: %d", msg.Id)
		return sch.SchEnoUserTask
	}

	if eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrProc: errors, eno: %d", eno)
		return sch.SchEnoUserTask
	}

	return sch.SchEnoNone
}

//
// Poweron handler
//
func TabMgrPoweron(ptn interface{}) TabMgrErrno {

	var eno TabMgrErrno = TabMgrEnoNone

	//
	// fetch configurations
	//

	if eno = tabGetConfig(&tabMgr.cfg); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("NdbcPoweron: tabGetConfig failed, eno: %d", eno)
		return eno
	}

	//
	// prepare node database
	//

	if eno = tabNodeDbPrepare(); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("NdbcPoweron: tabNodeDbPrepare failed, eno: %d", eno)
		return eno
	}

	//
	// build local node identity hash for neighbors finding
	//

	if eno = tabSetupLocalHashId(); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("NdbcPoweron: tabSetupLocalHash failed, eno: %d", eno)
		return eno
	}

	//
	// preapare related task ponters
	//

	if eno = tabRelatedTaskPrepare(ptn); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("NdbcPoweron: tabRelatedTaskPrepare failed, eno: %d", eno)
		return eno
	}

	//
	// setup the lookup table
	//

	if eno = tabSetupLog2DistanceLookupTable(tabMgr.dlkTab); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("NdbcPoweron: tabSetupLog2DistanceLookupTable failed, eno: %d", eno)
		return eno
	}

	//
	// setup auto-refresh timer
	//

	if eno = tabStartTimer(nil, sch.TabRefreshTimerId, autoRefreshCycle); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("NdbcPoweron: tabRefresh failed, eno: %d", eno)
		return eno
	}

	//
	// Since the system is just powered on at this moment, we start table
	// refreshing at once. Before dong this, we update the random seed for
	// the underlying.
	//

	rand.Seed(time.Now().UnixNano())
	tabMgr.refreshing = false

	if eno = tabRefresh(nil); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("NdbcPoweron: tabRefresh failed, eno: %d", eno)
		return eno
	}

	return TabMgrEnoNone
}

//
// Poweroff handler
//
func TabMgrPoweroff(ptn interface{})TabMgrErrno {
	if tabMgr.nodeDb != nil {
		tabMgr.nodeDb.close()
		tabMgr.nodeDb = nil
	}

	if sch.SchinfTaskDone(ptn, sch.SchEnoKilled) == sch.SchEnoNone {
		yclog.LogCallerFileLine("TabMgrPoweroff: done task failed")
		return TabMgrEnoNone
	}

	yclog.LogCallerFileLine("TabMgrPoweroff: task done")
	return TabMgrEnoScheduler
}

//
// Auto-Refresh timer handler
//
func TabMgrRefreshTimerHandler()TabMgrErrno {
	yclog.LogCallerFileLine("TabMgrPoweroff: atuo refresh timer expired, refresh table ...")
	return tabRefresh(nil)
}

//
// Pingpong timer expired event handler
//
func TabMgrPingpongTimerHandler(inst *instCtrlBlock) TabMgrErrno {

	yclog.LogCallerFileLine("TabMgrPingpongTimerHandler: timer expired")

	//
	// update database for the neighbor node
	//

	inst.state = TabInstStateBTimeout
	if eno := tabUpdateNodeDb(inst, TabMgrEnoTimeout); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPingpongTimerHandler: tabUpdateNodeDb failed, eno: %d", eno)
		return eno
	}

	//
	// update buckets
	//

	if eno := tabUpdateBucket(inst, TabMgrEnoTimeout); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPingpongTimerHandler: tabUpdateBucket failed, eno: %d", eno)
		return eno
	}

	//
	// delete the active instance
	//

	if eno := tabDeleteActiveBoundInst(inst); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPingpongTimerHandler: tabDeleteActiveQueryInst failed, eno: %d", eno)
		return eno
	}

	//
	// try to active more query instances
	//

	if eno := tabActiveBoundInst(); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPingpongTimerHandler: tabActiveQueryInst failed, eno: %d", eno)
		return eno
	}

	return TabMgrEnoNone
}

//
// FindNode timer expired event handler
//
func TabMgrFindNodeTimerHandler(inst *instCtrlBlock) TabMgrErrno {

	yclog.LogCallerFileLine("TabMgrFindNodeTimerHandler: timer expired")

	//
	// update database for the neighbor node
	//

	inst.state = TabInstStateQTimeout
	if eno := tabUpdateNodeDb(inst, TabMgrEnoTimeout); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrFindNodeTimerHandler: tabUpdateNodeDb failed, eno: %d", eno)
		return eno
	}

	//
	// update buckets
	//

	if eno := tabUpdateBucket(inst, TabMgrEnoTimeout); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrFindNodeTimerHandler: tabUpdateBucket failed, eno: %d", eno)
		return eno
	}

	//
	// delete the active instance
	//

	if eno := tabDeleteActiveQueryInst(inst); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrFindNodeTimerHandler: tabDeleteActiveQueryInst failed, eno: %d", eno)
		return eno
	}

	//
	// try to active more query instances
	//

	if eno := tabActiveQueryInst(); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrFindNodeTimerHandler: tabActiveQueryInst failed, eno: %d", eno)
		return eno
	}

	return TabMgrEnoNone
}

//
// Refresh request handler
//
func TabMgrRefreshReq(msg *sch.MsgTabRefreshReq)TabMgrErrno {
	yclog.LogCallerFileLine("TabMgrRefreshReq: requst to refresh table ...")
	_ = msg
	return tabRefresh(nil)
}

//
// FindNode response handler
//
func TabMgrFindNodeRsp(msg *sch.NblFindNodeRsp)TabMgrErrno {

	yclog.LogCallerFileLine("TabMgrRefreshReq: FindNode response received");

	//
	// lookup active instance for the response
	//

	var inst *instCtrlBlock = nil

	inst = tabFindInst(&msg.FindNode.To, TabInstStateBonding)
	if inst == nil {
		yclog.LogCallerFileLine("TabMgrFindNodeRsp: instance not found")
		return TabMgrEnoNotFound
	}

	//
	// obtain result
	//

	var result = msg.Result
	if result != 0 { result = TabMgrEnoFindNodeFailed }

	//
	// update database for the neighbor node
	//

	if eno := tabUpdateNodeDb(inst, result); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrFindNodeRsp: tabUpdateNodeDb failed, eno: %d", eno)
		return eno
	}

	//
	// update buckets
	//

	if eno := tabUpdateBucket(inst, result); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrFindNodeRsp: tabUpdateBucket failed, eno: %d", eno)
		return eno
	}

	//
	// delete the active instance
	//

	if eno := tabDeleteActiveQueryInst(inst); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrFindNodeRsp: tabDeleteActiveQueryInst failed, eno: %d", eno)
		return eno
	}

	//
	// try to active more query instances
	//

	if eno := tabActiveQueryInst(); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrFindNodeRsp: tabActiveQueryInst failed, eno: %d", eno)
		return eno
	}

	//
	// check result reported
	//

	if msg.Result != 0 {
		yclog.LogCallerFileLine("TabMgrFindNodeRsp: fail reported, result: %d", msg.Result)
		return TabMgrEnoNone
	}

	//
	// deal with neighbors reported
	//

	for _, node := range msg.Neighbors.Nodes {
		if eno := tabAddPendingBoundInst(node); eno != TabMgrEnoNone {
			yclog.LogCallerFileLine("TabMgrFindNodeRsp: tabAddPendingBoundInst failed, eno: %d", eno)
			break;
		}
	}

	//
	// try to active more BOUND instances
	//

	if eno := tabActiveBoundInst(); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrFindNodeRsp: tabActiveBoundInst failed, eno: %d", eno)
		return eno
	}

	return TabMgrEnoNone
}

//
// Pingpong respone handler
//
func TabMgrPingpongRsp(msg *sch.NblPingRsp) TabMgrErrno {

	//
	// lookup active instance for the response. Notice: some respons without actived
	// instances might be sent here, see file neighbor.go please. To speed up the p2p
	// network, one might push those nodes into buckets and node database, but now in
	// current implement, except that the local node is a bootstrap node, we discard
	// all response without an actived instance.
	//

	var inst *instCtrlBlock = nil

	inst = tabFindInst(&msg.Ping.To, TabInstStateQuering)
	if inst == nil {
		if tabMgr.cfg.bootstratNode == false {
			yclog.LogCallerFileLine("TabMgrPingpongRsp: instance not found and local is not a bootstrap node")
			return TabMgrEnoNotFound
		}
		return tabUpdateBootstarpNode(&msg.Pong.From)
	}

	//
	// obtain result
	//

	var result = msg.Result
	if result != 0 { result = TabMgrEnoPingpongFailed }

	//
	// update database for the neighbor node
	//

	if eno := tabUpdateNodeDb(inst, result); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPingpongRsp: tabUpdateNodeDb failed, eno: %d", eno)
		return eno
	}

	//
	// update buckets
	//

	if eno := tabUpdateBucket(inst, result); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPingpongRsp: tabUpdateBucket failed, eno: %d", eno)
		return eno
	}


	//
	// delete the active instance
	//

	if eno := tabDeleteActiveBoundInst(inst); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPingpongRsp: tabDeleteActiveQueryInst failed, eno: %d", eno)
		return eno
	}

	//
	// try to active more BOUND instances
	//

	if eno := tabActiveBoundInst(); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPingpongRsp: tabActiveBoundInst failed, eno: %d", eno)
		return eno
	}

	//
	// check result reported
	//

	if msg.Result != 0 {
		yclog.LogCallerFileLine("TabMgrPingpongRsp: fail reported, result: %d", msg.Result)
		return TabMgrEnoNone
	}

	//
	// response to the discover manager task
	//

	if eno := tabDiscoverResp(&msg.Pong.From); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPingpongRsp: tabDiscoverResp failed, eno: %d", eno)
		return eno
	}

	return TabMgrEnoNone
}

//
// Static task to clean the node database
//
const NdbcName = "ndbCleaner"

type nodeDbCleaner struct {
	name	string				// name
	tep		sch.SchUserTaskEp	// entry point
	tid		int					// cleaner timer
}

var ndbCleaner = nodeDbCleaner{
	name:	NdbcName,
	tep:	NdbcProc,
	tid:	sch.SchInvalidTid,
}

//
// NodeDb cleaner entry
//
func NdbcProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	yclog.LogCallerFileLine("NdbcProc: scheduled, msg id: %d", msg.Id)

	var eno TabMgrErrno

	switch msg.Id {

	case sch.EvSchPoweron:
		eno = NdbcPoweron(ptn)

	case sch.EvSchPoweroff:
		eno = NdbcPoweroff(ptn)

	case sch.EvNdbCleanerTimer:
		eno = NdbcAutoCleanTimerHandler()

	default:
		yclog.LogCallerFileLine("NdbcProc: invalid message: %d", msg.Id)
		return sch.SchEnoInternal
	}

	if eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("NdbcProc: errors, eno: %d", eno)
		return sch.SchEnoUserTask
	}

	return sch.SchEnoNone
}

//
// Pwoeron handler
//
func NdbcPoweron(ptn interface{}) TabMgrErrno {

	var tmd  = sch.TimerDescription {
		Name:	"",
		Utid:	0,
		Tmt:	sch.SchTmTypeAbsolute,
		Dur:	nodeCleanupCycle,
		Extra:	nil,
	}

	var (
		eno	sch.SchErrno
		tid int
	)

	if eno, tid = sch.SchInfSetTimer(ptn, &tmd); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("NdbcPoweron: set timer failed, eno: %d", eno)
		return TabMgrEnoScheduler
	}
	ndbCleaner.tid = tid

	return TabMgrEnoNone
}

//
// Poweroff handler
//
func NdbcPoweroff(ptn interface{}) TabMgrErrno {

	if ndbCleaner.tid != sch.SchInvalidTid {
		if eno := sch.SchinfKillTimer(ptn, ndbCleaner.tid); eno != sch.SchEnoNone {
			yclog.LogCallerFileLine("NdbcPoweroff: SchinfKillTimer failed, eno: %d", eno)
			return TabMgrEnoScheduler
		}
		ndbCleaner.tid = sch.SchInvalidTid
	}

	if eno := sch.SchinfTaskDone(ptn, sch.SchEnoKilled); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("NdbcPoweroff: SchinfTaskDone failed, eno: %d", eno)
		return TabMgrEnoScheduler
	}

	return TabMgrEnoNone
}

//
// Auto clean timer handler
//
func NdbcAutoCleanTimerHandler() TabMgrErrno {
	return TabMgrEnoNone
}

//
// Fetch configuration
//
func tabGetConfig(tabCfg *tabConfig) TabMgrErrno {

	if tabCfg == nil {
		yclog.LogCallerFileLine("tabGetConfig: invalid parameter(s)")
		return TabMgrEnoParameter
	}

	cfg := ycfg.P2pConfig4TabManager()
	if cfg == nil {
		yclog.LogCallerFileLine("tabGetConfig: P2pConfig4TabManager failed")
		return TabMgrEnoConfig
	}

	tabCfg.local			= cfg.Local
	tabCfg.dataDir			= cfg.DataDir
	tabCfg.nodeDb			= cfg.NodeDB
	tabCfg.bootstratNode	= cfg.BootstrapNode

	return TabMgrEnoNone
}

//
// Prepare node database when poweron
//
func tabNodeDbPrepare() TabMgrErrno {
	if tabMgr.nodeDb == nil {
		yclog.LogCallerFileLine("tabNodeDbPrepare: node database had been opened")
		return TabMgrEnoDatabase
	}

	dbPath := path.Join(tabMgr.cfg.dataDir, tabMgr.cfg.nodeDb)
	db, err := newNodeDB(dbPath, ndbVersion, NodeID(tabMgr.cfg.local.ID))
	if err != nil {
		yclog.LogCallerFileLine("tabNodeDbPrepare: newNodeDB failed, err: %s", err.Error())
		return TabMgrEnoDatabase
	}
	tabMgr.nodeDb = db

	return TabMgrEnoNone
}

//
// Node identity to sha
//
func tabNodeId2Hash(id NodeID) *Hash {
	h := sha256.Sum256(tabMgr.cfg.local.ID[:])
	return (*Hash)(&h)
}

//
// Setup local node id hash
//
func tabSetupLocalHashId() TabMgrErrno {
	if cap(tabMgr.shaLocal) != 32 {
		yclog.LogCallerFileLine("tabSetupLocalHashId: hash identity should be 32 bytes")
		return TabMgrEnoParameter
	}
	var h = tabNodeId2Hash(NodeID(tabMgr.cfg.local.ID))
	tabMgr.shaLocal = *h
	return TabMgrEnoNone
}

//
// Prepare pointers to related tasks
//
func tabRelatedTaskPrepare(ptnMe interface{}) TabMgrErrno {
	var eno = sch.SchEnoNone
	tabMgr.ptnMe = ptnMe
	if eno, tabMgr.ptnNgbMgr = sch.SchinfGetTaskNodeByName(sch.NgbMgrName); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("tabRelatedTaskPrepare: " +
			"get task node failed, name: %s", sch.NgbMgrName)
		return TabMgrEnoScheduler
	}
	if eno, tabMgr.ptnDcvMgr = sch.SchinfGetTaskNodeByName(sch.DcvMgrName); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("tabRelatedTaskPrepare: " +
			"get task node failed, name: %s", sch.DcvMgrName)
		return TabMgrEnoScheduler
	}
	if tabMgr.ptnMe == nil || tabMgr.ptnNgbMgr == nil || tabMgr.ptnDcvMgr == nil {
		yclog.LogCallerFileLine("tabRelatedTaskPrepare: invaid task node pointer")
		return TabMgrEnoInternal
	}
	return TabMgrEnoNone
}

//
// Setup lookup table for bytes
//
func tabSetupLog2DistanceLookupTable(lkt []int) TabMgrErrno {
	var n uint
	var b uint
	lkt[0] = 8
	for n = 0; n < 8; n++ {
		for b = 1<<n; b < 1<<(n+1); b++ {
			lkt[b] = int(8 - n - 1);
		}
	}
	return TabMgrEnoNone
}

//
// Init a refreshing procedure
//
func tabRefresh(tid *NodeID) TabMgrErrno {

	//
	// If we are in refreshing, return at once. When the pending table for query
	// is empty, this flag is set to false.
	//

	if tabMgr.refreshing == true {
		yclog.LogCallerFileLine("tabRefresh: already in refreshing")
		return TabMgrEnoNone
	}

	var nodes []*Node
	var target NodeID

	if tid == nil {
		rand.Read(target[:])
	} else {
		target = *tid
	}

	if nodes = tabClosest(target, TabInstQPendingMax); len(nodes) == 0 {
		seeds := tabSeedsFromDb(TabInstQPendingMax)
		nodes = append(nodes, tabMgr.cfg.bootstrapNodes...)
		nodes = append(nodes, seeds...)
		if len(nodes) > TabInstQPendingMax {
			nodes = nodes[:TabInstQPendingMax]
		}
	}

	var eno TabMgrErrno

	if eno := tabQuery(&target, nodes); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("tabRefresh: tabQuery failed, eno: %d", eno)
	} else {
		tabMgr.refreshing = true
	}

	return eno
}

//
// Caculate the distance between two nodes
//
func tabLog2Dist(h1 Hash, h2 Hash) int {
	var d = 0
	for i, b := range h2 {
		x := h1[i] ^ b
		d += tabMgr.dlkTab[x]
		if x != 8 {
			break
		}
	}
	return d
}

//
// Get nodes closest to target
//
func tabClosest(target NodeID, size int) []*Node {

	//
	// Notice: in this function, we got []*Node with a approximate order,
	// since we do not sort the nodes in the first bank, see bellow pls.
	//

	var closest = []*Node{}
	var count = 0

	ht := tabNodeId2Hash(target)
	dt := tabLog2Dist(tabMgr.shaLocal, *ht)

	var addClosest = func (bk *bucket) int {
		count = cap(closest)
		if bk := tabMgr.buckets[dt]; bk != nil {
			for _, ne := range bk.nodes {
				closest = append(closest, &Node {
					Node:	ne.Node,
					sha:	ne.sha,
				})
				if count++; count >= size {
					break
				}
			}
		}
		return count
	}

	//
	// the most closest bank: one should sort nodes in this bank if accurate
	// order by log2 distance against the target node is expected, but we not.
	//

	if bk := tabMgr.buckets[dt]; bk != nil {
		if addClosest(bk) >= size {
			return closest
		}
	}

	//
	// the second closest bank
	//

	for loop := dt + 1; loop < cap(tabMgr.buckets); loop++ {
		if bk := tabMgr.buckets[loop]; bk != nil {
			if addClosest(bk) >= size {
				return closest
			}
		}
	}

	if dt <= 0 { return closest }

	//
	// the last bank
	//

	for loop := dt - 1; loop >= 0; loop-- {
		if bk := tabMgr.buckets[loop]; bk != nil {
			if addClosest(bk) >= size {
				return closest
			}
		}
	}

	return closest
}

//
// Fetch seeds from node database
//
func tabSeedsFromDb(size int) []*Node {

	nodes := tabMgr.nodeDb.querySeeds(seedCount, seedMaxAge)

	if nodes == nil {
		yclog.LogCallerFileLine("tabSeedsFromDb: nil nodes")
		return nil
	}
	if len(nodes) == 0 {
		yclog.LogCallerFileLine("tabSeedsFromDb: empty node table")
		return nil
	}

	return nodes
}

//
// Query nodes
//
func tabQuery(target *NodeID, nodes []*Node) TabMgrErrno {

	//
	// check: since we apply doing best to active more, it's impossible that the active
	// table is not full while the pending table is not empty.
	//

	remain := len(nodes)
	actNum := len(tabMgr.queryIcb)
	pndNum := len(tabMgr.queryPending)

	if remain == 0 {
		yclog.LogCallerFileLine("tabQuery: invalid parameters, no node to be handled")
		return TabMgrEnoParameter
	}

	if actNum < TabInstQueringMax && pndNum != 0 {
		yclog.LogCallerFileLine("tabQuery: " +
			"internal errors, active: %d, pending: %d",
			actNum, pndNum)
		return TabMgrEnoInternal
	}

	//
	// create query instances
	//

	var schMsg = sch.SchMessage{}
	var nidx = 0

	if actNum < TabInstQueringMax {

		for loop := actNum; loop < remain && loop < TabInstQueringMax - actNum; loop++ {

			msg := new(um.FindNode)
			icb := new(instCtrlBlock)

			icb.state	= TabInstStateQuering
			icb.req		= msg
			icb.rsp		= nil
			icb.tid		= sch.SchInvalidTid

			msg.From = um.Node{
				IP:		tabMgr.cfg.local.IP,
				UDP:	tabMgr.cfg.local.UDP,
				TCP:	tabMgr.cfg.local.TCP,
				NodeId:	tabMgr.cfg.local.ID,
			}
			msg.To = um.Node{
				IP:     nodes[nidx].IP,
				UDP:    nodes[nidx].UDP,
				TCP:    nodes[nidx].TCP,
				NodeId: nodes[nidx].ID,
			}
			msg.Target		= ycfg.NodeID(*target)
			msg.Id			= uint64(time.Now().UnixNano())
			msg.Expiration	= 0
			msg.Extra		= nil

			if eno := sch.SchinfMakeMessage(&schMsg, tabMgr.ptnMe, tabMgr.ptnNgbMgr, sch.EvNblFindNodeReq, msg);
				eno != sch.SchEnoNone {
				yclog.LogCallerFileLine("tabQuery: SchinfMakeMessage failed, eno: %d", eno)
				return TabMgrEnoScheduler
			}

			if eno := sch.SchinfSendMsg2Task(&schMsg); eno != sch.SchEnoNone {
				yclog.LogCallerFileLine("tabQuery: SchinfSendMsg2Task failed, eno: %d", eno)
				return TabMgrEnoScheduler
			}

			if eno := tabStartTimer(icb, sch.TabFindNodeTimerId, findNodeExpiration); eno != TabMgrEnoNone {
				yclog.LogCallerFileLine("tabQuery: tabRefresh failed, eno: %d", eno)
				return eno
			}

			tabMgr.queryIcb[loop] = icb
			nidx++
		}
	}

	//
	// append nodes to pending table if any
	//

	for ; nidx < remain; nidx++ {
		pidx := len(tabMgr.queryPending)
		if  pidx >= TabInstQPendingMax {
			break
		}
		tabMgr.queryPending[pidx] = nodes[nidx]
	}

	return TabMgrEnoNone
}

//
// Find active instance by node
//
func tabFindInst(node *um.Node, state int) *instCtrlBlock {

	if node == nil || (state != TabInstStateQuering &&
						state != TabInstStateBonding &&
						state != TabInstStateQTimeout &&
						state != TabInstStateBTimeout) {
		yclog.LogCallerFileLine("tabFindInst: invalid parameters")
		return nil
	}

	if state == TabInstStateQuering || state == TabInstStateQTimeout {
		for _, icb := range tabMgr.queryIcb {
			req := icb.req.(*um.FindNode)
			if req.To.CompareWith(node) == um.CmpNodeEqu {
				return icb
			}
		}
	} else {
		for _, icb := range tabMgr.boundIcb {
			req := icb.req.(*um.Ping)
			if req.To.CompareWith(node) == um.CmpNodeEqu {
				return icb
			}
		}
	}

	yclog.LogCallerFileLine("tabFindInst: " +
		"node not found, id: %s",
		ycfg.P2pNodeId2HexString(node.NodeId))

	return nil
}

//
// Update node database
//
func tabUpdateNodeDb(inst *instCtrlBlock, result int) TabMgrErrno {

	//
	// The logic:
	// 1) Update sending ping request time;
	// 2) Update receiving pong response time if we receive it indeed;
	// 3) Update find node failed counter;
	// 4) When pingpong ok, init the peer node record totally;
	// 5) When pingpong failed, if the node had been recorded in database, clear
	// the find node failed counter to be zero;
	//
	// Notice: in current implement, peer node records would be removed from node
	// database just by cleaner task, which checks if conditions are fullfilled to
	// do that, see it pls.
	//

	var fnFailUpdate = func() TabMgrErrno {
		id := NodeID(inst.req.(*um.FindNode).To.NodeId)
		fails := tabMgr.nodeDb.findFails(id) + 1
		if err := tabMgr.nodeDb.updateFindFails(id, fails); err != nil {
			yclog.LogCallerFileLine("tabUpdateNodeDb: updateFindFails failed, err: %s", err.Error())
			return TabMgrEnoDatabase
		}
		return TabMgrEnoNone
	}

	var fnFailClear = func () TabMgrErrno {
		id := NodeID(inst.req.(*um.FindNode).To.NodeId)
		if node := tabMgr.nodeDb.node(id); node == nil {
			yclog.LogCallerFileLine("tabUpdateNodeDb: node not exist, do nothing")
			return TabMgrEnoNone
		}
		if err := tabMgr.nodeDb.updateFindFails(id, 0); err != nil {
			yclog.LogCallerFileLine("tabUpdateNodeDb: updateFindFails failed, err: %s", err.Error())
			return TabMgrEnoDatabase
		}
		return TabMgrEnoNone
	}

	var update = func() TabMgrErrno {
		umn := inst.req.(*um.Ping).To
		n := Node{
			sha: *tabNodeId2Hash(NodeID(umn.NodeId)),
			Node: ycfg.Node{
				IP:  umn.IP,
				UDP: umn.UDP,
				TCP: umn.TCP,
				ID:  ycfg.NodeID(umn.NodeId),
			},
		}
		if err := tabMgr.nodeDb.updateNode(&n); err != nil {
			yclog.LogCallerFileLine("tabUpdateNodeDb: updateNode failed, err: %s", err.Error())
			return TabMgrEnoDatabase
		}
		return TabMgrEnoNone
	}

	switch {
	case inst.state == TabInstStateQuering && result == TabMgrEnoNone:
		yclog.LogCallerFileLine("tabUpdateNodeDb: need not to do anything")
		return TabMgrEnoNone

	case inst.state == TabInstStateQuering && result == TabMgrEnoFindNodeFailed:
		return fnFailUpdate()

	case inst.state == TabInstStateQuering && result == TabMgrEnoTimeout:
		return fnFailUpdate()

	case inst.state == TabInstStateBonding && result == TabMgrEnoNone:
		return update()

	case inst.state == TabInstStateBonding && result == TabMgrEnoPingpongFailed:
		return fnFailClear()

	case inst.state == TabInstStateBonding && result == TabMgrEnoTimeout:
		return fnFailClear()

	default:
		yclog.LogCallerFileLine("tabUpdateNodeDb: " +
			"invalid context, update nothing, state: %d, result: %d",
			inst.state, result)
		return TabMgrEnoInternal
	}

	yclog.LogCallerFileLine("tabUpdateNodeDb: should never come here")
	return TabMgrEnoInternal
}

//
// Update buckets
//
func tabUpdateBucket(inst *instCtrlBlock, result int) TabMgrErrno {

	//
	// The logic:
	// 1) When pingpong ok, add peer node to a bucket;
	// 2) When pingpong failed, add peer node to a bucket;
	// 3) When findnode failed counter reash the threshold, remove peer node from bucket;
	//

	switch {
	case inst.state == TabInstStateQuering && result == TabMgrEnoNone:
		yclog.LogCallerFileLine("tabUpdateBucket: need not to do anything")
		return TabMgrEnoNone

	case inst.state == TabInstStateQuering && result == TabMgrEnoFindNodeFailed:
		id := NodeID(inst.req.(*um.FindNode).To.NodeId)
		if fails := tabMgr.nodeDb.findFails(id) + 1; fails >= maxFindnodeFailures {
			return tabBucketRemoveNode(id)
		}
		return TabMgrEnoNone

	case inst.state == TabInstStateQuering && result == TabMgrEnoTimeout:
		id := NodeID(inst.req.(*um.FindNode).To.NodeId)
		if fails := tabMgr.nodeDb.findFails(id) + 1; fails >= maxFindnodeFailures {
			return tabBucketRemoveNode(id)
		}
		return TabMgrEnoNone

	case inst.state == TabInstStateBonding && result == TabMgrEnoNone:
		id := NodeID(inst.req.(*um.Ping).To.NodeId)
		return tabBucketAddNode(id)

	case inst.state == TabInstStateBonding && result == TabMgrEnoPingpongFailed:
		id := NodeID(inst.req.(*um.Ping).To.NodeId)
		return tabBucketAddNode(id)

	case inst.state == TabInstStateBonding && result == TabMgrEnoTimeout:
		id := NodeID(inst.req.(*um.Ping).To.NodeId)
		return tabBucketAddNode(id)

	default:
		yclog.LogCallerFileLine("tabUpdateBucket: " +
			"invalid context, update nothing, state: %d, result: %d",
			inst.state, result)
		return TabMgrEnoInternal
	}

	yclog.LogCallerFileLine("tabUpdateBucket: should never come here")
	return TabMgrEnoInternal
}

//
// Update node database while local node is a bootstrap node for an unexcepeted
// bounding procedure: this procedure is inited by neighbor manager task when a
// Ping or Pong message recived without an according neighbor instance can be mapped
// it. In this case, the neighbor manager then carry the pingpong procedure (if Ping
// received, a Pong sent firstly), and when Pong recvied, it is sent to here the
// table manager task, see Ping, Pong handler in file neighbor.go for details pls.
//
func tabUpdateBootstarpNode(n *um.Node) TabMgrErrno {

	id := NodeID(n.NodeId)

	//
	// update node database
	//

	node := Node {
		sha: *tabNodeId2Hash(id),
		Node: ycfg.Node {
			IP:  n.IP,
			UDP: n.UDP,
			TCP: n.TCP,
			ID:  n.NodeId,
		},
	}

	if err := tabMgr.nodeDb.updateNode(&node); err != nil {
		yclog.LogCallerFileLine("tabUpdateBootstarpNode: updateNode failed, err: %s", err.Error())
		return TabMgrEnoDatabase
	}

	//
	// add to bucket
	//

	return tabBucketAddNode(id)
}

//
// Start timer according instance, timer type, and duration
//
func tabStartTimer(inst *instCtrlBlock, tmt int, dur time.Duration) TabMgrErrno {
	return TabMgrEnoNone
}

//
// Remove node from bucket
//
func tabBucketRemoveNode(id NodeID) TabMgrErrno {
	return TabMgrEnoNone
}

//
// Add node to bucket
//
func tabBucketAddNode(id NodeID) TabMgrErrno {
	return TabMgrEnoNone
}

//
// Delete active query instance
//
func tabDeleteActiveQueryInst(inst *instCtrlBlock) TabMgrErrno {
	return TabMgrEnoNone
}

//
// Active query instance
//
func tabActiveQueryInst() TabMgrErrno {
	return TabMgrEnoNone
}

//
// Delete active bound instance
//
func tabDeleteActiveBoundInst(inst *instCtrlBlock) TabMgrErrno {
	return TabMgrEnoNone
}

//
// Add pending bound instance for node
//
func tabAddPendingBoundInst(node *um.Node) TabMgrErrno {
	return TabMgrEnoNone
}

//
// Active bound instance
//
func tabActiveBoundInst() TabMgrErrno {
	return TabMgrEnoNone
}

//
// Send respone to discover task for a bounded node
//
func tabDiscoverResp(node *um.Node) TabMgrErrno {
	return TabMgrEnoNone
}