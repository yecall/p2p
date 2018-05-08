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


package scheduler

import (
	"time"
	golog	"log"
	yclog	"ycp2p/logger"
	"strings"
)

//
// The scheduler for p2p module, and its' pointer. Notice: we do not plan to
// export any scheduler logic to other modules in any system, so we prefert
// a static var here than creating a shceduler objcet on demand, see it pls.
//
var p2pSDL = scheduler {
	tkFree:			nil,
	freeSize:		0,
	tkBusy:			nil,
	busySize:		0,
	tmFree:			nil,
	tmFreeSize: 	0,
	tmBusy:			nil,
	tmBusySize:		0,
	grpCnt:			0,
}

//
// Default task node for shceduler to send event
//
const rawSchTaskName = "schTsk"

var rawSchTsk = schTaskNode {
	task: schTask{name:rawSchTaskName,},
	last: nil,
	next: nil,
}

//
// Default task node for shceduler to send event
//
const rawTmTaskName = "tmTsk"

var rawTmTsk = schTaskNode {
	task: schTask{name:rawTmTaskName,},
	last: nil,
	next: nil,
}

//
// Scheduler initilization
//
func schimplSchedulerInit() SchErrno {

	//
	// make maps
	//

	p2pSDL.tkMap = make(map[schTaskName] *schTaskNode)
	p2pSDL.tmMap = make(map[*schTmcbNode] *schTaskNode)
	p2pSDL.grpMap = make(map[schTaskGroupName][]*schTaskNode)

	//
	// setup free task node queue
	//

	for loop := 0; loop < schTaskNodePoolSize; loop++ {
		schTaskNodePool[loop].last = &schTaskNodePool[(loop - 1 + schTaskNodePoolSize) & (schTaskNodePoolSize - 1)]
		schTaskNodePool[loop].next = &schTaskNodePool[(loop + 1) & (schTaskNodePoolSize - 1)]
		schTaskNodePool[loop].task.tmIdxTab = make(map[*schTmcbNode] int)
	}
	p2pSDL.freeSize = schTaskNodePoolSize
	p2pSDL.tkFree = &schTaskNodePool[0]

	//
	// setup free timer node queue
	//

	for loop := 0; loop < schTimerNodePoolSize; loop++ {
		schTimerNodePool[loop].last = &schTimerNodePool[(loop - 1 + schTimerNodePoolSize) & (schTimerNodePoolSize - 1)]
		schTimerNodePool[loop].next = &schTimerNodePool[(loop + 1) & (schTimerNodePoolSize - 1)]
	}
	p2pSDL.tmFreeSize = schTimerNodePoolSize
	p2pSDL.tmFree = &schTimerNodePool[0]

	return SchEnoNone
}

//
// the common entry point for a scheduler task
//
func schimplCommonTask(ptn *schTaskNode) SchErrno {

	var queMsg	*chan schMessage
	var done 	*chan SchErrno
	var wdt		*time.Timer
	var eno		SchErrno

	// check pointer to task node
	if ptn == nil {
		yclog.LogCallerFileLine("schimplCommonTask: invalid task node pointer")
		return SchEnoParameter
	}

	// check user task more
	if ptn.task.utep == nil || ptn.task.mailbox.que == nil || ptn.task.done == nil {
		yclog.LogCallerFileLine("schimplCommonTask: invalid user task")
		return SchEnoParameter
	}

	// get chans
	queMsg = ptn.task.mailbox.que
	done = &ptn.task.done

	//
	// new watchdog timer: notice: a dog always comes out, but if flag "HaveDog"
	// is false, the user task would never be bited to die, see bellow. since it
	// is a slowy timer, the performance lost is not so much. the dog logic in
	// such a case can be applied as some kinds of statistics for tasks.
	//

	wdt = time.NewTimer(schDeaultWatchCycle)
	defer wdt.Stop()

	//
	// loop to schedule, until done(or something else happened)
	//

taskLoop:

	for {
		select {

		case msg := <-*queMsg:
			ptn.task.utep(ptn, (*SchMessage)(&msg))

		case eno = <-*done:
			if eno != SchEnoNone {
				yclog.LogCallerFileLine("schimplCommonTask: " +
					"task done, name: %s, eno: %d", ptn.task.name, eno)
			}
			break taskLoop

		case <-ptn.task.dog.Feed:
			ptn.task.dog.BiteCounter = 0

		case <-wdt.C:
			ptn.task.dog.BiteCounter++

			if ptn.task.dog.HaveDog {
				// we do have a dog, and we can be bited to die, see aboved
				// comments about dog please.
				if ptn.task.dog.BiteCounter >= ptn.task.dog.DieThreshold {
					eno = SchEnoWatchDog
					break taskLoop
				}
			}
		}
	}

	// exit, remove user task
	ptn.task.stopped<-true
	return schimplStopTaskEx(ptn)
}

