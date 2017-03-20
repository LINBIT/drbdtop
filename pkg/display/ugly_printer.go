package display

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"time"

	"drbdtop.io/drbdtop/pkg/resource"
	"drbdtop.io/drbdtop/pkg/update"
	"github.com/fatih/color"
)

// UglyPrinter is the bare minimum screen printer.
type UglyPrinter struct {
	resources *update.ResourceCollection
	lastErr   []error
}

func NewUglyPrinter() UglyPrinter {
	return UglyPrinter{
		resources: update.NewResourceCollection(),
	}
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
		time.Sleep(time.Millisecond * 50)
	}
}

func printByRes(r *update.ByRes) {

	printRes(r)
	fmt.Printf("\n")

	printLocalDisk(r.Device)
	fmt.Printf("\n")

	var connKeys []string
	for j := range r.Connections {
		connKeys = append(connKeys, j)
	}
	sort.Strings(connKeys)

	for _, conn := range connKeys {
		if c, ok := r.Connections[conn]; ok {

			printConn(c)

			if d, ok := r.PeerDevices[conn]; ok {
				printPeerDev(d)
			}
			fmt.Printf("\n")
		}
	}
}

func printRes(r *update.ByRes) {
	dColor := dangerColor(r.Danger).Add(color.Italic).SprintFunc()
	fmt.Printf("%s (%d): %s ", dColor(r.Res.Name), r.Danger, r.Res.Role)

	if r.Res.Suspended != "no" {
		c := color.New(color.FgHiRed)
		c.Printf("(Suspended)")
	}

	fmt.Printf("\n")
}

func printLocalDisk(d *resource.Device) {
	h2 := color.New(color.FgHiCyan)
	h2.Printf("\tLocal Disk:\n")

	var keys []string
	for k := range d.Volumes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		dColor := dangerColor(d.Danger).SprintFunc()
		v := d.Volumes[k]
		fmt.Printf("\t\tvolume %s (/dev/drbd/%s):", dColor(k), v.Minor)
		dState := v.DiskState

		if dState != "UpToDate" {
			c := color.New(color.FgHiYellow)
			c.Printf(" %s ", dState)
		} else {
			fmt.Printf(" %s ", dState)
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
			kib2Human(float64(v.Size)),
			kib2Human(float64(v.ReadKiB.Total)), kib2Human(v.ReadKiB.PerSecond),
			kib2Human(float64(v.WrittenKiB.Total)), kib2Human(v.WrittenKiB.PerSecond))

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

	if c.Congested == "no" {
		cl = color.New(color.FgHiYellow)
		cl.Printf(" Congested ")
	}

	fmt.Printf("\n")
}

func printPeerDev(d *resource.PeerDevice) {
	dColor := dangerColor(d.Danger).SprintFunc()
	var keys []string
	for k := range d.Volumes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := d.Volumes[k]
		fmt.Printf("\t\tvolume %s: ", k)
		cl := color.New(color.FgWhite)

		if v.ReplicationStatus != "Established" {
			cl = color.New(color.FgHiYellow)
			cl.Printf("Replication:%s", v.ReplicationStatus)
			fmt.Printf("(%s)", v.ReplicationHint)
		}

		if v.ResyncSuspended != "no" {
			cl = color.New(color.FgHiYellow)
			cl.Printf(" ResyncSuspended:%s ", v.ResyncSuspended)
		}
		fmt.Printf("\n")

		fmt.Printf("\t\t\tSent: total:%s Per/Sec:%s\n",
			kib2Human(float64(v.SentKiB.Total)), kib2Human(v.SentKiB.PerSecond))

		fmt.Printf("\t\t\tReceived: total:%s Per/Sec:%s\n",
			kib2Human(float64(v.ReceivedKiB.Total)), kib2Human(v.ReceivedKiB.PerSecond))

		dColor = dangerColor(v.OutOfSyncKiB.Current / uint64(1024)).SprintFunc()
		fmt.Printf("\t\t\tOutOfSync: current:%s average:%s min:%s max%s\n",
			dColor(kib2Human(float64(v.OutOfSyncKiB.Current))),
			dColor(kib2Human(v.OutOfSyncKiB.Avg)),
			dColor(kib2Human(float64(v.OutOfSyncKiB.Min))),
			dColor(kib2Human(float64(v.OutOfSyncKiB.Max))))

		dColor = dangerColor(v.PendingWrites.Current).SprintFunc()
		fmt.Printf("\t\t\tPendingWrites: current:%s average:%s min:%s max%s\n",
			dColor(kib2Human(float64(v.PendingWrites.Current))),
			dColor(kib2Human(v.PendingWrites.Avg)),
			dColor(kib2Human(float64(v.PendingWrites.Min))),
			dColor(kib2Human(float64(v.PendingWrites.Max))))

		dColor = dangerColor(v.UnackedWrites.Current).SprintFunc()
		fmt.Printf("\t\t\tUnackedWrites: current:%s average:%s min:%s max%s\n",
			dColor(kib2Human(float64(v.UnackedWrites.Current))),
			dColor(kib2Human(v.UnackedWrites.Avg)),
			dColor(kib2Human(float64(v.UnackedWrites.Min))),
			dColor(kib2Human(float64(v.UnackedWrites.Max))))

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

// kib2Human takes a size in KiB and returns a human readable size with suffix.
func kib2Human(kiBytes float64) string {
	sizes := []string{"KiB", "Mib", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"}
	unit := float64(1024)
	if kiBytes < unit {
		return fmt.Sprintf("%.1f%s", kiBytes, sizes[0])
	}

	exp := int(math.Log(kiBytes) / math.Log(unit))
	return fmt.Sprintf("%.1f%s", (kiBytes / (math.Pow(unit, float64(exp)))), sizes[exp])
}
