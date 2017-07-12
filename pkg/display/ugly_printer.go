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

package display

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/linbit/drbdtop/pkg/convert"
	"github.com/linbit/drbdtop/pkg/resource"
	"github.com/linbit/drbdtop/pkg/update"
)

// UglyPrinter is the bare minimum screen printer.
type UglyPrinter struct {
	resources *update.ResourceCollection
	lastErr   []error
}

func NewUglyPrinter(d time.Duration) UglyPrinter {
	return UglyPrinter{resources: update.NewResourceCollection(d)}
}

// Display clears the screen and resource information in a loop.
func (u *UglyPrinter) Display(event <-chan resource.Event, err <-chan error) {
	done := false
	go func() {
		for {
			select {
			case evt := <-event:
				if evt.Target == resource.EOF {
					done = true
				}
				u.resources.Update(evt)
			case err := <-err:
				if len(u.lastErr) >= 5 {
					u.lastErr = append(u.lastErr[1:], err)
				} else {
					u.lastErr = append(u.lastErr, err)
				}
			}
		}
	}()

	u.resources.OrderBy(update.Danger, update.Size, update.Name)
	ticker := time.NewTicker(time.Millisecond * 50)
	for {
		c := exec.Command("clear")
		c.Stdout = os.Stdout
		c.Run()

		u.resources.RLock()
		for _, r := range u.resources.List {
			printByRes(r)
		}
		fmt.Printf("\n")
		fmt.Println("Errors:")
		for _, e := range u.lastErr {
			fmt.Printf("%v\n", e)
		}
		if done {
			return
		}
		u.resources.RUnlock()
		<-ticker.C
	}
}

func printByRes(r *update.ByRes) {

	printRes(r)
	fmt.Printf("\n")

	// Nothing more to do if the resource is down, exit.
	if r.Res.Unconfigured {
		return
	}

	printLocalDisk(r)
	fmt.Printf("\n")

	var connKeys []string
	for j := range r.Connections {
		connKeys = append(connKeys, j)
	}
	sort.Strings(connKeys)

	for _, conn := range connKeys {
		if c, ok := r.Connections[conn]; ok {

			printConn(c)

			if _, ok := r.PeerDevices[conn]; ok {
				printPeerDev(r, conn)
			}
			fmt.Printf("\n")
		}
	}
}

func printRes(r *update.ByRes) {
	fmt.Printf("%s: (%d) ", r.Res.Name, r.Danger)

	c := color.New(color.FgHiRed)
	if r.Res.Suspended == "yes" {
		c.Printf("(Suspended)")
	}

	if r.Res.Unconfigured {
		c.Printf("(Down)")
	}

	fmt.Printf("\n")
}

func printLocalDisk(r *update.ByRes) {
	h2 := color.New(color.FgHiCyan)

	roleColor := color.New(color.FgHiGreen).SprintFunc()
	if r.Res.Role == "Unknown" {
		roleColor = color.New(color.FgHiYellow).SprintFunc()
	}

	h2.Printf("\tLocal Disk(%s):\n", roleColor(r.Res.Role))

	d := r.Device

	var keys []string
	for k := range d.Volumes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		dColor := dangerColor(d.Danger).SprintFunc()
		v := d.Volumes[k]
		fmt.Printf("\t\tvolume %s (/dev/drbd%s):", dColor(k), v.Minor)
		dState := v.DiskState

		if dState != "UpToDate" {
			c := color.New(color.FgHiYellow)
			c.Printf(" %s", dState)

			fmt.Printf("(%s)", v.DiskHint)
		}

		if v.Blocked != "no" {
			c := color.New(color.FgHiYellow)
			c.Printf(" Blocked: %s ", d.Volumes[k].Blocked)
		}

		if v.ActivityLogSuspended != "no" {
			c := color.New(color.FgHiYellow)
			c.Printf(" Activity Log Suspended: %s ", d.Volumes[k].Blocked)
		}

		fmt.Printf("\n")
		fmt.Printf("\t\t\tsize: %s total-read:%s read/Sec:%s total-written:%s written/Sec:%s ",
			convert.KiB2Human(float64(v.Size)),
			convert.KiB2Human(float64(v.ReadKiB.Total)), convert.KiB2Human(v.ReadKiB.PerSecond),
			convert.KiB2Human(float64(v.WrittenKiB.Total)), convert.KiB2Human(v.WrittenKiB.PerSecond))

		fmt.Printf("\n")
	}

}

