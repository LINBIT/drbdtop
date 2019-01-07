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

package update

import (
	"sort"
	"sync"
	"time"

	"github.com/LINBIT/drbdtop/pkg/resource"
	"github.com/facette/natsort"
)

// ByRes organizes events related to a particular resource.
type ByRes struct {
	sync.RWMutex
	Res         *resource.Resource
	Connections map[string]*resource.Connection
	Device      *resource.Device
	PeerDevices map[string]*resource.PeerDevice
	// Aggregate danger score from all connections, peer devices, and the local device.
	Danger uint64
}

// NewByRes returns an empty ByRes that's ready to be Updated.
func NewByRes() *ByRes {
	return &ByRes{
		Res:         &resource.Resource{},
		Connections: make(map[string]*resource.Connection),
		Device:      resource.NewDevice(),
		PeerDevices: make(map[string]*resource.PeerDevice),
	}
}

// Update a ByRes with a new Event's data.
func (b *ByRes) Update(evt resource.Event) {
	b.Lock()
	defer b.Unlock()

	switch evt.Target {
	case "resource":
		b.Res.Update(evt)

	case "device":
		b.Device.Update(evt)

	case "connection":
		conn := evt.Fields[resource.ConnKeys.ConnName]

		if _, ok := b.Connections[conn]; !ok {
			b.Connections[conn] = &resource.Connection{}
		}
		b.Connections[conn].Update(evt)

	case "peer-device":
		conn := evt.Fields[resource.ConnKeys.ConnName]

		if _, ok := b.PeerDevices[conn]; !ok {
			b.PeerDevices[conn] = resource.NewPeerDevice()
		}
		b.PeerDevices[conn].Update(evt)
	default:
		// Unknown event target, ignore it.
		_ = evt
	}

	b.setDanger()
}

func (b *ByRes) setDanger() {
	var dangerScore uint64

	dangerScore += b.Res.Danger

	for _, c := range b.Connections {
		dangerScore += c.Danger
	}

	dangerScore += b.Device.Danger

	for _, p := range b.PeerDevices {
		dangerScore += p.Danger
	}

	b.Danger = dangerScore
}

// Remove old connections, devices, and peer devices and their volumes
// that haven't been updated since time.
func (b *ByRes) prune(t time.Time) {
	for k, c := range b.Connections {
		if c.CurrentTime.Before(t) {
			delete(b.Connections, k)
		}
	}
	for k, v := range b.Device.Volumes {
		if v.CurrentTime.Before(t) {
			delete(b.Device.Volumes, k)
		}
	}
	for k, p := range b.PeerDevices {
		if p.CurrentTime.Before(t) {
			delete(b.PeerDevices, k)
		} else {
			for key, v := range p.Volumes {
				if v.CurrentTime.Before(t) {
					delete(p.Volumes, key)
				}
			}
		}
	}
}

// ResourceCollection is a collection of stats collected organized under their respective resource names.
// Implements the Sort interface, sorting the *ByRes within List.
type ResourceCollection struct {
	sync.RWMutex
	Map            map[string]*ByRes
	List           []*ByRes
	less           []LessFunc
	updateInterval time.Duration
}

// NewResourceCollection returns a new *ResourceCollection with maps created
// and configured to sort by Name only.
func NewResourceCollection(d time.Duration) *ResourceCollection {
	return &ResourceCollection{
		Map:            make(map[string]*ByRes),
		less:           []LessFunc{Name},
		updateInterval: d,
	}
}

// Update a collection of ByRes from an Event.
func (rc *ResourceCollection) Update(e resource.Event) {
	rc.Lock()
	defer rc.Unlock()

	// Clean up old data.
	if rc.updateInterval != 0 {
		rc.prune(e.TimeStamp.Add(-3 * rc.updateInterval))
	}

	resName := e.Fields[resource.ResKeys.Name]
	if resName != "" {
		resource, ok := rc.Map[resName]
		if !ok {
			resource = NewByRes()
			rc.Map[resName] = resource
		}
		resource.Update(e)
	}
}

