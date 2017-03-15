package display

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"

	"drbdtop.io/drbdtop/pkg/resource"
	"drbdtop.io/drbdtop/pkg/update"
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

	u.resources.OrderBy(update.Danger, update.SizeReverse, update.NameReverse)
	for {
		c := exec.Command("clear")
		c.Stdout = os.Stdout
		c.Run()

		u.resources.RLock()
		for _, r := range u.resources.List {
			fmt.Printf("%s (%d):\n", r.Res.Name, r.Danger)
			fmt.Printf("\tRole: %s Suspended: %s WriteOrdering: %s\n", r.Res.Role, r.Res.Suspended, r.Res.WriteOrdering)

			fmt.Printf("\n")

			fmt.Printf("\tLocal Disk:\n")

			d := r.Device
			var keys []string
			for k := range d.Volumes {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("\t\tvolume %s: diskState: %s site: %d blocked: %s minor: %s readKiB/Sec: %.1f total read KiB %d writtenKiB/Sec: %.1f total written KiB %d LowerPedning (%d %d %.1f %d) min/max/avg/current\n",
					k, d.Volumes[k].DiskState, d.Volumes[k].Size, d.Volumes[k].Blocked, d.Volumes[k].Minor,
					d.Volumes[k].ReadKiB.PerSecond, d.Volumes[k].ReadKiB.Total, d.Volumes[k].WrittenKiB.PerSecond, d.Volumes[k].WrittenKiB.Total,
					d.Volumes[k].LowerPending.Min, d.Volumes[k].LowerPending.Max, d.Volumes[k].LowerPending.Avg, d.Volumes[k].LowerPending.Current)
			}

			fmt.Printf("\n")

			var connKeys []string
			for j := range r.Connections {
				connKeys = append(connKeys, j)
			}
			sort.Strings(connKeys)

			for _, conn := range connKeys {
				if c, ok := r.Connections[conn]; ok {
					fmt.Printf("\tConnection to %s: Status: %s Role: %s Congested: %s\n", c.ConnectionName, c.ConnectionStatus, c.Role, c.Congested)

					if d, ok := r.PeerDevices[conn]; ok {
						fmt.Printf("\t%s's device:\n", d.ConnectionName)
						var keys []string
						for k := range d.Volumes {
							keys = append(keys, k)
						}
						sort.Strings(keys)
						for _, k := range keys {
							fmt.Printf("\t\tvolume %s: repStatus: %s resyncSuspended: %s disk state:: %s out-of-sync: %d %d %.1f %d (min/max/avg/current) send KiB/Sec: %.1f total sent KiB %d\n",
								k, d.Volumes[k].ReplicationStatus, d.Volumes[k].ResyncSuspended, d.Volumes[k].DiskState,
								d.Volumes[k].OutOfSyncKiB.Min, d.Volumes[k].OutOfSyncKiB.Max, d.Volumes[k].OutOfSyncKiB.Avg, d.Volumes[k].OutOfSyncKiB.Current,
								d.Volumes[k].SentKiB.PerSecond, d.Volumes[k].SentKiB.Total)
						}
					}
					fmt.Printf("\n")
				}
			}
		}
		u.resources.RUnlock()
		fmt.Printf("\n")
		fmt.Println("Errors:")
		for _, e := range u.lastErr {
			fmt.Printf("%v\n", e)
		}
		if done {
			return
		}
		time.Sleep(time.Millisecond * 50)
	}
}
