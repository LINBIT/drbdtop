package update

import (
	"sort"
	"sync"

	"drbdtop.io/drbdtop/pkg/resource"
)

// ByRes organizes events related to a particular resource.
type ByRes struct {
	sync.RWMutex
	Res         *resource.Resource
	Connections map[string]*resource.Connection
	Device      *resource.Device
	PeerDevices map[string]*resource.PeerDevice
	Danger      uint64
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
		conn := evt.Fields["conn-name"]

		if _, ok := b.Connections[conn]; !ok {
			b.Connections[conn] = &resource.Connection{}
		}
		b.Connections[conn].Update(evt)

	case "peer-device":
		conn := evt.Fields["conn-name"]

		if _, ok := b.PeerDevices[conn]; !ok {
			b.PeerDevices[conn] = resource.NewPeerDevice()
		}
		b.PeerDevices[conn].Update(evt)
	default:
		// Unknown event target, ignore it.
		_ = evt
	}

	// Calculate Danger.
	var dangerScore uint64

	for _, c := range b.Connections {
		dangerScore += c.Danger
	}

	dangerScore += b.Device.Danger

	for _, p := range b.PeerDevices {
		dangerScore += p.Danger
	}

	b.Danger = dangerScore
}

// ResourceCollection is a collection of stats collected organized under their respective resource names.
// Implements the Sort interface, sorting the *ByRes within List.
type ResourceCollection struct {
	sync.RWMutex
	Map  map[string]*ByRes
	List []*ByRes
	less []LessFunc
}

// NewResourceCollection returns a new *ResourceCollection with maps created
// and configured to sort by Name only.
func NewResourceCollection() *ResourceCollection {
	return &ResourceCollection{
		Map:  make(map[string]*ByRes),
		less: []LessFunc{Name},
	}
}

// Update a collection of ByRes from an Event.
func (rc *ResourceCollection) Update(e resource.Event) {
	rc.Lock()

	resName := e.Fields["name"]
	if resName != "" {
		if _, ok := rc.Map[resName]; !ok {
			rc.Map[resName] = NewByRes()
		}
		rc.Map[resName].Update(e)
	}

	// Rebuild list from map values.
	rc.List = []*ByRes{}
	for _, v := range rc.Map {
		rc.List = append(rc.List, v)
	}

	// Sort locks the rc as well, so we need to release it here to avoid deadlock.
	rc.Unlock()

	rc.Sort()
}

// Sorting code adapted from https://golang.org/pkg/sort/#example__sortMultiKeys

// LessFunc determines if p1 should come before p2 during a sort.
type LessFunc func(p1, p2 *ByRes) bool

// Sort sorts the argument slice according to the less functions passed to OrderedBy.
func (rc *ResourceCollection) Sort() {
	rc.Lock()
	defer rc.Unlock()
	sort.Sort(rc)
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

// Name sorts resource names by alpha.
func Name(r1, r2 *ByRes) bool {
	return r1.Res.Name < r2.Res.Name
}

// NameReverse sorts resource names by alpha in reverse order.
func NameReverse(r1, r2 *ByRes) bool {
	return r1.Res.Name > r2.Res.Name
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