//
// the common entry point for timer task
//
func schimplTimerCommonTask(ptm *schTmcbNode) SchErrno {

	var tm *time.Timer

	// check pointer to timer node
	if ptm == nil {
		yclog.LogCallerFileLine("schimplTimerCommonTask: invalid timer pointer")
		return SchEnoParameter
	}

	// check timer type to deal with it
	if ptm.tmcb.tmt == schTmTypePeriod {

		//
		// period, we loop for ever until killed
		//

		tm = time.NewTimer(ptm.tmcb.dur)
		defer tm.Stop()

timerLoop:

		for {
			select {
			case <-tm.C:
				schimplSendTimerEvent(ptm)
				tm.Reset(ptm.tmcb.dur)

			case stop := <-ptm.tmcb.stop:
				if stop == true {
					tm.Stop()
					break timerLoop
				}
			}
		}
	} else if ptm.tmcb.tmt == schTmTypeAbsolute {

		// absolute, check duration
		if ptm.tmcb.dur <= 0 {
			yclog.LogCallerFileLine("schimplTimerCommonTask: " +
				"invalid absolute timer duration:%d", ptm.tmcb.dur)
			return SchEnoParameter
		}

		// send timer event after duration specified
		select {
		case <-time.After(ptm.tmcb.dur):
			schimplSendTimerEvent(ptm)
		}
	} else {

		// unknown
		yclog.LogCallerFileLine("schimplTimerCommonTask: " +
			"invalid timer type: %d", ptm.tmcb.tmt)
		return SchEnoParameter
	}

	// exit
	yclog.LogCallerFileLine("schimplTimerCommonTask: " +
		"timer task exit, timer:%s", ptm.tmcb.name)
	ptm.tmcb.stopped<-true
	return SchEnoNone
}

//
// Get timer node
//
func schimplGetTimerNode() (SchErrno, *schTmcbNode) {

	var tmn *schTmcbNode = nil

	// lock/unlock schduler control block
	p2pSDL.lock.Lock()
	defer p2pSDL.lock.Unlock()

	// if empty
	if p2pSDL.tmFree == nil {
		yclog.LogCallerFileLine("schimplGetTimerNode: free queue is empty")
		return SchEnoResource, nil
	}

	// dequeue one node
	tmn = p2pSDL.tmFree
	last := tmn.last
	next := tmn.next
	next.last = last
	last.next = next
	p2pSDL.tmFree = next
	p2pSDL.tmFreeSize--

	tmn.next = tmn
	tmn.last = tmn

	return SchEnoNone, tmn
}

//
// Ret timer node to free queue
//
func schimplRetTimerNode(ptm *schTmcbNode) SchErrno {

	if ptm == nil {
		yclog.LogCallerFileLine("schimplRetTimerNode: invalid timer node pointer")
		return SchEnoParameter
	}

	// lock/unlock scheduler control block
	p2pSDL.lock.Lock()
	defer  p2pSDL.lock.Unlock()

	// enqueue a node
	if p2pSDL.tmFree == nil {
		ptm.last = ptm
		ptm.next = ptm
	} else {
		last := p2pSDL.tmFree.last
		next := p2pSDL.tmFree.next
		ptm.last = last
		ptm.next = next
		last.next = ptm
		next.last = ptm
	}
	p2pSDL.tmFree = ptm
	p2pSDL.tmFreeSize++

	return SchEnoNone
}

//
// Get task node
//
func schimplGetTaskNode() (SchErrno, *schTaskNode) {

	var tkn *schTaskNode = nil

	// lock/unlock schduler control block
	p2pSDL.lock.Lock()
	defer p2pSDL.lock.Unlock()

	// if empty
	if p2pSDL.tkFree== nil {
		yclog.LogCallerFileLine("schimplGetTaskNode: free queue is empty")
		return SchEnoResource, nil
	}

	// dequeue one node
	tkn = p2pSDL.tkFree
	last := tkn.last
	next := tkn.next
	next.last = last
	last.next = next
	p2pSDL.tkFree = next
	p2pSDL.freeSize--

	tkn.next = tkn
	tkn.last = tkn

	return SchEnoNone, tkn
}

