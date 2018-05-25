//
// This file implements the DHT Route module: a static task for route manager
// is needed.
//
// liyy, 20180408
//

package route

import (
	sch 	"ycp2p/scheduler"
	yclog	"ycp2p/logger"
)

//
// Route manager
//
const DhtrMgrName = "DhtpMgr"

type dhtRouteManager struct {
	name	string				// name
	tep		sch.SchUserTaskEp	// entry
}

var dhtpMgr = dhtRouteManager{
	name:	DhtrMgrName,
	tep:	DhtrMgrProc,
}

//
// Table manager entry
//
func DhtrMgrProc(ptn interface{}, msg *sch.SchMessage) sch.SchErrno {
	yclog.LogCallerFileLine("DhtrMgrProc: scheduled, msg: %d", msg.Id)
	return sch.SchEnoNone
}