func (rc *ResourceCollection) UpdateList() {
	rc.Lock()
	defer rc.Unlock()

	// Rebuild list from map values.
	rc.List = []*ByRes{}
	for _, v := range rc.Map {
		rc.List = append(rc.List, v)
	}

	rc.sort()
}

// Remove old fields that haven't been updated since time.
func (rc *ResourceCollection) prune(t time.Time) {
	for k, v := range rc.Map {
		if v.Res.CurrentTime.Before(t) {
			delete(rc.Map, k)
		} else {
			v.prune(t)
		}
	}
}

// Sorting code adapted from https://golang.org/pkg/sort/#example__sortMultiKeys

// LessFunc determines if p1 should come before p2 during a sort.
type LessFunc func(p1, p2 *ByRes) bool

// The actual sorting, call this one *only* if you already hold the lock
func (rc *ResourceCollection) sort() {
	sort.Sort(rc)
}

// Sort sorts the argument slice according to the less functions passed to OrderedBy.
func (rc *ResourceCollection) Sort() {
	rc.Lock()
	rc.sort()
	rc.Unlock()
}

// OrderBy replaces the less functions used to sort, in order.
// Call its Sort method to sort the data.
func (rc *ResourceCollection) OrderBy(less ...LessFunc) {
	rc.Lock()
	defer rc.Unlock()
	rc.less = less
}

// Len is part of sort.Interface.
func (rc *ResourceCollection) Len() int {
	return len(rc.List)
}

// Swap is part of sort.Interface.
func (rc *ResourceCollection) Swap(i, j int) {
	rc.List[i], rc.List[j] = rc.List[j], rc.List[i]
}

// Less is part of sort.Interface. It is implemented by looping along the
// less functions until it finds a comparison that is either Less or
// !Less. Note that it can call the less functions twice per call. We
// could change the functions to return -1, 0, 1 and reduce the
// number of calls for greater efficiency: an exercise for the reader.
func (rc *ResourceCollection) Less(i, j int) bool {
	p, q := rc.List[i], rc.List[j]
	// Try all but the last comparison.
	var k int
	for k = 0; k < len(rc.less)-1; k++ {
		less := rc.less[k]
		switch {
		case less(p, q):
			// p < q, so we have a decision.
			return true
		case less(q, p):
			// p > q, so we have a decision.
			return false
		}
		// p == q; try the next comparison.
	}
	// All comparisons to here said "equal", so just return whatever
	// the final comparison reports.
	return rc.less[k](p, q)
}

// Name sorts resource names by natual sorting order.
func Name(r1, r2 *ByRes) bool {
	return natsort.Compare(r1.Res.Name, r2.Res.Name)
}

// NameReverse sorts resource names by reverse natual sorting order.
func NameReverse(r1, r2 *ByRes) bool {
	// If we simply negate the comparison function, we'll swap elements that are equal.
	// Names have to be unique, so we can get away with it here.
	return !Name(r1, r2)
}

// Size sorts resource names by local disk size.
func Size(r1, r2 *ByRes) bool {
	return localSize(r1) < localSize(r2)
}

// SizeReverse sorts resource names by local disk size in reverse order.
func SizeReverse(r1, r2 *ByRes) bool {
	return localSize(r1) > localSize(r2)
}

func localSize(b *ByRes) uint64 {
	var size uint64
	for _, v := range b.Device.Volumes {
		size += v.Size
	}

	return size
}

// Danger sorts resource names by danger.
func Danger(r1, r2 *ByRes) bool {
	return r1.Danger < r2.Danger
}

// DangerReverse sorts resource names by danger in reverse order.
func DangerReverse(r1, r2 *ByRes) bool {
	return r1.Danger > r2.Danger
}
