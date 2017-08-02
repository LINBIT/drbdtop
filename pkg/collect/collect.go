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

package collect

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/linbit/drbdtop/pkg/resource"
	"github.com/linbit/godrbdutils"
)

// Collector sends raw event strings into a channel.
type Collector interface {
	Collect(events chan<- resource.Event, errors chan<- error)
}

// FileCollector gathers newline delimited events from a plaintext file.
type FileCollector struct {
	Path *string
}

func (c FileCollector) Collect(events chan<- resource.Event, errors chan<- error) {
	f, err := os.Open(*c.Path)
	if err != nil {
		errors <- err
	}
	defer f.Close()

	displayEvent := resource.NewDisplayEvent()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		e := scanner.Text()
		evt, err := resource.NewEvent(e)
		if err != nil {
			errors <- err
		} else {
			events <- evt
		}
		events <- displayEvent
	}
	events <- resource.NewEOF()
}

// Events2Poll continuously calls drbdsetup events2 at a specified Interval.
type Events2Poll struct {
	// Interval to wait between calls to drbdsetup devents2
	Interval time.Duration
}

func (c Events2Poll) Collect(events chan<- resource.Event, errors chan<- error) {
	ticker := time.NewTicker(c.Interval)
	displayEvent := resource.NewDisplayEvent()
	for {
		remainingResources, err := allResources()
		if err != nil {
			errors <- err
		}
		out, err := exec.Command("drbdsetup", "events2", "--timestamps", "--statistics", "--now").CombinedOutput()
		if err != nil {
			errors <- err
			events <- resource.NewEOF()
		} else {
			s := string(out)
			for _, e := range strings.Split(s, "\n") {
				if e != "" {
					evt, err := resource.NewEvent(e)
					if err != nil {
						errors <- err
					} else {
						delete(remainingResources, evt.Fields[resource.ResKeys.Name])
						events <- evt
					}
				}
			}
		}
		for res := range remainingResources {
			events <- resource.NewUnconfiguredRes(res)
		}
		events <- displayEvent
		<-ticker.C
	}
}

func allResources() (map[string]bool, error) {
	cmd, err := godrbdutils.NewDrbdCmd(godrbdutils.Drbdadm, godrbdutils.Connect, []string{"all"}, "-d")
	if err != nil {
		return nil, fmt.Errorf("unable to find all reources: %v", err)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("unable to find all reources: %v", err)
	}
	return doAllResources(string(out))
}

func doAllResources(s string) (map[string]bool, error) {
	ret := map[string]bool{}
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		valid := false

		if len(f) == 4 && f[0] == "drbdsetup" { // drbd 9
			valid = true
		} else if len(f) == 5 && f[0] == "drbdsetup-84" { // drbd 8.4
			valid = true
		}

		if valid {
			ret[f[2]] = true
		}
	}
	return ret, nil
}
