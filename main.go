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


package main


import (
	"os"
	"os/signal"
	"ycp2p/shell"
	yclog	"ycp2p/logger"
	sch		"ycp2p/scheduler"
	"fmt"
	"ycp2p/peer"
)


//
// Indication/Package handlers
//
var (
	p2pIndHandler shell.P2pInfIndCallback = p2pIndProc
	p2pPkgHandler shell.P2pInfPkgCallback = p2pPkgProc
)

//
// Indication handler
//
func p2pIndProc(what int, para interface{}) interface{} {

	switch what {

	case shell.P2pIndPeerActivated:

		pap := para.(*shell.P2pIndPeerActivatedPara)

		yclog.LogCallerFileLine("p2pIndProc: " +
			"P2pIndPeerActivated, para: %s",
			fmt.Sprintf("%+v", *pap))

		if eno := shell.P2pInfRegisterCallback(shell.P2pInfPkgCb, p2pPkgHandler, pap.Ptn);
		eno != shell.P2pInfEnoNone {

			yclog.LogCallerFileLine("p2pIndProc: " +
				"P2pInfRegisterCallback failed, eno: %d, task: %s",
				eno,
				sch.SchinfGetTaskName(pap.Ptn))
		}

	case shell.P2pIndConnStatus:

		psp := para.(*shell.P2pIndConnStatusPara)

		yclog.LogCallerFileLine("p2pIndProc: " +
			"P2pIndConnStatus, para: %s",
			fmt.Sprintf("%+v", *psp))

		if psp.Status != 0 {

			yclog.LogCallerFileLine("p2pIndProc: " +
				"status: %d, close peer: %s",
				psp.Status,
				fmt.Sprintf("%x", psp.PeerInfo.NodeId	))

			if eno := shell.P2pInfClosePeer((*peer.PeerId)(&psp.PeerInfo.NodeId));
			eno != shell.P2pInfEnoNone {
				yclog.LogCallerFileLine("p2pIndProc: " +
					"P2pInfClosePeer failed, eno: %d, peer: %s",
					eno,
					fmt.Sprintf("%x", psp.PeerInfo.NodeId))
			}
		}

	case shell.P2pIndPeerClosed:

		yclog.LogCallerFileLine("p2pIndProc: " +
			"P2pIndPeerClosed, para: %d",
			what, *para.(*int))


	default:
		yclog.LogCallerFileLine("p2pIndProc: inknown indication: %d", what)
	}

	return para
}

//
// Package handler
//
func p2pPkgProc(msg *shell.P2pPackage4Callback) interface{} {
	return nil
}

func main() {

	yclog.LogCallerFileLine("main: going to start ycp2p ...")

	//
	// fetch default from underlying
	//

	dftCfg := shell.ShellDefaultConfig()
	if dftCfg == nil {
		yclog.LogCallerFileLine("main: ShellDefaultConfig failed")
		return
	}

	//
	// one can then apply his configurations based on the default by calling
	// ShellConfig with a defferent configuration if he likes to.
	//

	myCfg := *dftCfg
	shell.ShellConfig(&myCfg)

	//
	// init and then start
	//

	if eno := shell.P2pInit(); eno != sch.SchEnoNone {
		yclog.LogCallerFileLine("main: SchinfSchedulerInit failed, eno: %d", eno)
		return
	}

	//
	// register indication handler
	//

	if eno := shell.P2pInfRegisterCallback(shell.P2pInfIndCb, p2pIndHandler, nil);
	eno != shell.P2pInfEnoNone {
		yclog.LogCallerFileLine("main: P2pInfRegisterCallback failed, eno: %d", eno)
		return
	}

	eno, _ := shell.P2pStart()

	yclog.LogCallerFileLine("main: ycp2p started, eno: %d", eno)

	//
	// hook a system interrupt signal and wait on it
	//

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	defer signal.Stop(sigc)
	<-sigc
}