//
// Ret task node
//
func schimplRetTaskNode(ptn *schTaskNode) SchErrno {

	if ptn == nil {
		yclog.LogCallerFileLine("schimplRetTaskNode: invalid task node pointer")
		return SchEnoParameter
	}

	// lock/unlock scheduler control block
	p2pSDL.lock.Lock()
	defer  p2pSDL.lock.Unlock()

	// enqueue a node
	if p2pSDL.tkFree == nil {
		ptn.last = ptn
		ptn.next = ptn
	} else {
		last := p2pSDL.tkFree.last
		next := p2pSDL.tkFree.next
		ptn.last = last
		ptn.next = next
		last.next = ptn
		next.last = ptn
	}
	p2pSDL.tkFree = ptn
	p2pSDL.freeSize++

	return SchEnoNone
}

//
// Task node enter the busy queue
//
func schimplTaskBusyEnque(ptn *schTaskNode) SchErrno {
	
	if ptn == nil {
		yclog.LogCallerFileLine("schimplTaskBusyEnque: invalid task node pointer")
		return SchEnoParameter
	}

	// lock/unlock scheduler control block
	p2pSDL.lock.Lock()
	defer  p2pSDL.lock.Unlock()

	// enqueue a node
	if p2pSDL.tkBusy == nil {
		ptn.last = ptn
		ptn.next = ptn
	} else {
		last := p2pSDL.tkBusy.last
		next := p2pSDL.tkBusy.next
		ptn.last = last
		ptn.next = next
		last.next = ptn
		next.last = ptn
	}
	p2pSDL.tkBusy = ptn
	p2pSDL.busySize++

	return SchEnoNone
}

//
// Task node dequeue from the busy queue
//
func schimplTaskBusyDeque(ptn *schTaskNode) SchErrno {

	if ptn == nil {
		yclog.LogCallerFileLine("schimplTaskBusyDeque: invalid parameter")
		return SchEnoParameter
	}

	// lock/unlock schduler control block
	p2pSDL.lock.Lock()
	defer p2pSDL.lock.Unlock()

	// remove the busy node
	if p2pSDL.busySize <= 0 {

		yclog.LogCallerFileLine("schimplTaskBusyDeque: invalid parameter")
		return SchEnoInternal

	} else if p2pSDL.busySize == 1 {

		if p2pSDL.tkBusy != ptn {

			yclog.LogCallerFileLine("schimplTaskBusyDeque: invalid parameter")
			return SchEnoInternal

		} else {

			p2pSDL.tkBusy = nil
			p2pSDL.busySize = 0
			return SchEnoNone
		}
	}

	last := ptn.last
	next := ptn.next
	last.next = next
	next.last = last
	p2pSDL.busySize--

	if p2pSDL.tkBusy == ptn {
		p2pSDL.tkBusy = next
	}

	return SchEnoNone
}

//
// Send timer event to user task when timer expired
//
func schimplSendTimerEvent(ptm *schTmcbNode) SchErrno {

	// lock/unlock task control block
	var task = &ptm.tmcb.taskNode.task
	task.lock.Lock()
	defer task.lock.Unlock()

	//
	// setup timer event message. notice that the sender is the scheduler indeed,
	// we set sender pointer to raw timer task in this case; and the extra set when
	// timer crated is also return to timer owner.
	//

	var msg = schMessage{
		sender:	&rawTmTsk,
		recver:	ptm.tmcb.taskNode,
		Id:		EvSchNull,
		Body:	ptm.tmcb.extra,
	}

	msg.Id = EvTimerBase + ptm.tmcb.utid

	// put message to task mailbox
	*task.mailbox.que<-msg

	return SchEnoNone
}

//
// Create a single task
//
type schTaskDescription SchTaskDescription

