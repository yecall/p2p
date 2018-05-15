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
	"fmt"
	"crypto/sha256"
	sch		"ycp2p/scheduler"
	ycfg	"ycp2p/config"
	um		"ycp2p/discover/udpmsg"
	yclog	"ycp2p/logger"
	"sync"
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
	TabMgrEnoResource
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
	nBuckets			= HashBits				// total number of buckets
	maxBonding			= 16					// max concurrency bondings
	maxFindnodeFailures	= 5						// max FindNode failures to remove a node
	autoRefreshCycle	= 1 * time.Hour			// period to auto refresh
	findNodeExpiration	= 21 * time.Second		// should be (NgbProtoFindNodeResponseTimeout + delta)
	pingpongExpiration	= 21 * time.Second		// should be (NgbProtoPingResponseTimeout + delta)
	seedMaxCount          = 32					// wanted number of seeds
	seedMaxAge          = 5 * 24 * time.Hour	// max age can seeds be
	nodeExpiration		= 24 * time.Hour		// Time after which an unseen node should be dropped.
	nodeReboundDuration	= 1 * time.Minute		// duration for a node to be rebound
	nodeAutoCleanCycle	= time.Hour				// Time period for running the expiration task.
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
	ycfg.Node				// node
	sha			Hash		// hash of id
	addTime		time.Time	// time when node added
	lastPing	time.Time	// time when node latest pinged
	lastPong	time.Time	// time when node pong latest received
	failCount	int			// fail to response find node request counter
}

//
// bucket type
//
type bucket struct {
	nodes []*bucketEntry	// node table for a bucket
}

//
// Table task configuration
//
type tabConfig struct {
	local			ycfg.Node	// local node identity
	bootstrapNodes	[]*Node		// bootstrap nodes
	dataDir			string		// data directory
	nodeDb			string		// node database
	bootstratNode	bool		// bootstrap flag of local node
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
	TabInstQPendingMax	= 16	// max nodes in pending for quering
	TabInstBPendingMax	= 128	// max nodes in pending for bounding
	TabInstQueringMax	= 8		// max concurrency quering instances
	TabInstBondingMax	= 64	// max concurrency bonding instances
)

type instCtrlBlock struct {
	state	int					// instance state, see aboved consts about state pls
	req		interface{}			// request message pointer which inited this instance
	rsp		interface{}			// pointer to response message received
	tid		int					// identity of timer for response
	pit		time.Time			// ping sent time
	pot		time.Time			// pong received time
}

//
// FindNode pending item
//
type queryPendingEntry struct {
	node	*Node				// peer node to be queried
	target	*NodeID				// target looking for
}

//
// Table manager
//
const TabMgrName = sch.TabMgrName

type tableManager struct {
	lock			sync.Mutex			// lock for sync
	name			string				// name
	tep				sch.SchUserTaskEp	// entry
	cfg				tabConfig			// configuration
	ptnMe			interface{}			// pointer to task node of myself
	ptnNgbMgr		interface{}			// pointer to neighbor manager task node
	ptnDcvMgr		interface{}			// pointer to discover manager task node
	shaLocal		Hash				// hash of local node identity
	buckets			[nBuckets]*bucket	// buckets
	queryIcb		[]*instCtrlBlock	// active query instance table
	boundIcb		[]*instCtrlBlock	// active bound instance table
	queryPending	[]*queryPendingEntry// pending to be queried
	boundPending	[]*Node				// pending to be bound
	dlkTab			[]int				// log2 distance lookup table for a xor byte
	refreshing		bool				// busy in refreshing now
	dataDir			string				// data directory
	arfTid			int					// auto refresh timer identity

	//
	// Notice: currently Ethereum's database interface is introduced, and
	// we had make some modification on it, see file nodedb.go for details
	// please.
	//

	nodeDb			*nodeDB				// node database object pointer
}

var tabMgr = tableManager{
	name:			TabMgrName,
	tep:			nil,
	cfg:			tabConfig{},
	ptnMe:			nil,
	ptnNgbMgr:		nil,
	ptnDcvMgr:		nil,
	shaLocal:		Hash{},
	buckets:		[nBuckets]*bucket{},
	queryIcb:		make([]*instCtrlBlock, 0, TabInstQueringMax),
	boundIcb:		make([]*instCtrlBlock, 0, TabInstBondingMax),
	queryPending:	make([]*queryPendingEntry, 0, TabInstQPendingMax),
	boundPending:	make([]*Node, 0, TabInstBPendingMax),
	dlkTab:			make([]int, 256),
	refreshing:		false,
	dataDir:		"",
	nodeDb:			nil,
	arfTid:			sch.SchInvalidTid,
}

//
// To escape the compiler "initialization loop" error
//
func init() {
	tabMgr.tep = TabMgrProc
	ndbCleaner.tep = NdbcProc
}

