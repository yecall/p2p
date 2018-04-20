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


package shell

import (
	sch 	"ycp2p/scheduler"
	dcv		"ycp2p/discover"
	tab		"ycp2p/discover/table"
	ngb		"ycp2p/discover/neighbor"
			"ycp2p/peer"
			"ycp2p/dht"
	dhtro	"ycp2p/dht/router"
	dhtch	"ycp2p/dht/chunker"
	dhtre	"ycp2p/dht/retriver"
	dhtst	"ycp2p/dht/storer"
	dhtsy	"ycp2p/dht/syncer"
	yclog	"ycp2p/logger"
)

//
// Static tasks should be listed in following table, which would be passed to scheduler to
// create and schedule them while p2p starts up.
//
var noDog = sch.SchWatchDog {
	HaveDog:false,
}

var TaskStaticTab = []sch.TaskStaticDescription {

	//
	// Following are static tasks for ycp2p module internal. Notice that fields of struct
	// sch.TaskStaticDescription like MbSize, Wd, Flag will be set to default values internal
	// scheduler, please see function schimplSchedulerStart for details pls.
	//

	{	Name:dcv.DcvMgrName,		Tep:dcv.DcvMgrProc,			DieCb: nil,		Wd:noDog,	},
	{	Name:tab.TabMgrName,		Tep:tab.TabMgrProc,			DieCb: nil,		Wd:noDog,	},
	{	Name:ngb.LsnMgrName,		Tep:ngb.LsnMgrProc,			DieCb: nil,		Wd:noDog,	},
	{	Name:ngb.NgbMgrName,		Tep:ngb.NgbMgrProc,			DieCb: nil,		Wd:noDog,	},
	{	Name:peer.PeerMgrName,		Tep:peer.PeerMgrProc,		DieCb: nil,		Wd:noDog,	},
	{	Name:peer.PeerLsnMgrName,	Tep:peer.LsnMgrProc,		DieCb: nil,		Wd:noDog,	},
	{	Name:dht.DhtMgrName,		Tep:dht.DhtMgrProc,			DieCb: nil,		Wd:noDog,	},
	{	Name:dhtro.DhtroMgrName,	Tep:dhtro.DhtroMgrProc,		DieCb: nil,		Wd:noDog,	},
	{	Name:dhtch.DhtchMgrName,	Tep:dhtch.DhtchMgrProc,		DieCb: nil,		Wd:noDog,	},
	{	Name:dhtre.DhtreMgrName,	Tep:dhtre.DhtreMgrProc,		DieCb: nil,		Wd:noDog,	},
	{	Name:dhtst.DhtstMgrName,	Tep:dhtst.DhtstMgrProc,		DieCb: nil,		Wd:noDog,	},
	{	Name:dhtsy.DhtsyMgrName,	Tep:dhtsy.DhtsyMgrProc,		DieCb: nil,		Wd:noDog,	},

	//
	// More static tasks outside ycp2p can be appended bellow
	// handly or by calling function AppendStaticTasks. When
	// function SchinfSchedulerStart called, currently, all
	// tasks registered here would be created and scheduled
	// to go in order.
	//
	// Since static tasks might depend each other, the order
	// to be scheduled to go might have to be taken into account
	// in the future, we leave this possible work later.
	//
}

var taskName2TasNode *map[string]interface{} = nil

//
// Append a static user task to table TaskStaticTab
//
func AppendStaticTasks(
	name string,
	tep sch.SchUserTaskEp,
	dcb func(interface{})sch.SchErrno,
	dog sch.SchWatchDog) sch.SchErrno {
	TaskStaticTab = append(TaskStaticTab, sch.TaskStaticDescription{Name:name, Tep:tep, DieCb:dcb, Wd:dog})
	return sch.SchEnoNone
}

//
// Start p2p
//
func StartP2p() (sch.SchErrno, *map[string]interface{}) {
	var eno sch.SchErrno
	eno, taskName2TasNode = sch.SchinfSchedulerStart(TaskStaticTab)
	return eno, taskName2TasNode
}

//
// Get user static task pointer: this pointer would be required when accessing
// to scheduler, see file schinf.go for more please.
//
func GetTaskNode(name string) interface{} {

	//
	// Notice: this function should be called only after StartYcp2p is called
	// and it returns successfully.
	//

	if taskName2TasNode == nil {
		yclog.LogCallerFileLine("GetTaskNode: seems ycp2p is not started")
		return nil
	}
	_, v := (*taskName2TasNode)[name]
	return v
}




