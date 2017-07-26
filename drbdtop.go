/*
 *drbdtop - statistics for DRBD
 *Copyright © 2017 Hayley Swimelar and Roland Kammerer
 *
 *This program is free software; you can redistribute it and/or modify
 *it under the terms of the GNU General Public License as published by
 *the Free Software Foundation; either version 2 of the License, or
 *(at your option) any later version.
 *
 *This program is distributed in the hope that it will be useful,
 *but WITHOUT ANY WARRANTY; without even the implied warranty of
 *MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *GNU General Public License for more details.
 *
 *You should have received a copy of the GNU General Public License
 *along with this program; if not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"fmt"
	"os"
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/linbit/drbdtop/pkg/collect"
	"github.com/linbit/drbdtop/pkg/display"
	"github.com/linbit/drbdtop/pkg/resource"
)

// Version defines the version of the program and gets set via ldflags
var Version string

func main() {
	app := kingpin.New("drbdtop", "Statistics for DRBD")
	file := app.Flag(
		"file", "Path to a file containing output gathered from polling 'drbdsetup events2 --timestamps --statistics --now'.").PlaceHolder("/path/to/file").Short('f').String()
	interval := app.Flag(
		"interval", "Time to wait between updating DRBD status, minimum 400ms. Valid units are 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h'.").Short('i').Default("1s").String()
	tui := app.Flag(
		"tui", "Set the TUI (text/interactive)").Short('t').Default("interactive").String()
	expert := app.Flag(
		"expert", "Enable expert mode (e.g., does not print for confirmation)").Short('e').Bool()

	// Prints the version.
	app.Version(Version)

	// Enable short flags for help and version.
	app.VersionFlag.Short('v')
	app.HelpFlag.Short('h')

	kingpin.MustParse(app.Parse(os.Args[1:]))

	errors := make(chan error, 100)

	duration, err := time.ParseDuration(*interval)
	if err != nil {
		errors <- fmt.Errorf("defaulting to 1s polling interval: %v", err)
		duration = time.Second * 1
	}
	if duration < time.Millisecond*400 {
		errors <- fmt.Errorf("interval %s is too quick, switching to 400ms minimum polling interval", duration.String())
		duration = time.Millisecond * 400
	}

	var input collect.Collector

	if *file != "" {
		duration = 0 // Set duration to zero to prevent pruning.
		input = collect.FileCollector{Path: file}
	} else {
		input = collect.Events2Poll{Interval: duration}
	}

	events := make(chan resource.Event, 5)
	go input.Collect(events, errors)

	if *tui == "interactive" {
		display := display.NewFancyTUI(duration, *expert)
		display.SetVersion(Version)
		display.Display(events, errors)
	} else {
		display := display.NewUglyPrinter(duration)
		display.Display(events, errors)
	}
}
