package update

import (
	"drbdtop.io/drbdtop/pkg/resource"
)

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
