package update

import (
	"sort"

	"drbdtop.io/drbdtop/pkg/resource"
)

// ResourceCollection is a collection of stats collected organized under their respective resource names.
type ResourceCollection struct {
	Map    map[string]*ByRes
	Sorted *MultiSorter
}

// Update a collection of ByRes from an Event.
func (r *ResourceCollection) Update(e resource.Event) {
}

// ByRes organizes events related to a particular resource.
type ByRes struct {
	Res         *resource.Resource
	Connections map[string]*resource.Connection
	Device      *resource.Device
	PeerDevices map[string]*resource.PeerDevice
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
	for {
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
		}
	}
}

// https://golang.org/pkg/sort/#example__sortMultiKeys
type lessFunc func(p1, p2 *ByRes) bool

// MultiSorter implements the Sort interface, sorting the changes within.
type MultiSorter struct {
	Resources []*ByRes
	less      []lessFunc
}

// Sort sorts the argument slice according to the less functions passed to OrderedBy.
func (ms *MultiSorter) Sort(changes []*ByRes) {
	ms.Resources = changes
	sort.Sort(ms)
}

// OrderedBy returns a Sorter that sorts using the less functions, in order.
// Call its Sort method to sort the data.
func OrderedBy(less ...lessFunc) *MultiSorter {
	return &MultiSorter{
		less: less,
	}
}

// Len is part of sort.Interface.
func (ms *MultiSorter) Len() int {
	return len(ms.Resources)
}

// Swap is part of sort.Interface.
func (ms *MultiSorter) Swap(i, j int) {
	ms.Resources[i], ms.Resources[j] = ms.Resources[j], ms.Resources[i]
}

// Less is part of sort.Interface. It is implemented by looping along the
// less functions until it finds a comparison that is either Less or
// !Less. Note that it can call the less functions twice per call. We
// could change the functions to return -1, 0, 1 and reduce the
// number of calls for greater efficiency: an exercise for the reader.
func (ms *MultiSorter) Less(i, j int) bool {
	p, q := ms.Resources[i], ms.Resources[j]
	// Try all but the last comparison.
	var k int
	for k = 0; k < len(ms.less)-1; k++ {
		less := ms.less[k]
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
	return ms.less[k](p, q)
}

// Name sorts resource names by alpha.
func Name(r1, r2 *ByRes) bool {
	return r1.Res.Name < r2.Res.Name
}