//
// Table manager entry
//
func TabMgrProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	yclog.LogCallerFileLine("TabMgrProc: scheduled, msg: %d", msg.Id)

	if ptn == nil {
		yclog.LogCallerFileLine("TabMgrProc: invalid parameters")
		return TabMgrEnoParameter
	}

	//
	// Notice: this table module had exported some functions to access the bucket
	// and the node database, such as function TabBucketAddNode, and so on. The
	// other tasks, for example, the neighbor manager "NgbMgr"(sch.NgbMgrName),
	// would call those shared functions. To sync the accessing, here we apply a
	// simple(but not a good) method: obtain the lock at the task entry and free
	// it until leaving.
	//

	tabMgr.lock.Lock()
	defer tabMgr.lock.Unlock()

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

	if ptn == nil {
		yclog.LogCallerFileLine("TabMgrPoweron: invalid parameters")
		return TabMgrEnoParameter
	}

	var eno TabMgrErrno = TabMgrEnoNone

	//
	// fetch configurations
	//

	if eno = tabGetConfig(&tabMgr.cfg); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPoweron: tabGetConfig failed, eno: %d", eno)
		return eno
	}

	//
	// prepare node database
	//

	if eno = tabNodeDbPrepare(); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPoweron: tabNodeDbPrepare failed, eno: %d", eno)
		return eno
	}

	//
	// build local node identity hash for neighbors finding
	//

	if eno = tabSetupLocalHashId(); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPoweron: tabSetupLocalHash failed, eno: %d", eno)
		return eno
	}

	//
	// preapare related task ponters
	//

	if eno = tabRelatedTaskPrepare(ptn); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPoweron: tabRelatedTaskPrepare failed, eno: %d", eno)
		return eno
	}

	//
	// setup the lookup table
	//

	if eno = tabSetupLog2DistanceLookupTable(tabMgr.dlkTab); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPoweron: tabSetupLog2DistanceLookupTable failed, eno: %d", eno)
		return eno
	}

	//
	// setup auto-refresh timer
	//

	if eno = tabStartTimer(nil, sch.TabRefreshTimerId, autoRefreshCycle); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPoweron: tabStartTimer failed, eno: %d", eno)
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
		yclog.LogCallerFileLine("TabMgrPoweron: tabRefresh failed, eno: %d", eno)
		return eno
	}

	return TabMgrEnoNone
}

//
// Poweroff handler
//
func TabMgrPoweroff(ptn interface{}) TabMgrErrno {

	if ptn == nil {
		yclog.LogCallerFileLine("TabMgrPoweroff: invalid parameters")
		return TabMgrEnoParameter
	}

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

	if inst == nil {
		yclog.LogCallerFileLine("TabMgrPingpongTimerHandler: invalid parameters")
		return TabMgrEnoParameter
	}

	//
	// update database for the neighbor node
	//

	inst.state = TabInstStateBTimeout
	inst.rsp = nil
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

	if inst == nil {
		yclog.LogCallerFileLine("TabMgrFindNodeTimerHandler: invalid parameters")
		return TabMgrEnoParameter
	}

	//
	// update database for the neighbor node
	//

	inst.state = TabInstStateQTimeout
	inst.rsp = nil
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

	yclog.LogCallerFileLine("TabMgrFindNodeRsp: FindNode response received")

	if msg == nil {
		yclog.LogCallerFileLine("TabMgrFindNodeRsp: invalid parameters")
		return TabMgrEnoParameter
	}

	//
	// lookup active instance for the response
	//

	var inst *instCtrlBlock = nil

	inst = tabFindInst(&msg.FindNode.To, TabInstStateQuering)
	if inst == nil {
		yclog.LogCallerFileLine("TabMgrFindNodeRsp: instance not found")
		return TabMgrEnoNotFound
	}
	inst.rsp = msg

	//
	// obtain result
	//

	var result = msg.Result
	if result != 0 { result = TabMgrEnoFindNodeFailed }

	//
	// update database for the neighbor node.
	// DON'T care the result, we must go ahead to remove the instance,
	// see bellow.
	//

	if eno := tabUpdateNodeDb(inst, result); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrFindNodeRsp: tabUpdateNodeDb failed, eno: %d", eno)
	}

	//
	// update buckets：DON'T care the result, we must go ahead to remove the instance,
	// see bellow.
	//

	if eno := tabUpdateBucket(inst, result); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrFindNodeRsp: tabUpdateBucket failed, eno: %d", eno)
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
	// check result reported, if it's failed, need not go further
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
			break
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
	// Lookup active instance for the response. Notice: some respons without actived
	// instances might be sent here, see file neighbor.go please. To speed up the p2p
	// network, one might push those nodes into buckets and node database, but now in
	// current implement, except the case that the local node is a bootstrap node, we
	// discard all pong responses without an actived instance.
	//
	// Notice: we had modify the logic to accept all pong responses. If local instance
	// if not found, we act as we are a bootstrap node, see bellow pls.
	//

	if msg == nil {
		yclog.LogCallerFileLine("TabMgrPingpongRsp: invalid parameters")
		return TabMgrEnoParameter
	}

	var inst *instCtrlBlock = nil
	inst = tabFindInst(&msg.Ping.To, TabInstStateQuering)

	if inst == nil {

		//
		// Not found
		//

		if tabMgr.cfg.bootstratNode == false {

			yclog.LogCallerFileLine("TabMgrPingpongRsp: instance not found and local is not a bootstrap node")

			//
			// Comment the following statement to act like a bootstrap node
			//

			/*return TabMgrEnoNotFound*/
		}

		return tabUpdateBootstarpNode(&msg.Pong.From)
	}

	inst.rsp = msg

	//
	// Obtain result
	//

	var result = msg.Result
	if result != 0 { result = TabMgrEnoPingpongFailed }

	//
	// Update database for the neighbor node, we should not return when function
	// tabUpdateNodeDb return failed, for some nodes we try to operate on might
	// still not be backup to database.
	//

	if eno := tabUpdateNodeDb(inst, result); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPingpongRsp: tabUpdateNodeDb failed, eno: %d", eno)
	}

	//
	// Update buckets, we should not return when function tabUpdateBucket return
	// failed, for some nodes might not be added into any buckets.
	//

	if eno := tabUpdateBucket(inst, result); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("TabMgrPingpongRsp: tabUpdateBucket failed, eno: %d", eno)
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
	// Check result reported
	//

	if msg.Result != 0 {
		yclog.LogCallerFileLine("TabMgrPingpongRsp: fail reported, result: %d", msg.Result)
		return TabMgrEnoNone
	}

	//
	// Update last pong time
	//

	pot	:= time.Now()
	if eno := tabBucketUpdatePingpongTime(NodeID(inst.req.(*um.Ping).To.NodeId), nil, &pot);
	eno != TabMgrEnoNone {

		yclog.LogCallerFileLine("tabActiveBoundInst: " +
			"tabBucketUpdatePingpongTime failed, eno: %d",
			eno)

		return eno
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
	tep:	nil,
	tid:	sch.SchInvalidTid,
}

