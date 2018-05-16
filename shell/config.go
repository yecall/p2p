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
	"fmt"
	ycfg	"ycp2p/config"
	yclog	"ycp2p/logger"
)

//
// Get default configuration pointer
//
func ShellDefaultConfig() *ycfg.Config {

	cfg := ycfg.P2pDefaultConfig()

	yclog.LogCallerFileLine("ShellDefaultConfig: %s",
		fmt.Sprintf("%+v", *cfg))

	return cfg
}

//
// Set configuration
//
func ShellConfig(cfg *ycfg.Config) ycfg.P2pCfgErrno {

	if cfg == nil {
		yclog.LogCallerFileLine("ShellConfig: invalid parameter")
		return ycfg.PcfgEnoParameter
	}

	yclog.LogCallerFileLine("ShellConfig: %s",
		fmt.Sprintf("%+v", *cfg))

	return ycfg.P2pConfig(cfg)
}

