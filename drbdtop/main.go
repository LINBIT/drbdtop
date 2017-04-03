/*
 *drbdtop - continously update stats on drbd
 *Copyright © 2017 Hayley Swimelar
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
	"log"
	"os"
	"runtime/pprof"
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"drbdtop.io/drbdtop/pkg/collect"
	"drbdtop.io/drbdtop/pkg/display"
	"drbdtop.io/drbdtop/pkg/resource"
)

// Version defines the version of the program and gets set via ldflags
var Version string

func main() {
	file := kingpin.Flag(
		"file", "Path to a file containing output gathered from polling 'drbdsetup events2 --timestamps --statistics --now'.").PlaceHolder("/path/to/file").Short('f').String()
	interval := kingpin.Flag(
		"interval", "Time to wait between updating DRBD status. Valid units are 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h'.").Short('i').Default("500ms").String()

	// Prints the version.
	kingpin.Version(Version)

	// Enable short flags for help and version.
	kingpin.CommandLine.VersionFlag.Short('v')
	kingpin.CommandLine.HelpFlag.Short('h')

	kingpin.Parse()

	f, err := os.Create("profile")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	errors := make(chan error, 100)

	duration, err := time.ParseDuration(*interval)
	if err != nil {
		errors <- fmt.Errorf("defaulting to 500ms polling interval: %v", err)
		duration = time.Millisecond * 500
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

	display := display.NewUglyPrinter(duration)
	display.Display(events, errors)

}
