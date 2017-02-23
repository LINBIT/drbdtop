/*
 *drbdtop - continously update stats on drbd
 *Copyright © 2017 Hayley Swimelar
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

package resource

import (
	"math"
	"strconv"
	"sync"
	"time"
)

const timeFormat = "2006-01-02T15:04:05.000000-07:00"

var resKeys = []string{"name", "role", "suspended", "write-ordering"}

const (
	resName = iota
	resRole
	resSuspended
	resWriteOrdering
)

var connKeys = []string{"name", "peer-node-id", "conn-name", "connection", "role", "congested"}

const (
	connName = iota
	connPeerNodeID
	connConnName
	connConnection
	connRole
	connCongested
)

var devKeys = []string{"name", "volume", "minor", "disk", "size", "read", "written", "al-writes", "bm-writes", "upper-pending", "lower-pending", "al-suspended", "blocked"}

const (
	devName = iota
	devVolume
	devMinor
	devDisk
	devSize
	devRead
	devWritten
	devALWrites
	devBMWrites
	devUpperPending
	devLowerPending
	devALSuspended
	devBlocked
)

var peerDevKeys = []string{"name", "peer-node-id", "conn-name", "volume", "replication", "peer-disk", "resync-suspended", "received", "sent", "out-of-sync", "pending", "unacked"}

const (
	peerDevName = iota
	peerDevNodeID
	peerDevConnName
	peerDevVolume
	peerDevReplication
	peerDevPeerDisk
	peerDevResyncSuspended
	peerDevReceived
	peerDevSent
	peerDevOutOfSync
	peerDevPending
	peerDevUnacked
)

type uptimer struct {
	StartTime   time.Time
	CurrentTime time.Time
	Uptime      time.Duration
}

func (u *uptimer) updateTimes(t time.Time) {
	u.CurrentTime = t
	// Init timestamp for new resources.
	if u.StartTime.IsZero() {
		u.StartTime = u.CurrentTime
	}

	u.Uptime = u.CurrentTime.Sub(u.StartTime)
}

type minMaxAvgCurrent struct {
	updateCount int
	total       uint64

	Min     uint64
	Max     uint64
	Avg     float64
	Current uint64
}

func newMinMaxAvgCurrent() *minMaxAvgCurrent {
	return &minMaxAvgCurrent{Min: math.MaxUint64}
}

func (m *minMaxAvgCurrent) calculate(s string) {
	i, _ := strconv.ParseUint(s, 10, 64)

	m.updateCount++
	m.total += i

	if i < m.Min {
		m.Min = i
	}
	if i > m.Max {
		m.Max = i
	}

	m.Avg = float64(m.total) / float64(m.updateCount)

	m.Current = i
}

type rate struct {
	initial uint64
	current uint64
	new     bool

	Previous  *previousFloat64
	PerSecond float64
	Total     uint64
}

func (r *rate) calculate(t time.Duration, s string) {
	i, _ := strconv.ParseUint(s, 10, 64)

	// We have not been calculated before, set initial value
	// to account for the fact that we are seeing a partial dataset.
	if r.new {
		r.initial = i
		r.new = false
	}
	// A connection flapped and we're seeing a new dataset, reset initial to 0.
	if i < r.current {
		r.initial = 0
	}

	r.current = i

	r.Total += (i - r.initial)

	rate := float64(r.Total) / t.Seconds()
	if math.IsNaN(rate) {
		rate = 0
	}
	r.Previous.Push(rate)
	r.PerSecond = r.Previous.Values[len(r.Previous.Values)-1]
}

// Preserve maxLen number of float64s, old values drop off from the front
// when the maxlen as been hit.
type previousFloat64 struct {
	maxLen int
	Values []float64
}

func (p *previousFloat64) Push(i float64) {
	if len(p.Values) == p.maxLen {
		p.Values = append(p.Values[1:], i)
	} else {
		p.Values = append(p.Values, i)
	}
}

type Event struct {
	timeStamp time.Time
	EventType string
	target    string
	fields    map[string]string
}

// NewEvent parses the normal string output of drbdsetup events2 and returns an Event.
func NewEvent(e string) (Event, error) {

	return Event{}, nil
}

type Resource struct {
	sync.RWMutex
	uptimer
	Name          string
	Role          string
	Suspended     string
	WriteOrdering string

	// Calulated Values
	updateCount int
}

func (r *Resource) Update(e Event) bool {
	r.Lock()
	defer r.Unlock()

	r.Name = e.fields[resKeys[resName]]
	r.Role = e.fields[resKeys[resRole]]
	r.Suspended = e.fields[resKeys[resSuspended]]
	r.WriteOrdering = e.fields[resKeys[resWriteOrdering]]
	r.updateTimes(e.timeStamp)
	r.updateCount++

	return true
}

type Connection struct {
	sync.RWMutex
	uptimer
	Resource         string
	PeerNodeID       string
	ConnectionName   string
	ConnectionStatus string
	Role             string
	Congested        string
	// Calculated Values

	updateCount int
}

func (c *Connection) Update(e Event) bool {
	c.Lock()
	defer c.Unlock()

	c.Resource = e.fields[connKeys[connName]]
	c.PeerNodeID = e.fields[connKeys[connPeerNodeID]]
	c.ConnectionName = e.fields[connKeys[connConnName]]
	c.ConnectionStatus = e.fields[connKeys[connConnection]]
	c.Role = e.fields[connKeys[connRole]]
	c.Congested = e.fields[connKeys[connCongested]]
	c.updateTimes(e.timeStamp)
	c.updateCount++
	return true
}

type Device struct {
	sync.RWMutex
	Resource string
	Volumes  map[string]*DevVolume
}

func (d *Device) Update(e Event) bool {
	d.Lock()
	defer d.Unlock()

	d.Resource = e.fields[devKeys[devName]]

	// If this volume doesn't exist, create a fresh one.
	_, ok := d.Volumes[e.fields[devKeys[devVolume]]]
	if !ok {
		d.Volumes[e.fields[devKeys[devVolume]]] = newDevVolume(200)
	}

	vol := d.Volumes[e.fields[devKeys[devVolume]]]

	vol.uptimer.updateTimes(e.timeStamp)
	vol.Minor = e.fields[devKeys[devMinor]]
	vol.DiskState = e.fields[devKeys[devDisk]]
	vol.ActivityLogSuspended = e.fields[devKeys[devALSuspended]]
	vol.Blocked = e.fields[devKeys[devBlocked]]

	// Only update size if we can parse the field correctly.
	if size, err := strconv.ParseUint(e.fields[devKeys[devSize]], 10, 64); err == nil {
		vol.Size = size
	}

	vol.ReadKiB.calculate(vol.uptimer.Uptime, e.fields[devKeys[devRead]])
	vol.WrittenKiB.calculate(vol.uptimer.Uptime, e.fields[devKeys[devWritten]])
	vol.ActivityLogUpdates.calculate(vol.uptimer.Uptime, e.fields[devKeys[devALWrites]])
	vol.BitMapUpdates.calculate(vol.uptimer.Uptime, e.fields[devKeys[devBMWrites]])

	vol.UpperPending.calculate(e.fields[devKeys[devUpperPending]])
	vol.LowerPending.calculate(e.fields[devKeys[devLowerPending]])

	return true
}

type DevVolume struct {
	uptimer
	Minor                string
	DiskState            string
	Size                 uint64
	ActivityLogSuspended string
	Blocked              string

	// Calculated Values
	ReadKiB            *rate
	WrittenKiB         *rate
	ActivityLogUpdates *rate
	BitMapUpdates      *rate

	UpperPending *minMaxAvgCurrent
	LowerPending *minMaxAvgCurrent
	Pending      *minMaxAvgCurrent
}

func newDevVolume(maxLen int) *DevVolume {
	return &DevVolume{
		ReadKiB:            &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
		WrittenKiB:         &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
		ActivityLogUpdates: &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
		BitMapUpdates:      &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},

		UpperPending: newMinMaxAvgCurrent(),
		LowerPending: newMinMaxAvgCurrent(),
	}
}

type PeerDevice struct {
	sync.RWMutex
	uptimer
	Resource       string
	PeerNodeID     string
	ConnectionName string
	Volumes        map[string]*PeerDevVol
}

func (p *PeerDevice) Update(e Event) bool {
	p.Lock()
	defer p.Unlock()

	p.Resource = e.fields[peerDevKeys[peerDevName]]
	p.PeerNodeID = e.fields[peerDevKeys[peerDevNodeID]]
	p.ConnectionName = e.fields[peerDevKeys[peerDevConnName]]
	p.updateTimes(e.timeStamp)

	// If this volume doesn't exist, create a fresh one.
	_, ok := p.Volumes[e.fields[peerDevKeys[peerDevVolume]]]
	if !ok {
		p.Volumes[e.fields[peerDevKeys[peerDevVolume]]] = newPeerDevVol(200)
	}

	vol := p.Volumes[e.fields[peerDevKeys[peerDevVolume]]]

	vol.ReplicationStatus = e.fields[peerDevKeys[peerDevReplication]]
	vol.DiskState = e.fields[peerDevKeys[peerDevPeerDisk]]
	vol.ResyncSuspended = e.fields[peerDevKeys[peerDevResyncSuspended]]

	vol.OutOfSyncKiB.calculate(e.fields[peerDevKeys[peerDevOutOfSync]])
	vol.PendingWrites.calculate(e.fields[peerDevKeys[peerDevPending]])
	vol.UnackedWrites.calculate(e.fields[peerDevKeys[peerDevUnacked]])

	vol.ReceivedKiB.calculate(p.Uptime, e.fields[peerDevKeys[peerDevReceived]])
	vol.SentKiB.calculate(p.Uptime, e.fields[peerDevKeys[peerDevSent]])

	return true
}

type PeerDevVol struct {
	uptimer
	ReplicationStatus string
	DiskState         string
	ResyncSuspended   string

	// Calulated Values
	OutOfSyncKiB  *minMaxAvgCurrent
	PendingWrites *minMaxAvgCurrent
	UnackedWrites *minMaxAvgCurrent

	ReceivedKiB *rate
	SentKiB     *rate
}

func newPeerDevVol(maxLen int) *PeerDevVol {
	return &PeerDevVol{
		OutOfSyncKiB:  newMinMaxAvgCurrent(),
		PendingWrites: newMinMaxAvgCurrent(),
		UnackedWrites: newMinMaxAvgCurrent(),

		ReceivedKiB: &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
		SentKiB:     &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
	}
}
