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
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/hayswim/drbdtop/pkg/display"
	"github.com/hayswim/drbdtop/pkg/resource"
)

func main() {
	file := flag.String("file", "", "Path to a file containing output gathered from polling `drbdsetup events2 --timestamps --statistics --now`")

	flag.Parse()

	rawEvents := make(chan string)

	errors := make(chan error, 100)
	if *file != "" {
		f, err := os.Open(*file)
		if err != nil {
			fmt.Printf("%v\n", err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		go func() {
			for scanner.Scan() {
				rawEvents <- scanner.Text()
			}
			close(rawEvents)
		}()

	} else {
		go func() {
			for {
				out, err := exec.Command("drbdsetup", "events2", "--timestamps", "--statistics", "--now").CombinedOutput()
				if err != nil {
					errors <- err
				} else {
					s := string(out)
					for _, rawEvent := range strings.Split(s, "\n") {
						if rawEvent != "" {
							rawEvents <- rawEvent
						}
					}
				}
				time.Sleep(time.Millisecond * 500)
			}
		}()
	}

	events := make(chan resource.Event, 5)

	display := display.NewUglyPrinter()
	go display.Display(events, errors)

	// Main update loop.
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
				// Update logic goes here.
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
}