func schimplCreateTask(taskDesc *schTaskDescription) (SchErrno, interface{}){

	var eno SchErrno
	var ptn *schTaskNode

	if taskDesc == nil {
		yclog.LogCallerFileLine("schimplCreateTask: invalid user task description")
		return SchEnoParameter, nil
	}

	//
	// get task node
	//

	if eno, ptn = schimplGetTaskNode(); eno != SchEnoNone || ptn == nil {
		yclog.LogCallerFileLine("schimplCreateTask: schimplGetTaskNode failed, eno: %d", eno)
		return eno, nil
	}

	//
	// check if a nil mailbox
	//

	if ptn.task.mailbox.que != nil {
		yclog.LogCallerFileLine("schimplCreateTask: raw task node mailbox not empty")
		close(*ptn.task.mailbox.que)
	}

	//
	// check if a nil done channel
	//

	if ptn.task.done != nil {
		yclog.LogCallerFileLine("schimplCreateTask: raw task node done not empty")
		close(*ptn.task.mailbox.que)
	}

	//
	// setup user task
	//

	ptn.task.name			= strings.TrimSpace(taskDesc.Name)
	ptn.task.utep			= schUserTaskEp(taskDesc.Ep)
	mq 						:= make(chan schMessage, taskDesc.MbSize)
	ptn.task.mailbox.que	= &mq
	ptn.task.done			= make(chan SchErrno)
	ptn.task.dog			= schWatchDog(*taskDesc.Wd)
	ptn.task.dieCb			= taskDesc.DieCb
	ptn.task.userData		= taskDesc.UserDa

	//
	// make timer table
	//

	for idx, ptm := range ptn.task.tmTab {
		if ptm != nil {
			yclog.LogCallerFileLine("schimplCreateTask: nonil task timer pointer")
			ptn.task.tmTab[idx] = nil
		}
	}

	//
	// make timer map clean
	//

	for k := range ptn.task.tmIdxTab {
		yclog.LogCallerFileLine("schimplCreateTask: user task timer map is not empty")
		delete(ptn.task.tmIdxTab, k)
	}

	//
	// put task node to busy queue
	//

	if eno := schimplTaskBusyEnque(ptn); eno != SchEnoNone {
		yclog.LogCallerFileLine("schimplCreateTask: schimplTaskBusyEnque failed, rc: %d", eno)
		return eno, nil
	}

	//
	// map task name to task node ponter. some dynamic tasks might have empty
	// task name, in this case, the task node pointer would not be mapped in
	// table, and this task could not be found by function schimplGetTaskNodeByName
	//

	if len(ptn.task.name) <= 0 {
		yclog.LogCallerFileLine("schimplCreateTask: task with empty name")
	} else if p2pSDL.tkMap[schTaskName(ptn.task.name)] != nil {
		yclog.LogCallerFileLine("schimplCreateTask: duplicated task name: %s", ptn.task.name)
		return SchEnoDuplicated, nil
	} else {
		p2pSDL.tkMap[schTaskName(ptn.task.name)] = ptn
	}

	//
	// check flag to go
	//

	if taskDesc.Flag == SchCreatedGo {

		yclog.LogCallerFileLine("schimplCreateTask: user task created to go, name:%s", ptn.task.name)

		ptn.task.goStatus = SchCreatedGo
		go schimplCommonTask(ptn)

	} else if taskDesc.Flag == SchCreatedSuspend {

		yclog.LogCallerFileLine("schimplCreateTask: user task created to suspend, name:%s", ptn.task.name)
		ptn.task.goStatus = SchCreatedSuspend

	} else {

		yclog.LogCallerFileLine("schimplCreateTask: unknown flag: %d, set it suspended, name:%s",
			taskDesc.Flag, ptn.task.name)
		ptn.task.goStatus = SchCreatedSuspend
	}

	return SchEnoNone, ptn
}

//
// Create a task group
//
type schTaskGroupDescription SchTaskGroupDescription

func schimplCreateTaskGroup(taskGrpDesc *schTaskGroupDescription)(SchErrno, interface{}) {

	if taskGrpDesc == nil {
		yclog.LogCallerFileLine("")
		return SchEnoParameter, nil
	}

	// check total member number
	mbCount := len(taskGrpDesc.MbList)
	if mbCount > schMaxGroupSize {
		yclog.LogCallerFileLine("")
		return SchEnoResource, nil
	}

	// deal with each group member
	var tkDesc = schTaskDescription {
		Name:	"",
		MbSize:	taskGrpDesc.MbSize,
		Ep:		taskGrpDesc.Ep,
		Wd:		taskGrpDesc.Wd,
		Flag:	taskGrpDesc.Flag,
		DieCb:	taskGrpDesc.DieCb,
	}

	var ptnTab = make([]*schTaskNode, mbCount)

	for _, mbName := range taskGrpDesc.MbList {

		tkDesc.Name = mbName
		if eno, ptn := schimplCreateTask(&tkDesc); eno != SchEnoNone || ptn == nil {

			//
			// When failed for a member, just debug out now. More needed in
			// the future.
			//

			yclog.LogCallerFileLine("")
			ptnTab = nil
		}

		ptnTab = append(ptnTab, nil)
	}

	// update group map
	p2pSDL.lock.Lock()
	defer p2pSDL.lock.Unlock()

	p2pSDL.grpMap[schTaskGroupName(taskGrpDesc.Grp)] = ptnTab
	p2pSDL.grpCnt++

	// notice: SchEnoNone always ret, caller should check ptnTab to known
	// those fialed
	return SchEnoNone, ptnTab
}

