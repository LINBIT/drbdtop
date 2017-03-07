package display

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"

	"drbdtop.io/drbdtop/pkg/resource"
)

// UglyPrinter is the bare minimum screen printer.
type UglyPrinter struct {
	resources   map[string]*resource.Resource
	connections map[string]map[string]*resource.Connection
	devices     map[string]*resource.Device
	peerDevices map[string]map[string]*resource.PeerDevice
	lastErr     []error
}

func NewUglyPrinter() UglyPrinter {
	return UglyPrinter{
		resources:   make(map[string]*resource.Resource),
		connections: make(map[string]map[string]*resource.Connection),
		devices:     make(map[string]*resource.Device),
		peerDevices: make(map[string]map[string]*resource.PeerDevice),
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
						u.connections[evt.Fields["name"]] = make(map[string]*resource.Connection)
					}

					_, ok = u.connections[evt.Fields["name"]][evt.Fields["conn-name"]]
					if !ok {
						u.connections[evt.Fields["name"]][evt.Fields["conn-name"]] = &resource.Connection{}
					}

					u.connections[evt.Fields["name"]][evt.Fields["conn-name"]].Update(evt)
				case "device":
					_, ok := u.devices[evt.Fields["name"]]
					if !ok {
						u.devices[evt.Fields["name"]] = resource.NewDevice()
					}
					u.devices[evt.Fields["name"]].Update(evt)
				case "peer-device":
					_, ok := u.peerDevices[evt.Fields["name"]]
					if !ok {
						u.peerDevices[evt.Fields["name"]] = make(map[string]*resource.PeerDevice)
					}

					_, ok = u.peerDevices[evt.Fields["name"]][evt.Fields["conn-name"]]
					if !ok {
						u.peerDevices[evt.Fields["name"]][evt.Fields["conn-name"]] = resource.NewPeerDevice()
					}

					u.peerDevices[evt.Fields["name"]][evt.Fields["conn-name"]].Update(evt)
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

		var keys []string
		for k := range u.resources {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			u.resources[k].RLock()
			fmt.Printf("%s:\n", k)
			fmt.Printf("\tRole: %s Suspended: %s WriteOrdering: %s\n", u.resources[k].Role, u.resources[k].Suspended, u.resources[k].WriteOrdering)
			u.resources[k].RUnlock()

			fmt.Printf("\n")

			if _, ok := u.connections[k]; ok {
				var connKeys []string
				for j := range u.connections[k] {
					connKeys = append(connKeys, j)
				}
				sort.Strings(connKeys)

				for _, conn := range connKeys {
					if c, ok := u.connections[k][conn]; ok {
						c.RLock()
						fmt.Printf("\tConnection to %s: Status: %s Role: %s Congested: %s\n", c.ConnectionName, c.ConnectionStatus, c.Role, c.Congested)
						c.RUnlock()

						if d, ok := u.peerDevices[k][conn]; ok {
							d.RLock()
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
							d.RUnlock()
						}
						fmt.Printf("\n")
					}
				}
			}

			if d, ok := u.devices[k]; ok {
				fmt.Printf("\tLocal Disk:\n")
				d.RLock()
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
				d.RUnlock()
			}
			fmt.Printf("\n")
		}
		fmt.Printf("\n")
		fmt.Println("Errors:")
		for _, e := range u.lastErr {
			fmt.Printf("%v\n", e)
		}
		time.Sleep(time.Millisecond * 50)
	}
}
