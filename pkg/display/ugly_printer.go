package display

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"drbdtop.io/drbdtop/pkg/resource"
)

// UglyPrinter is the bare minimum screen printer.
type UglyPrinter struct {
	resources   map[string]*resource.Resource
	connections map[string]*resource.Connection
	devices     map[string]*resource.Device
	peerDevices map[string]*resource.PeerDevice
	lastErr     []error
}

func NewUglyPrinter() UglyPrinter {
	return UglyPrinter{
		resources:   make(map[string]*resource.Resource),
		connections: make(map[string]*resource.Connection),
		devices:     make(map[string]*resource.Device),
		peerDevices: make(map[string]*resource.PeerDevice),
	}
}

// Display clears the screen and resource information in a loop.
func (u *UglyPrinter) Display(event <-chan resource.Event, err <-chan error) {
	go func() {
		for {
			select {
			case evt := <-event:
				switch evt.Target {
				case "resource":
					_, ok := u.resources[evt.Fields["name"]]
					if !ok {
						u.resources[evt.Fields["name"]] = &resource.Resource{}
					}
					u.resources[evt.Fields["name"]].Update(evt)
				case "connection":
					_, ok := u.connections[evt.Fields["name"]]
					if !ok {
						u.connections[evt.Fields["name"]] = &resource.Connection{}
					}
					u.connections[evt.Fields["name"]].Update(evt)
				case "device":
					_, ok := u.devices[evt.Fields["name"]]
					if !ok {
						u.devices[evt.Fields["name"]] = resource.NewDevice()
					}
					u.devices[evt.Fields["name"]].Update(evt)
				case "peer-device":
					_, ok := u.peerDevices[evt.Fields["name"]]
					if !ok {
						u.peerDevices[evt.Fields["name"]] = resource.NewPeerDevice()
					}
					u.peerDevices[evt.Fields["name"]].Update(evt)
				}
			case err := <-err:
				if len(u.lastErr) >= 5 {
					u.lastErr = append(u.lastErr[1:], err)
				} else {
					u.lastErr = append(u.lastErr, err)
				}
			}
		}
	}()

	for {
		c := exec.Command("clear")
		c.Stdout = os.Stdout
		c.Run()

		for k, v := range u.resources {
			v.RLock()
			fmt.Printf("%s:\n", v.Name)
			fmt.Printf("\tRole: %s Suspended: %s WriteOrdering: %s\n", v.Role, v.Suspended, v.WriteOrdering)
			v.RUnlock()

			fmt.Printf("\n")

			if c, ok := u.connections[k]; ok {
				c.RLock()
				fmt.Printf("\tConnection to %s: Status: %s Role: %s Congested: %s\n", c.ConnectionName, c.ConnectionStatus, c.Role, c.Congested)
				c.RUnlock()
			}

			fmt.Printf("\n")

			if d, ok := u.devices[k]; ok {
				fmt.Printf("\tLocal Disk:\n")
				d.RLock()
				for k, v := range d.Volumes {
					fmt.Printf("\t\tvolume %s: diskState: %s site: %d blocked: %s minor: %s readKiB/Sec: %.1f total read KiB %d writtenKiB/Sec: %.1f total written KiB %d LowerPedning (%d %d %.1f %d) min/max/avg/current\n",
						k, v.DiskState, v.Size, v.Blocked, v.Minor,
						v.ReadKiB.PerSecond, v.ReadKiB.Total, v.WrittenKiB.PerSecond, v.WrittenKiB.Total,
						v.LowerPending.Min, v.LowerPending.Max, v.LowerPending.Avg, v.LowerPending.Current)
				}
				d.RUnlock()
			}

			fmt.Printf("\n")

			if d, ok := u.peerDevices[k]; ok {
				d.RLock()
				fmt.Printf("\t%s's device:\n", d.ConnectionName)
				for k, v := range d.Volumes {
					fmt.Printf("\t\tvolume %s: repStatus: %s resyncSuspended: %s disk state:: %s out-of-sync: %d %d %.1f %d (min/max/avg/current) send KiB/Sec: %.1f total sent KiB %d\n",
						k, v.ReplicationStatus, v.ResyncSuspended, v.DiskState,
						v.OutOfSyncKiB.Min, v.OutOfSyncKiB.Max, v.OutOfSyncKiB.Avg, v.OutOfSyncKiB.Current,
						v.SentKiB.PerSecond, v.SentKiB.Total)
				}
				d.RUnlock()
			}
		}
		fmt.Printf("\n")
		fmt.Println("Errors:")
		for _, e := range u.lastErr {
			fmt.Printf("%v\n", e)
		}
		time.Sleep(time.Millisecond * 50)
	}
}