//
// Start a single task by task name
//
func schimplStartTask(name string) SchErrno {

	//
	// Notice: only those suspended user task can be started, so this function does
	// not create new user task, instead, it try to find the suspended task and then
	// start it.
	//

	// get task node pointer by name
	eno, ptn := schimplGetTaskNodeByName(name)
	if eno != SchEnoNone || ptn == nil {
		yclog.LogCallerFileLine("schimplStartTask: " +
			"schimplGetTaskNodeByName failed, name: %s, eno: %d, ptn: %x",
			name, eno, ptn)
		return eno
	}

	// can only a suspended task be started
	if ptn.task.goStatus != SchCreatedSuspend {
		yclog.LogCallerFileLine("schimplStartTask: " +
			"invalid user task status: %d", ptn.task.goStatus)
		return SchEnoMismatched
	}

	// go the user task
	ptn.task.goStatus = SchCreatedGo
	go schimplCommonTask(ptn)

	return SchEnoNone
}

//
// Start a task group by group name
//
func schimplStartTaskGroup(grp string) (SchErrno, int) {

	//
	// Notice: only those suspended member user task can be started, so this function
	// does not create new user task, instead, it try to find the suspended member task
	// of the specific group and then start it.
	//

	var failedCount = 0

	// check group counter
	if p2pSDL.grpCnt <= 0 {
		yclog.LogCallerFileLine("schimplStartTaskGroup: none of group exist")
		return SchEnoNotFound, -1
	}

	// group must be exist
	if _, err := p2pSDL.grpMap[schTaskGroupName(grp)]; !err {
		yclog.LogCallerFileLine("schimplStartTaskGroup: not exist, group: %s", grp)
		return SchEnoMismatched, -1
	}

	// deal with every member of group
	var mbTaskMap = p2pSDL.grpMap[schTaskGroupName(grp)]

	for _, ptn := range mbTaskMap {

		yclog.LogCallerFileLine("schimplStartTaskGroup: s" +
			"tart member task: %s of group: %s",
			ptn.task.name, grp)

		if eno := schimplStartTask(ptn.task.name); eno != SchEnoNone {

			yclog.LogCallerFileLine("schimplStartTaskGroup: " +
				"schimplStartTask failed, task: %s, group: %s",
				ptn.task.name, grp)

			failedCount++
		}
	}

	// check total failed counter to return
	if failedCount > 0 {
		yclog.LogCallerFileLine("schimplStartTaskGroup: total failed count: %d", failedCount)
		return SchEnoUnknown, failedCount
	}

	return SchEnoNone, 0
}

//
// Stop a single task by task name
//
func schimplStopTask(name string) SchErrno {

	//
	// get task node pointer by name
	//

	eno, ptn := schimplGetTaskNodeByName(name)
	if eno != SchEnoNone || ptn == nil {

		yclog.LogCallerFileLine("schimplStopTask: " +
			"schimplGetTaskNodeByName failed, name: %s, eno: %d, ptn: %x",
			name, eno, ptn)

		return eno
	}

	//
	// free task node by pointer
	//

	return schimplStopTaskEx(ptn)
}

//
// Stop a task group
//
func schimplStopTaskGroup(grp string) SchErrno {

	// check group counter
	if p2pSDL.grpCnt <= 0 {
		yclog.LogCallerFileLine("schimplStopTaskGroup: none of group exist")
		return SchEnoNotFound
	}

	// check if specific group exist
	if _, err := p2pSDL.grpMap[schTaskGroupName(grp)]; !err {
		yclog.LogCallerFileLine("schimplStopTaskGroup: not exist, group: %s", grp)
		return SchEnoNotFound
	}

	// deal with every member of the group
	var mbTaskMap = p2pSDL.grpMap[schTaskGroupName(grp)]
	for mbName, mbTaskNode := range mbTaskMap {
		yclog.LogCallerFileLine("schimplStopTaskGroup: " +
			"stop member task: %s of group: %s", mbName, grp)
		schimplStopTaskEx(mbTaskNode)
	}

	// remove the group
	p2pSDL.lock.Lock()
	defer p2pSDL.lock.Unlock()

	delete(p2pSDL.grpMap, schTaskGroupName(grp))
	p2pSDL.grpCnt--

	return SchEnoNone
}