//
// NodeDb cleaner entry
//
func NdbcProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {

	yclog.LogCallerFileLine("NdbcProc: " +
		"scheduled, sender: %s, recver: %s, msg: %d",
		sch.SchinfGetMessageSender(msg), sch.SchinfGetMessageRecver(msg), msg.Id)

	if ptn == nil {
		yclog.LogCallerFileLine("NdbcProc: invalid parameters")
		return TabMgrEnoParameter
	}

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

	if ptn == nil {
		yclog.LogCallerFileLine("NdbcPoweron: invalid parameters")
		return TabMgrEnoParameter
	}

	var tmd  = sch.TimerDescription {
		Name:	NdbcName + "_autoclean",
		Utid:	0,
		Tmt:	sch.SchTmTypeAbsolute,
		Dur:	nodeAutoCleanCycle,
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

	if ptn == nil {
		yclog.LogCallerFileLine("NdbcPoweroff: invalid parameters")
		return TabMgrEnoParameter
	}

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

	//
	// Carry out cleanup procedure
	//

	yclog.LogCallerFileLine("NdbcAutoCleanTimerHandler: " +
		"auto cleanup timer expired, it's time to clean ...")

	err := tabMgr.nodeDb.expireNodes()

	if err != nil {

		yclog.LogCallerFileLine("NdbcAutoCleanTimerHandler: " +
			"cleanup failed, err: %s",
			err.Error())

		return TabMgrEnoDatabase
	}

	yclog.LogCallerFileLine("NdbcAutoCleanTimerHandler: cleanup ok")

	return TabMgrEnoNone
}

//
// Fetch configuration
//
func tabGetConfig(tabCfg *tabConfig) TabMgrErrno {

	if tabCfg == nil {
		yclog.LogCallerFileLine("tabGetConfig: invalid parameters")
		return TabMgrEnoParameter
	}

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

	tabCfg.bootstrapNodes = make([]*Node, len(cfg.BootstrapNodes))
	for idx, n := range cfg.BootstrapNodes {
		tabCfg.bootstrapNodes[idx] = new(Node)
		tabCfg.bootstrapNodes[idx].Node = *n
		tabCfg.bootstrapNodes[idx].sha = *tabNodeId2Hash(NodeID(n.ID))
	}

	return TabMgrEnoNone
}

//
// Prepare node database when poweron
//
func tabNodeDbPrepare() TabMgrErrno {
	if tabMgr.nodeDb != nil {
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
	h := sha256.Sum256(id[:])
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

	if ptnMe == nil {
		yclog.LogCallerFileLine("tabRelatedTaskPrepare: invalid parameters")
		return TabMgrEnoParameter
	}

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
			lkt[b] = int(8 - n - 1)
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
	// is empty, this flag is set to false;
	//
	// If the "tid"(target identity) passed in is nil, we get a random one;
	//

	//
	// Check if the active query instances table full. notice that if it's false,
	// then the pending table must be empty in current implement logic.
	//

	tabMgr.refreshing = len(tabMgr.queryIcb) >= TabInstQueringMax
	if tabMgr.refreshing == true {
		yclog.LogCallerFileLine("tabRefresh: already in refreshing")
		return TabMgrEnoNone
	}

	//
	// If nil target passed in, we get a random one
	//

	var nodes []*Node
	var target NodeID

	if tid == nil {
		rand.Read(target[:])
	} else {
		target = *tid
	}

	if nodes = tabClosest(target, TabInstQPendingMax); len(nodes) == 0 {

		//
		// Here all our buckets are empty, we then apply our local node as
		// the target.
		//

		yclog.LogCallerFileLine("tabRefresh: seems all buckets are empty, " +
			"try seeds from database and bootstrap nodes ...")

		target = NodeID(tabMgr.cfg.local.ID)

		seeds := tabSeedsFromDb(TabInstQPendingMax, seedMaxAge)
		if len(seeds) == 0 {
			yclog.LogCallerFileLine("tabRefresh: empty seeds set from nodes database")
		}

		nodes = append(nodes, tabMgr.cfg.bootstrapNodes...)
		nodes = append(nodes, seeds...)

		if len(nodes) == 0 {
			yclog.LogCallerFileLine("tabRefresh: we can't do refreshing without any seeds")
			return TabMgrEnoResource
		}

		if len(nodes) > TabInstQPendingMax {
			yclog.LogCallerFileLine("tabRefresh: " +
				"too much seeds, truncated: %d",
				len(nodes) - TabInstQPendingMax)
			nodes = nodes[:TabInstQPendingMax]
		}
	}

	yclog.LogCallerFileLine("tabRefresh: total number of nodes to query: %d", len(nodes))

	var eno TabMgrErrno

	if eno := tabQuery(&target, nodes); eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("tabRefresh: tabQuery failed, eno: %d", eno)
	} else {
		tabMgr.refreshing = true
	}

	return eno
}

//
// Caculate the distance between two nodes.
// Notice: the return "d" more larger, it's more closer
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

	var closest = make([]*Node, 0, maxBonding)
	var count = 0

	ht := tabNodeId2Hash(target)
	dt := tabLog2Dist(tabMgr.shaLocal, *ht)

	var addClosest = func (bk *bucket) int {
		count = len(closest)
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
func tabSeedsFromDb(size int, age time.Duration) []*Node {

	if size == 0 {
		yclog.LogCallerFileLine("tabSeedsFromDb: invalid zero size")
		return nil
	}

	if size > seedMaxCount { size = seedMaxCount }
	if age > seedMaxAge { age = seedMaxAge }
	nodes := tabMgr.nodeDb.querySeeds(size, age)

	if nodes == nil {
		yclog.LogCallerFileLine("tabSeedsFromDb: nil nodes")
		return nil
	}

	if len(nodes) == 0 {
		yclog.LogCallerFileLine("tabSeedsFromDb: empty node table")
		return nil
	}

	if len(nodes) > size {
		nodes = nodes[0:size]
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

	if target == nil || nodes == nil {
		yclog.LogCallerFileLine("tabQuery: invalid parameters")
		return TabMgrEnoParameter
	}

	remain := len(nodes)
	actNum := len(tabMgr.queryIcb)
	pndNum := len(tabMgr.queryPending)

	yclog.LogCallerFileLine("tabQuery: " +
		"remain: %d, actNum: %d, pndNum: %d",
		remain, actNum, pndNum)

	if remain == 0 {
		yclog.LogCallerFileLine("tabQuery: invalid parameters, no node to be handled")
		return TabMgrEnoParameter
	}

	//
	// create query instances
	//

	var schMsg = sch.SchMessage{}
	var nidx = 0

	if actNum < TabInstQueringMax {

		for loop := 0; loop < remain && loop < TabInstQueringMax - actNum; loop++ {

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

			if eno := sch.SchinfSendMessage(&schMsg); eno != sch.SchEnoNone {
				yclog.LogCallerFileLine("tabQuery: SchinfSendMessage failed, eno: %d", eno)
				return TabMgrEnoScheduler
			}

			if eno := tabStartTimer(icb, sch.TabFindNodeTimerId, findNodeExpiration); eno != TabMgrEnoNone {
				yclog.LogCallerFileLine("tabQuery: tabStartTimer failed, eno: %d", eno)
				return eno
			}

			yclog.LogCallerFileLine("tabQuery: active query appended and EvNblFindNodeReq sent, \n" +
				"\t\tid: %s\n" +
				"\t\tip: %s, udp-port: %d, tcp-port: %d",
				ycfg.P2pNodeId2HexString(msg.To.NodeId),
				msg.To.IP.String(),
				msg.To.UDP,
				msg.To.TCP)

			tabMgr.queryIcb = append(tabMgr.queryIcb, icb)
			nidx++
		}
	}

	//
	// append nodes to pending table if any
	//

	for ; nidx < remain; nidx++ {
		if  len(tabMgr.queryPending) >= TabInstQPendingMax {
			break
		}
		tabMgr.queryPending = append(tabMgr.queryPending, &queryPendingEntry{
			node:nodes[nidx],
			target: target,
		})
		yclog.LogCallerFileLine("tabQuery: " +
			"node append to query pending, id: %s",
			ycfg.P2pNodeId2HexString(nodes[nidx].ID))
	}

	return TabMgrEnoNone
}

//
// Find active instance by node
//
func tabFindInst(node *um.Node, state int) *instCtrlBlock {

	if node == nil {
		yclog.LogCallerFileLine("tabFindInst: invalid parameters")
		return nil
	}

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

	if inst == nil {
		yclog.LogCallerFileLine("tabUpdateNodeDb: invalid parameters")
		return TabMgrEnoParameter
	}

	var fnFailUpdate = func() TabMgrErrno {

		id := NodeID(inst.req.(*um.FindNode).To.NodeId)

		if node := tabMgr.nodeDb.node(id); node == nil {
			yclog.LogCallerFileLine("tabUpdateNodeDb: " +
				"fnFailUpdate: node not exist, do nothing")
			return TabMgrEnoNone
		}

		fails := tabMgr.nodeDb.findFails(id) + 1

		if err := tabMgr.nodeDb.updateFindFails(id, fails); err != nil {

			yclog.LogCallerFileLine("tabUpdateNodeDb: " +
				"fnFailUpdate: updateFindFails failed, err: %s",
				err.Error())

			return TabMgrEnoDatabase
		}

		return TabMgrEnoNone
	}

	var fnFailClear = func () TabMgrErrno {

		id := NodeID(inst.req.(*um.FindNode).To.NodeId)

		if node := tabMgr.nodeDb.node(id); node == nil {
			yclog.LogCallerFileLine("tabUpdateNodeDb: " +
				"fnFailClear: node not exist, do nothing")
			return TabMgrEnoNone
		}

		if err := tabMgr.nodeDb.updateFindFails(id, 0); err != nil {

			yclog.LogCallerFileLine("tabUpdateNodeDb: " +
				"fnFailClear: updateFindFails failed, err: %s",
				err.Error())

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

			yclog.LogCallerFileLine("tabUpdateNodeDb: " +
				"update: updateNode failed, err: %s",
					err.Error())

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

	case (inst.state == TabInstStateQuering || inst.state == TabInstStateQTimeout) &&
		result == TabMgrEnoTimeout:
		return fnFailUpdate()

	case inst.state == TabInstStateBonding && result == TabMgrEnoNone:
		return update()

	case inst.state == TabInstStateBonding && result == TabMgrEnoPingpongFailed:
		return fnFailClear()

	case (inst.state == TabInstStateBonding || inst.state == TabInstStateBTimeout) &&
		result == TabMgrEnoTimeout:
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

	if inst == nil {
		yclog.LogCallerFileLine("tabUpdateBucket: invalid parameters")
		return TabMgrEnoParameter
	}

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

		if eno := tabBucketUpdateFailCounter(id, +1); eno != TabMgrEnoNone {

			yclog.LogCallerFileLine("tabUpdateBucket: " +
				"update FindNode failed counter failed, eno: %d",
				eno)

			return eno
		}

		return TabMgrEnoNone

	case inst.state == TabInstStateQuering && result == TabMgrEnoTimeout:

		id := NodeID(inst.req.(*um.FindNode).To.NodeId)

		if fails := tabMgr.nodeDb.findFails(id) + 1; fails >= maxFindnodeFailures {
			return tabBucketRemoveNode(id)
		}

		if eno := tabBucketUpdateFailCounter(id, +1); eno != TabMgrEnoNone {

			yclog.LogCallerFileLine("tabUpdateBucket: " +
				"update FindNode failed counter failed, eno: %d",
				eno)

			return eno
		}

		return TabMgrEnoNone

	case inst.state == TabInstStateBonding && result == TabMgrEnoNone:

		node := &inst.req.(*um.Ping).To
		inst.pot = time.Now()

		return tabBucketAddNode(node, &inst.pit, &inst.pot)

	case inst.state == TabInstStateBonding && result == TabMgrEnoPingpongFailed:

		node := &inst.req.(*um.Ping).To
		return tabBucketAddNode(node, &inst.pit,nil)

	case inst.state == TabInstStateBonding && result == TabMgrEnoTimeout:

		node := &inst.req.(*um.Ping).To
		return tabBucketAddNode(node, &inst.pit, nil)

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

	if n == nil {
		yclog.LogCallerFileLine("tabUpdateBootstarpNode: invalid parameters")
		return TabMgrEnoParameter
	}

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

	var now = time.Now()
	return tabBucketAddNode(interface{}(&node.Node).(*um.Node), &now, &now)
}

//
// Start timer according instance, timer type, and duration
//
func tabStartTimer(inst *instCtrlBlock, tmt int, dur time.Duration) TabMgrErrno {

	if tmt != sch.TabRefreshTimerId && inst == nil {
		yclog.LogCallerFileLine("tabStartTimer: invalid parameters")
		return TabMgrEnoParameter
	}

	var td = sch.TimerDescription {
		Name:	TabMgrName,
		Utid:	tmt,
		Dur:	dur,
		Extra:	inst,
	}

	switch tmt {
	case sch.TabRefreshTimerId:
		td.Tmt = sch.SchTmTypePeriod
		td.Name = td.Name + "_AutoRefresh"

	case sch.TabFindNodeTimerId:
		td.Tmt = sch.SchTmTypeAbsolute
		td.Name = td.Name + "_FindNode"

	case sch.TabPingpongTimerId:
		td.Tmt = sch.SchTmTypeAbsolute
		td.Name = td.Name + "_Pingpong"

	default:
		yclog.LogCallerFileLine("tabStartTimer: invalid time type, type: %d", tmt)
		return TabMgrEnoParameter
	}

	var eno sch.SchErrno
	var tid int

	if eno, tid = sch.SchInfSetTimer(tabMgr.ptnMe, &td); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("tabStartTimer: SchInfSetTimer failed, eno: %d", eno)
		return TabMgrEnoScheduler
	}

	if tmt == sch.TabRefreshTimerId {
		tabMgr.arfTid = tid
	} else {
		inst.tid = tid
	}

	return TabMgrEnoNone
}

//
// Find node in buckets
//
func tabBucketFindNode(id NodeID) (int, int, TabMgrErrno) {
	for bidx, b := range tabMgr.buckets {
		if b == nil { continue }
		for nidx, n := range b.nodes {
			if NodeID(n.ID) == id {
				return bidx, nidx, TabMgrEnoNone
			}
		}
	}
	return -1, -1, TabMgrEnoNotFound
}

//
// Remove node from bucket
//
func tabBucketRemoveNode(id NodeID) TabMgrErrno {

	bidx, nidx, eno := tabBucketFindNode(id)
	if eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("tabBucketRemoveNode: " +
			"not found, node: %s",
			ycfg.P2pNodeId2HexString(ycfg.NodeID(id)))
		return eno
	}

	nodes := tabMgr.buckets[bidx].nodes
	nodes = append(nodes[0:nidx], nodes[nidx+1:] ...)
	return TabMgrEnoNone
}

//
// Update FindNode failed counter
//
func tabBucketUpdateFailCounter(id NodeID, delta int) TabMgrErrno {

	bidx, nidx, eno := tabBucketFindNode(id)
	if eno != TabMgrEnoNone {
		yclog.LogCallerFileLine("tabBucketUpdateFailCounter: " +
			"not found, node: %s",
			ycfg.P2pNodeId2HexString(ycfg.NodeID(id)))
		return eno
	}

	tabMgr.buckets[bidx].nodes[nidx].failCount += delta
	return TabMgrEnoNone
}

//
// Update pingpong time
//
func tabBucketUpdatePingpongTime(id NodeID, pit *time.Time, pot *time.Time) TabMgrErrno {

	bidx, nidx, eno := tabBucketFindNode(id)
	if eno != TabMgrEnoNone {

		yclog.LogCallerFileLine("tabBucketUpdatePingpongTime: " +
			"not found, node: %s",
			ycfg.P2pNodeId2HexString(ycfg.NodeID(id)))

		return eno
	}

	//
	// if nil, do nothing then
	//

	if pit != nil {
		tabMgr.buckets[bidx].nodes[nidx].lastPing = *pit
	}

	if pot != nil {
		tabMgr.buckets[bidx].nodes[nidx].lastPong = *pot
	}

	return TabMgrEnoNone
}

//
// Find max find node faile count
//
func (b *bucket) maxFindNodeFailed(src []*bucketEntry) ([]*bucketEntry) {

	//
	// if the source is nil, recursive then
	//

	if src == nil {
		return b.maxFindNodeFailed(b.nodes)
	}

	//
	// else, pick entries from source
	//

	var max = 0
	var beMaxf = make([]*bucketEntry, 0)

	for _, be := range src {

		if be.failCount > max {

			max = be.failCount
			beMaxf = []*bucketEntry{}

			beMaxf = append(beMaxf, be)

		} else if be.failCount == max {

			beMaxf = append(beMaxf, be)
		}
	}

	return beMaxf
}

//
// Find node in a specific bucket
//
func (b *bucket) findNode(id NodeID) (int, TabMgrErrno) {

	for idx, n := range b.nodes {

		if NodeID(n.ID) == id {

			return idx, TabMgrEnoNone
		}
	}

	return -1, TabMgrEnoNotFound
}

//
// Find latest added
//
func (b *bucket) latestAdd(src []*bucketEntry) ([]*bucketEntry) {

	//
	// if the source is nil, recursive then
	//

	if src == nil {
		return b.latestAdd(b.nodes)
	}

	//
	// else, pick entries from source
	//

	var latest = time.Now().Add(time.Hour*24*1024)
	var beLatest = make([]*bucketEntry, 0)

	for _, be := range src {

		if be.addTime.After(latest) {

			latest = be.addTime
			beLatest = []*bucketEntry{}
			beLatest = append(beLatest, be)

		} else if be.addTime.Equal(latest) {

			beLatest = append(beLatest, be)
		}
	}

	return beLatest
}

//
// Find latest pong
//
func (b *bucket) eldestPong(src []*bucketEntry) ([]*bucketEntry) {

	//
	// if the source is nil, recursive then
	//

	if src == nil {
		return b.eldestPong(b.nodes)
	}

	//
	// else, pick entries from source
	//

	var eldest = time.Now()
	var beEldest = make([]*bucketEntry, 0)

	for _, be := range src {

		if be.lastPong.Before(eldest) {

			eldest = be.lastPong
			beEldest = []*bucketEntry{}
			beEldest = append(beEldest, be)

		} else if be.lastPong.Equal(eldest) {

			beEldest = append(beEldest, be)
		}
	}

	return beEldest
}

//
// Add node to bucket
//
func tabBucketAddNode(n *um.Node, lastPing *time.Time, lastPong *time.Time) TabMgrErrno {

	//
	// node must be pinged can it be added into a bucket, if pong does not received
	// while adding, we set a very old one.
	//

	if n == nil || lastPing == nil {
		yclog.LogCallerFileLine("tabBucketAddNode: invalid parameters")
		return TabMgrEnoParameter
	}

	if lastPong == nil {
		var veryOld = time.Now().Add(-time.Hour*24*1024)
		lastPong = &veryOld
	}

	//
	// locate bucket for node
	//

	id := NodeID(n.NodeId)
	h := tabNodeId2Hash(id)
	d := tabLog2Dist(tabMgr.shaLocal, *h)
	b := tabMgr.buckets[d]

	//
	// if node had been exist, update last pingpong time only
	//

	if nidx, eno := b.findNode(id); eno == TabMgrEnoNone {
		b.nodes[nidx].lastPing = *lastPing
		b.nodes[nidx].lastPong = *lastPong
		return TabMgrEnoNone
	}

	//
	// if bucket not full, append node
	//

	if len(b.nodes) != bucketSize {

		var be= new(bucketEntry)

		be.Node = *interface{}(n).(*ycfg.Node)
		be.sha = *tabNodeId2Hash(id)
		be.addTime = time.Now()
		be.lastPing = *lastPing
		be.lastPong = *lastPong
		be.failCount = 0

		b.nodes = append(b.nodes, be)

		return TabMgrEnoNone
	}

	//
	// full, we had to kick another node out. the following order applied:
	//
	// 1) the max find node failed
	// 2) the youngest added
	// 3) the eldest pong
	//
	// if at last more than one nodes selected, we kick one randomly.
	//

	var kicked []*bucketEntry = nil
	var beKicked *bucketEntry = nil

	if kicked = b.maxFindNodeFailed(nil); len(kicked) == 1 {
		beKicked = kicked[0]
		goto kickSelected
	}

	if kicked := b.latestAdd(kicked); len(kicked) == 1 {
		beKicked = kicked[0]
		goto kickSelected
	}

	if kicked := b.eldestPong(kicked); len(kicked) == 1 {
		beKicked = kicked[0]
		goto kickSelected
	}

	beKicked = kicked[rand.Int() % len(kicked)]

kickSelected:

	beKicked.Node = *interface{}(n).(*ycfg.Node)
	beKicked.sha = *tabNodeId2Hash(id)
	beKicked.addTime = time.Now()
	beKicked.lastPing = *lastPing
	beKicked.lastPong = *lastPong
	beKicked.failCount = 0

	return TabMgrEnoNone
}

//
// Delete active query instance
//
func tabDeleteActiveQueryInst(inst *instCtrlBlock) TabMgrErrno {

	if inst == nil {
		yclog.LogCallerFileLine("tabDeleteActiveQueryInst: invalid parameters")
		return TabMgrEnoParameter
	}

	for idx, icb := range tabMgr.queryIcb {

		if icb == inst {

			if inst.tid != sch.SchInvalidTid {

				if eno := sch.SchinfKillTimer(tabMgr.ptnMe, inst.tid); eno != sch.SchEnoNone {

					yclog.LogCallerFileLine("tabDeleteActiveQueryInst: " +
						"kill timer failed, eno: %d",
						eno	)

					return TabMgrEnoScheduler
				}

				inst.tid = sch.SchInvalidTid
			}

			tabMgr.queryIcb = append(tabMgr.queryIcb[0:idx], tabMgr.queryIcb[idx+1:]...)
			yclog.LogCallerFileLine("tabDeleteActiveQueryInst: active query removed")

			return TabMgrEnoNone
		}
	}

	yclog.LogCallerFileLine("tabDeleteActiveQueryInst: instance not found")

	return TabMgrEnoNotFound
}

//
// Active query instance
//
func tabActiveQueryInst() TabMgrErrno {

	//
	// check if we can activate more
	//

	if len(tabMgr.queryIcb) == TabInstQueringMax {
		yclog.LogCallerFileLine("tabActiveQueryInst: active query table full")
		return TabMgrEnoNone
	}

	//
	// check if any pending
	//

	if len(tabMgr.queryPending) == 0 {
		yclog.LogCallerFileLine("tabActiveQueryInst: pending query table empty")
		return TabMgrEnoNone
	}

	//
	// activate pendings
	//

	for len(tabMgr.queryPending) > 0 && len(tabMgr.queryIcb) < TabInstQueringMax {

		p := tabMgr.queryPending[0]
		var nodes = []*Node{p.node}

		if eno := tabQuery(p.target, nodes); eno != TabMgrEnoNone {

			yclog.LogCallerFileLine("tabActiveQueryInst: tabQuery failed, eno: %d", eno)
			return eno
		}

		tabMgr.queryPending = append(tabMgr.queryPending[:0], tabMgr.queryPending[1:]...)
		yclog.LogCallerFileLine("tabActiveQueryInst: pending query activated and removed")
	}

	return TabMgrEnoNone
}

//
// Delete active bound instance
//
func tabDeleteActiveBoundInst(inst *instCtrlBlock) TabMgrErrno {

	if inst == nil {
		yclog.LogCallerFileLine("tabDeleteActiveBoundInst: invalid parameters")
		return TabMgrEnoParameter
	}

	for idx, icb := range tabMgr.boundIcb {

		if icb == inst {

			if inst.tid != sch.SchInvalidTid {

				if eno := sch.SchinfKillTimer(tabMgr.ptnMe, inst.tid); eno != sch.SchEnoNone {

					yclog.LogCallerFileLine("tabDeleteActiveBoundInst: " +
						"kill timer failed, eno: %d",
						eno)

					return TabMgrEnoScheduler
				}

				inst.tid = sch.SchInvalidTid
			}

			tabMgr.boundIcb = append(tabMgr.boundIcb[0:idx], tabMgr.boundIcb[idx+1:]...)
			yclog.LogCallerFileLine("tabDeleteActiveBoundInst: active bound removed")
		}
	}

	yclog.LogCallerFileLine("tabDeleteActiveBoundInst: instance not found")

	return TabMgrEnoNotFound
}

//
// Add pending bound instance for node
//
func tabAddPendingBoundInst(node *um.Node) TabMgrErrno {

	if node == nil {
		yclog.LogCallerFileLine("tabAddPendingBoundInst: invalid parameters")
		return TabMgrEnoParameter
	}

	if len(tabMgr.boundPending) == TabInstBPendingMax {
		yclog.LogCallerFileLine("tabAddPendingBoundInst: pending table is full")
		return TabMgrEnoResource
	}

	tabMgr.boundPending = append(tabMgr.boundPending, interface{}(node).(*Node))
	yclog.LogCallerFileLine("tabAddPendingBoundInst: pending bound appended")

	return TabMgrEnoNone
}

//
// Active bound instance
//
func tabActiveBoundInst() TabMgrErrno {

	if len(tabMgr.boundIcb) == TabInstBondingMax {
		yclog.LogCallerFileLine("tabActiveBoundInst: active bounding table is full")
		return TabMgrEnoNone
	}

	if len(tabMgr.boundPending) == 0 {
		yclog.LogCallerFileLine("tabActiveBoundInst: pending table is empty")
		return TabMgrEnoNone
	}

	//
	// Try to activate as most as possible
	//

	for len(tabMgr.boundPending) > 0 && len(tabMgr.boundIcb) < TabInstBondingMax {

		var pn = tabMgr.boundPending[0]

		//
		// Check if bounding needed
		//

		if tabShouldBound(NodeID(pn.ID)) == false {

			yclog.LogCallerFileLine("tabActiveBoundInst: " +
				"need not to bound, peer: %s",
				fmt.Sprintf("%x", pn.ID))

			tabMgr.boundPending = tabMgr.boundPending[1:]
			continue
		}

		var req = um.Ping {
			From:		*interface{}(&tabMgr.cfg.local).(*um.Node),
			To:			*interface{}(&pn.Node).(*um.Node),
			Expiration:	0,
			Id: 		uint64(time.Now().UnixNano()),
			Extra:		nil,
		}

		var schMsg = sch.SchMessage{}
		eno := sch.SchinfMakeMessage(&schMsg, tabMgr.ptnMe, tabMgr.ptnNgbMgr, sch.EvNblPingpongReq, &req)
		if eno != sch.SchEnoNone {

			yclog.LogCallerFileLine("tabActiveBoundInst: SchinfMakeMessage failed, eno: %d", eno)

			return TabMgrEnoScheduler
		}

		var icb = new(instCtrlBlock)

		icb.state = TabInstStateBonding
		icb.req = &req
		icb.rsp = nil
		icb.tid = sch.SchInvalidTid
		icb.pit = time.Now()

		//
		// since we do not know what time we would be ponged, we set a very old time
		// for we believe it's more valuable at late as possible. please see function
		// tabBucketAddNode for more about this.
		//

		pot	:= time.Now().Add(time.Hour*24*1024)
		pit := time.Now()

		if eno := tabBucketUpdatePingpongTime(NodeID(pn.ID), &pit, &pot); eno != TabMgrEnoNone {

			yclog.LogCallerFileLine("tabActiveBoundInst: " +
				"tabBucketUpdatePingpongTime failed, eno: %d",
				eno)

			return eno
		}

		if eno := sch.SchinfSendMessage(&schMsg); eno != sch.SchEnoNone {
			yclog.LogCallerFileLine("tabActiveBoundInst: SchinfSendMessage failed, eno: %d", eno)
			return TabMgrEnoScheduler
		}

		if eno := tabStartTimer(icb, sch.TabPingpongTimerId, pingpongExpiration); eno != TabMgrEnoNone {
			yclog.LogCallerFileLine("tabActiveBoundInst: tabStartTimer failed, eno: %d", eno)
			return eno
		}

		tabMgr.boundPending = tabMgr.boundPending[1:]
		tabMgr.boundIcb = append(tabMgr.boundIcb, icb)

		yclog.LogCallerFileLine("tabActiveBoundInst: pending bound removed and activated")
	}

	return TabMgrEnoNone
}

//
// Send respone to discover task for a bounded node
//
func tabDiscoverResp(node *um.Node) TabMgrErrno {

	if node == nil {
		yclog.LogCallerFileLine("tabDiscoverResp: invalid parameter")
		return TabMgrEnoParameter
	}

	var rsp = sch.MsgTabRefreshRsp {
		Nodes: []*ycfg.Node{interface{}(node).(*ycfg.Node)},
	}

	var schMsg = sch.SchMessage{}

	if eno := sch.SchinfMakeMessage(&schMsg, tabMgr.ptnMe, tabMgr.ptnDcvMgr, sch.EvTabRefreshRsp, &rsp);
	eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("tabDiscoverResp: SchinfMakeMessage failed, eno: %d", eno)
		return TabMgrEnoScheduler
	}

	if eno := sch.SchinfSendMessage(&schMsg); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("tabDiscoverResp: SchinfSendMessage failed, eno: %d", eno)
		return TabMgrEnoScheduler
	}

	return TabMgrEnoNone
}

//
// Check should bounding procedure inited for a node
//
func tabShouldBound(id NodeID) bool {

	//
	// If node specified not found in database, bounding needed
	//

	if node := tabMgr.nodeDb.node(id); node == nil {
		yclog.LogCallerFileLine("tabShouldBound: not found, bounding needed")
		return true
	}

	//
	// If find node fail counter not be zero, bounding needed
	//

	failCnt := tabMgr.nodeDb.findFails(id)
	age := time.Since(tabMgr.nodeDb.lastPong(id))

	needed := failCnt > 0 || age > nodeReboundDuration

	yclog.LogCallerFileLine("tabShouldBound: " +
		"needed: %t, failCnt: %d, age: %d",
		needed, failCnt, age)

	return needed
}

//
// Upate node for the bucket
//
func TabBucketAddNode(n *um.Node, lastPing *time.Time, lastPong *time.Time) TabMgrErrno {

	//
	// We would be called by other task, we need to lock and
	// defer unlock.
	//

	tabMgr.lock.Lock()
	defer tabMgr.lock.Unlock()

	return tabBucketAddNode(n, lastPong, lastPong)
}


//
// Upate a node for node database
//
func TabUpdateNode(umn *um.Node) TabMgrErrno {

	//
	// We would be called by other task, we need to lock and
	// defer unlock. Also notice that: calling this function
	// for a node would append new node or overwrite the exist
	// one, the FindNode fail counter would be set to zero.
	// See function tabMgr.nodeDb.updateNode for more please.
	//

	if umn == nil {
		yclog.LogCallerFileLine("TabUpdateNode: invalid parameter")
		return TabMgrEnoParameter
	}

	tabMgr.lock.Lock()
	defer tabMgr.lock.Unlock()

	n := Node {
		sha: *tabNodeId2Hash(NodeID(umn.NodeId)),
		Node: ycfg.Node{
			IP:  umn.IP,
			UDP: umn.UDP,
			TCP: umn.TCP,
			ID:  ycfg.NodeID(umn.NodeId),
		},
	}

	if err := tabMgr.nodeDb.updateNode(&n); err != nil {

		yclog.LogCallerFileLine("TabUpdateNode: " +
			"update: updateNode failed, err: %s",
				err.Error())

		return TabMgrEnoDatabase
	}

	return TabMgrEnoNone
}

//
// Fetch closest nodes for target
//
func TabClosest(target NodeID, size int) []*Node {

	//
	// We would be called by other task, we need to lock and
	// defer unlock.
	//

	tabMgr.lock.Lock()
	defer tabMgr.lock.Unlock()

	return tabClosest(target, size)
}

//
// Build a table node for ycfg.Node
//
func TabBuildNode(pn *ycfg.Node) *Node {

	//
	// Need not to lock
	//

	return &Node{
		sha: *tabNodeId2Hash(NodeID(pn.ID)),
		Node: ycfg.Node{
			IP:  pn.IP,
			UDP: pn.UDP,
			TCP: pn.TCP,
			ID:  ycfg.NodeID(pn.ID),
		},
	}
}