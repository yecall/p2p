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
)


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