//
// Stop a single task by task pointer
//
func schimplStopTaskEx(ptn *schTaskNode) SchErrno {

	//
	// Seems need not to lock the scheduler control block ?! for functions
	// called here had applied the lock if necessary when they called in.
	//

	var ptm *schTmcbNode
	var tid int
	var eno SchErrno

	if ptn == nil {
		yclog.LogCallerFileLine("schimplStopTaskEx: invalid task node pointer")
		return SchEnoParameter
	}

	//
	// dequeue form busy queue
	//

	if eno := schimplTaskBusyDeque(ptn); eno != SchEnoNone {
		yclog.LogCallerFileLine("schimplStopTaskEx: " +
			"schimplTaskBusyDeque failed, eno: %d", eno)
		return eno
	}

	//
	// call user task to die
	//

	if ptn.task.dieCb != nil {
		if eno = ptn.task.dieCb(&ptn.task); eno != SchEnoNone {
			yclog.LogCallerFileLine("schimplStopTaskEx: "+
				"dieCb failed, task: %s, eno: %d",
				ptn.task.name, eno)
			return eno
		}
	}

	//
	// stop user timers
	//

	for ptm, tid = range ptn.task.tmIdxTab {

		if ptm == nil {
			yclog.LogCallerFileLine("schimplStopTaskEx: " +
				"failed, nil timer control block pointer")
			return SchEnoInternal
		}

		if eno = schimplKillTimer(ptn, tid); eno != SchEnoNone {
			yclog.LogCallerFileLine("schimplStopTaskEx: " +
				"schimplKillTimer failed, task: %s, eno: %d",
				ptn.task.name, eno)
			return eno
		}
	}

	//
	// clean the user task control block
	//

	if eno = schimplTcbClean(&ptn.task); eno != SchEnoNone {
		yclog.LogCallerFileLine("schimplStopTaskEx: schimplTcbClean faild, eno: %d", eno)
		return eno
	}

	//
	// free task node
	//

	if eno = schimplRetTaskNode(ptn); eno != SchEnoNone {
		yclog.LogCallerFileLine("schimplStopTaskEx: " +
			"schimplRetTimerNode failed, task: %s, eno: %d",
			ptn.task.name, eno)
		return  eno
	}

	//
	// remove name to task node ponter map
	//

	if len(ptn.task.name) > 0 {
		delete(p2pSDL.tkMap, schTaskName(ptn.task.name))
	}

	return SchEnoNone
}

//
// Make user tack control block clean
//
func schimplTcbClean(tcb *schTask) SchErrno {

	if tcb == nil {
		yclog.LogCallerFileLine("schimplTcbClean: invalid task control block pointer")
		return SchEnoParameter
	}

	tcb.name				= ""
	tcb.utep				= nil
	tcb.dog.Cycle			= SchDefaultDogCycle
	tcb.dog.BiteCounter		= 0
	tcb.dog.DieThreshold	= SchDefaultDogDieThresold
	tcb.dieCb				= nil
	tcb.goStatus			= SchCreatedSuspend

	close(*tcb.mailbox.que)

	for loop := 0; loop < schMaxTaskTimer; loop++ {
		if tcb.tmTab[loop] != nil {
			delete(tcb.tmIdxTab, tcb.tmTab[loop])
			tcb.tmTab[loop] = nil
		}
	}

	return SchEnoNone
}

//
// Delete a single task by task name
//
func schimplDeleteTask(name string) SchErrno {

	//
	// Currently, "Delete" implented as "Stop"
	//

	return schimplStopTask(name)
}

//
// Delete a task group by group name
//
func schimplDeleteTaskGroup(grp string) SchErrno {

	//
	// Currently, "Delete" implented as "Stop"
	//

	return schimplStopTaskGroup(grp)
}

//
// Send message to a specific task
//
func schimplSendMsg2Task(msg *schMessage) (eno SchErrno) {

	// check the message to be sent
	if msg == nil {
		yclog.LogCallerFileLine("schimplSendMsg2Task: invalid message")
		return SchEnoParameter
	}

	if msg.sender == nil {
		yclog.LogCallerFileLine("schimplSendMsg2Task: invalid sender")
		return SchEnoParameter
	}

	if msg.recver == nil {
		yclog.LogCallerFileLine("schimplSendMsg2Task: invalid receiver")
		return SchEnoParameter
	}

	//
	// put message to receiver mailbox. More work might be needed, such as
	// checking against the sender and receiver name to see if they are in
	// busy queue; checking go status of both to see if they are matched,
	// and so on.
	//

	*msg.recver.task.mailbox.que<-*msg

	return SchEnoNone
}

