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
	"flag"
	"fmt"
	"strings"
	"sync"
	"time"

	"drbdtop.io/drbdtop/pkg/collect"
	"drbdtop.io/drbdtop/pkg/display"
	"drbdtop.io/drbdtop/pkg/resource"
)

func main() {
	file := flag.String("file", "", "Path to a file containing output gathered from polling `drbdsetup events2 --timestamps --statistics --now`")
	interval := flag.String("interval", "500ms",
		"Time to wait between updating drbd status. Valid time units are 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h'. Defualt: 500ms")

	flag.Parse()

	rawEvents := make(chan string)
	errors := make(chan error, 100)

	var input collect.Collector

	if *file != "" {
		input = collect.FileCollector{Path: file}
	} else {
		duration, err := time.ParseDuration(*interval)
		if err != nil {
			errors <- fmt.Errorf("defaulting to 500ms polling interval: %v", err)
			duration = time.Millisecond * 500
		}
		input = collect.Events2Poll{Interval: duration}
	}

	go input.Collect(rawEvents, errors)

	events := make(chan resource.Event, 5)

	// Parse rawEvents and send them into the events channel.
	go func() {
		for {
			var wg sync.WaitGroup
			for {
				s := <-rawEvents

				// Break on these event targets so that updates are applied in order.
				if strings.HasSuffix(s, "-") {
					break
				}

				if s != "" {
					wg.Add(1)
					go func(s string) {
						defer wg.Done()
						e, err := resource.NewEvent(s)
						if err != nil {
							errors <- err
						}
						events <- e
					}(s)
				}
			}
			wg.Wait()
		}
	}()

	display := display.NewUglyPrinter()

	display.Display(events, errors)
}