func printConn(c *resource.Connection) {
	h2 := color.New(color.FgHiCyan)
	h2.Printf("\tConnection to %s", c.ConnectionName)

	cl := color.New()

	roleColor := color.New(color.FgHiGreen).SprintFunc()
	if c.Role == "Unknown" {
		roleColor = color.New(color.FgHiYellow).SprintFunc()
	}
	fmt.Printf("(%s):", roleColor(c.Role))

	if c.ConnectionStatus != "Connected" {
		cl = color.New(color.FgHiYellow)
		if c.ConnectionStatus == "StandAlone" {
			cl = color.New(color.FgHiRed)

			cl.Printf("%s", c.ConnectionStatus)
			fmt.Printf("(%s)", c.ConnectionHint)
		}
	}

	if c.Congested != "no" {
		cl = color.New(color.FgHiYellow)
		cl.Printf(" Congested ")
	}

	fmt.Printf("\n")
}

func printPeerDev(r *update.ByRes, conn string) {
	d := r.PeerDevices[conn]

	var keys []string
	for k := range d.Volumes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := d.Volumes[k]
		fmt.Printf("\t\tvolume %s: ", k)
		cl := color.New(color.FgWhite)

		if v.ResyncSuspended != "no" {
			cl = color.New(color.FgHiYellow)
			cl.Printf(" ResyncSuspended:%s ", v.ResyncSuspended)
		}
		fmt.Printf("\n")

		if v.ReplicationStatus != "Established" {
			cl = color.New(color.FgHiYellow)
			cl.Printf("\t\t\tReplication:%s", v.ReplicationStatus)
			fmt.Printf("(%s)", v.ReplicationHint)
		}

		if strings.HasPrefix(v.ReplicationStatus, "Sync") {
			fmt.Printf(" %.1f%% remaining",
				(float64(v.OutOfSyncKiB.Current)/float64(r.Device.Volumes[k].Size))*100)
		}

		if v.DiskState != "UpToDate" {
			c := color.New(color.FgHiYellow)
			c.Printf("\n\t\t\t%s", v.DiskState)

			fmt.Printf("(%s)", v.DiskHint)
		}

		fmt.Printf("\n")

		fmt.Printf("\t\t\tSent: total:%s Per/Sec:%s\n",
			convert.KiB2Human(float64(v.SentKiB.Total)), convert.KiB2Human(v.SentKiB.PerSecond))

		fmt.Printf("\t\t\tReceived: total:%s Per/Sec:%s\n",
			convert.KiB2Human(float64(v.ReceivedKiB.Total)), convert.KiB2Human(v.ReceivedKiB.PerSecond))

		oosCl := dangerColor(v.OutOfSyncKiB.Current / uint64(1024)).SprintFunc()
		oosAvgCl := dangerColor(uint64(v.OutOfSyncKiB.Avg) / uint64(1024)).SprintFunc()
		oosMinCl := dangerColor(v.OutOfSyncKiB.Min / uint64(1024)).SprintFunc()
		oosMaxCl := dangerColor(v.OutOfSyncKiB.Max / uint64(1024)).SprintFunc()
		fmt.Printf("\t\t\tOutOfSync: current:%s average:%s min:%s max:%s\n",
			oosCl(convert.KiB2Human(float64(v.OutOfSyncKiB.Current))),
			oosAvgCl(convert.KiB2Human(v.OutOfSyncKiB.Avg)),
			oosMinCl(convert.KiB2Human(float64(v.OutOfSyncKiB.Min))),
			oosMaxCl(convert.KiB2Human(float64(v.OutOfSyncKiB.Max))))

		penCl := dangerColor(v.PendingWrites.Current).SprintFunc()
		penAvgCl := dangerColor(uint64(v.PendingWrites.Avg)).SprintFunc()
		penMinCl := dangerColor(v.PendingWrites.Min).SprintFunc()
		penMaxCl := dangerColor(v.PendingWrites.Max).SprintFunc()
		fmt.Printf("\t\t\tPendingWrites: current:%s average:%s min:%s max:%s\n",
			penCl(v.PendingWrites.Current),
			penAvgCl(fmt.Sprintf("%.1f", v.PendingWrites.Avg)),
			penMinCl(v.PendingWrites.Min),
			penMaxCl(v.PendingWrites.Max))

		unAckCl := dangerColor(v.UnackedWrites.Current).SprintFunc()
		unAckAvgCl := dangerColor(uint64(v.UnackedWrites.Avg)).SprintFunc()
		unAckMinCl := dangerColor(v.UnackedWrites.Min).SprintFunc()
		unAckMaxCl := dangerColor(v.UnackedWrites.Max).SprintFunc()
		fmt.Printf("\t\t\tUnackedWrites: current:%s average:%s min:%s max:%s\n",
			unAckCl(v.UnackedWrites.Current),
			unAckAvgCl(fmt.Sprintf("%.1f", v.UnackedWrites.Avg)),
			unAckMinCl(v.UnackedWrites.Min),
			unAckMaxCl(v.UnackedWrites.Max))

		fmt.Printf("\n")
	}
}

func dangerColor(danger uint64) *color.Color {
	if danger == 0 {
		return color.New(color.FgHiGreen)
	} else if danger < 10000 {
		return color.New(color.FgHiYellow)
	} else {
		return color.New(color.FgHiRed)
	}
}