//
// Send message to a specific task group
//
func schimplSendMsg2TaskGroup(grp string, msg *schMessage) (SchErrno, int) {

	var mtl []*schTaskNode = nil
	var found bool
	var failedCount = 0

	// check message to be sent
	if msg == nil {
		yclog.LogCallerFileLine("schimplSendMsg2TaskGroup: invalid message")
		return SchEnoParameter, -1
	}

	// check group
	if mtl, found = p2pSDL.grpMap[schTaskGroupName(grp)]; found != true {
		yclog.LogCallerFileLine("schimplSendMsg2TaskGroup: not exist, group: %s", grp)
		return SchEnoParameter, -1
	}

	// send message to every group member
	for _, ptn := range mtl {
		msg.recver = ptn
		if eno := schimplSendMsg2Task(msg); eno != SchEnoNone {
			yclog.LogCallerFileLine("schimplSendMsg2TaskGroup: " +
				"send failed, group: %s, member: %s",
				grp, ptn.task.name)
			failedCount++
		}
	}

	// always SchEnoNone, caller should check failedCount
	return SchEnoNone, failedCount
}

//
// Set a timer: extra passed in, which would ret to timer owner when
// timer expired; and timer identity returned to caller. So when timer
// event received, one can determine which timer it is, and extract
// those extra put into timer when it's created.
//
func schimplSetTimer(ptn *schTaskNode, tdc *timerDescription) (SchErrno, int) {

	var tid int
	var eno SchErrno
	var ptm *schTmcbNode

	if ptn == nil || tdc == nil {
		yclog.LogCallerFileLine("schimplSetTimer: invalid parameter(s)")
		return SchEnoParameter, schInvalidTid
	}

	//
	// check if some user task timers are free
	//

	for tid = 0; tid < schMaxTaskTimer; tid++ {
		if ptn.task.tmTab[tid] == nil {
			break
		}
	}

	if tid >= schMaxTaskTimer {
		yclog.LogCallerFileLine("schimplSetTimer: too much, timer table is full")
		return SchEnoResource, schInvalidTid
	}

	//
	// get a timer node
	//

	if eno, ptm = schimplGetTimerNode(); eno != SchEnoNone || ptm == nil {
		yclog.LogCallerFileLine("schimplSetTimer: " +
			"schimplGetTimerNode failed, eno: %d", eno)
		return eno, schInvalidTid
	}

	//
	// backup timer node
	//

	ptn.task.tmTab[tid] = ptm
	ptn.task.tmIdxTab[ptm] = tid
	p2pSDL.tmMap[ptm] = ptn

	//
	// setup timer control block
	//

	tcb 			:= &ptm.tmcb
	tcb.name		= tdc.Name
	tcb.utid		= tdc.Utid
	tcb.tmt			= schTimerType(tdc.Tmt)
	tcb.dur			= tdc.Dur
	tcb.taskNode	= ptn
	tcb.extra		= tdc.Extra

	//
	// go timer common task for timer
	//

	go schimplTimerCommonTask(ptm)

	yclog.LogCallerFileLine("schimplSetTimer: timer started, name: %s", tdc.Name)
	return SchEnoNone, tid
}

//
// Kill a timer
//
func schimplKillTimer(ptn *schTaskNode, tid int) SchErrno {

	// para checks
	if ptn == nil || tid < 0 || tid > schMaxTaskTimer {
		yclog.LogCallerFileLine("schimplKillTimer: invalid parameter(s)")
		return SchEnoParameter
	}

	if ptn.task.tmTab[tid] == nil {
		yclog.LogCallerFileLine("schimplKillTimer: try to kill a null timer")
		return SchEnoParameter
	}

	// emit stop signal and wait stopped signal
	var tcb = &ptn.task.tmTab[tid].tmcb
	tcb.stop<-true
	<-tcb.stopped

	// clean timer control block after timer stopped
	tcb.name		= ""
	tcb.tmt			= schTmTypeNull
	tcb.dur			= 0
	tcb.taskNode	= nil
	tcb.extra		= nil
	delete(ptn.task.tmIdxTab, ptn.task.tmTab[tid])
	delete(p2pSDL.tmMap, ptn.task.tmTab[tid])

	// return timer node to free queue
	if eno := schimplRetTimerNode(ptn.task.tmTab[tid]); eno != SchEnoNone {
		yclog.LogCallerFileLine("schimplRetTimerNode failed, eno: %d", eno)
		return eno
	}

	return SchEnoNone
}

//
// Get task node pointer by task name
//
func schimplGetTaskNodeByName(name string) (SchErrno, *schTaskNode) {

	// if exist
	if _, err := p2pSDL.tkMap[schTaskName(name)]; !err {
		return SchEnoNotFound, nil
	}

	// yes
	return SchEnoNone, p2pSDL.tkMap[schTaskName(name)]
}

//
// Done a task
//
func schimplTaskDone(ptn *schTaskNode, eno SchErrno) SchErrno {

	if ptn == nil {
		yclog.LogCallerFileLine("schimplTaskDone: invalid task node pointer")
		return SchEnoParameter
	}

	ptn.task.done<-eno
	if <-ptn.task.stopped != true {
		yclog.LogCallerFileLine("schimplTaskDone: it's done, but seems some errors")
		return SchEnoInternal
	}

	return SchEnoNone
}

//
// Check if KILLED signal fired for task
//
func schimplTaskKilled(ptn *schTaskNode) bool {

	if ptn == nil {
		yclog.LogCallerFileLine("schimplTaskKilled: invalid parameter(s)")
		return false
	}

	var killed = false

	select {
	case eno:= <-ptn.task.done:
		ptn.task.done<-eno
		if eno == SchEnoKilled {
			killed = true
		}
	default:
	}

	return killed
}

//
// Get user data area pointer
//
func schimplGetUserDataArea(ptn *schTaskNode) interface{} {
	if ptn == nil {
		return nil
	}
	return ptn.task.userData
}

//
// Get task name
//
func schimplGetTaskName(ptn *schTaskNode) string {
	if ptn == nil {
		return ""
	}
	return ptn.task.name
}

//
// Start scheduler
//
func schimplSchedulerStart(tsd []TaskStaticDescription) (eno SchErrno, name2Ptn *map[string]interface{}){


	golog.Printf("schimplSchedulerStart:")
	golog.Printf("schimplSchedulerStart:")
	golog.Printf("schimplSchedulerStart: going to start ycp2p scheduler ...")
	golog.Printf("schimplSchedulerStart:")
	golog.Printf("schimplSchedulerStart:")

	var po = schMessage {
		sender:	&rawSchTsk,
		recver: nil,
		Id:		EvSchPoweron,
		Body:	nil,
	}

	var ptn interface{} = nil

	var tkd  = schTaskDescription {
		MbSize:	schMaxMbSize,
		Wd:		&SchWatchDog {Cycle:SchDefaultDogCycle, DieThreshold:SchDefaultDogDieThresold},
		Flag:	SchCreatedGo,
	}

	var name2PtnMap = make(map[string] interface{})

	if len(tsd) <= 0 {
		yclog.LogCallerFileLine("schimplSchedulerStart: static task table is empty")
		return SchEnoParameter, nil
	}

	//
	// loop the static table table
	//

	for loop, desc := range tsd {

		yclog.LogCallerFileLine("schimplSchedulerStart: " +
			"start a static task, idx: %d, name: %s", loop, desc.Name)

		//
		// setup task description
		//

		tkd.Name	= tsd[loop].Name
		tkd.DieCb	= tsd[loop].DieCb
		tkd.Ep		= tsd[loop].Tep

		//
		// create task
		//

		if eno, ptn = schimplCreateTask(&tkd); eno != SchEnoNone {

			yclog.LogCallerFileLine("schimplSchedulerStart: " +
				"schimplCreateTask failed, task: %s", tkd.Name)

			return SchEnoParameter, nil
		}

		//
		// backup task node pointer by name
		//

		name2PtnMap[tkd.Name] = ptn

		//
		// send poweron
		//

		po.recver = ptn.(*schTaskNode)
		if eno = schimplSendMsg2Task(&po); eno != SchEnoNone {

			yclog.LogCallerFileLine("schimplSchedulerStart: " +
				"schimplSendMsg2Task failed, event: EvSchPoweron, target task: %s", tkd.Name)

			return eno, nil
		}
	}

	golog.Printf("schimplSchedulerStart:")
	golog.Printf("schimplSchedulerStart:")
	golog.Printf("schimplSchedulerStart: it's ok, ycp2p in running")
	golog.Printf("schimplSchedulerStart:")
	golog.Printf("schimplSchedulerStart:")

	return SchEnoNone, &name2PtnMap
}

