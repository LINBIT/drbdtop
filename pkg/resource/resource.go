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

type maxAvgCurrent struct {
	updateCount int
	total       uint64

	Max     uint64
	Avg     float64
	Current uint64
}

func (m *maxAvgCurrent) calculate(s string) {
	i, _ := strconv.ParseUint(s, 10, 64)

	m.updateCount++
	m.total += i

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
	target    string
	fields    map[string]string
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
	peerNodeID       string
	connectionName   string
	connectionStatus string
	role             string
	congested        string
	// Calculated Values

	updateCount int
}

func (c *Connection) Update(e Event) bool {
	c.Lock()
	defer c.Unlock()

	c.peerNodeID = e.fields[connKeys[connPeerNodeID]]
	c.connectionName = e.fields[connKeys[connConnName]]
	c.connectionStatus = e.fields[connKeys[connConnection]]
	c.role = e.fields[connKeys[connRole]]
	c.congested = e.fields[connKeys[connCongested]]
	c.updateTimes(e.timeStamp)
	c.updateCount++
	return true
}

type Device struct {
	sync.RWMutex
	resource string
	volumes  map[string]*DevVolume
}

func (d *Device) Update(e Event) bool {
	d.Lock()
	defer d.Unlock()

	d.resource = e.fields[devKeys[devName]]

	// If this volume doesn't exist, create a fresh one.
	_, ok := d.volumes[e.fields[devKeys[devVolume]]]
	if !ok {
		d.volumes[e.fields[devKeys[devVolume]]] = newDevVolume(200)
	}

	vol := d.volumes[e.fields[devKeys[devVolume]]]

	vol.uptimer.updateTimes(e.timeStamp)
	vol.minor = e.fields[devKeys[devMinor]]
	vol.diskState = e.fields[devKeys[devDisk]]
	vol.activityLogSuspended = e.fields[devKeys[devALSuspended]]
	vol.blocked = e.fields[devKeys[devBlocked]]

	// Only update size if we can parse the field correctly.
	if size, err := strconv.ParseUint(e.fields[devKeys[devSize]], 10, 64); err == nil {
		vol.size = size
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
	minor                string
	diskState            string
	size                 uint64
	activityLogSuspended string
	blocked              string

	// Calculated Values
	ReadKiB            *rate
	WrittenKiB         *rate
	ActivityLogUpdates *rate
	BitMapUpdates      *rate

	UpperPending *maxAvgCurrent
	LowerPending *maxAvgCurrent
	Pending      *maxAvgCurrent
}

func newDevVolume(maxLen int) *DevVolume {
	return &DevVolume{
		ReadKiB:            &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
		WrittenKiB:         &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
		ActivityLogUpdates: &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
		BitMapUpdates:      &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},

		UpperPending: &maxAvgCurrent{},
		LowerPending: &maxAvgCurrent{},
	}
}

type PeerDevice struct {
	sync.RWMutex
	uptimer
	resource       string
	peerNodeID     string
	connectionName string
	volumes        map[string]*PeerDevVol
}

func (p *PeerDevice) Update(e Event) bool {
	p.Lock()
	defer p.Unlock()

	p.resource = e.fields[peerDevKeys[peerDevName]]
	p.peerNodeID = e.fields[peerDevKeys[peerDevNodeID]]
	p.connectionName = e.fields[peerDevKeys[peerDevConnName]]
	p.updateTimes(e.timeStamp)

	// If this volume doesn't exist, create a fresh one.
	_, ok := p.volumes[e.fields[peerDevKeys[peerDevVolume]]]
	if !ok {
		p.volumes[e.fields[peerDevKeys[peerDevVolume]]] = newPeerDevVol(200)
	}

	vol := p.volumes[e.fields[peerDevKeys[peerDevVolume]]]

	vol.replicationStatus = e.fields[peerDevKeys[peerDevReplication]]
	vol.diskState = e.fields[peerDevKeys[peerDevPeerDisk]]
	vol.resyncSuspended = e.fields[peerDevKeys[peerDevResyncSuspended]]

	vol.OutOfSyncKiB.calculate(e.fields[peerDevKeys[peerDevOutOfSync]])
	vol.PendingWrites.calculate(e.fields[peerDevKeys[peerDevPending]])
	vol.UnackedWrites.calculate(e.fields[peerDevKeys[peerDevUnacked]])

	vol.ReceivedKiB.calculate(p.Uptime, e.fields[peerDevKeys[peerDevReceived]])
	vol.SentKiB.calculate(p.Uptime, e.fields[peerDevKeys[peerDevSent]])

	return true
}

type PeerDevVol struct {
	uptimer
	replicationStatus string
	diskState         string
	resyncSuspended   string

	// Calulated Values
	OutOfSyncKiB  *maxAvgCurrent
	PendingWrites *maxAvgCurrent
	UnackedWrites *maxAvgCurrent

	ReceivedKiB *rate
	SentKiB     *rate
}

func newPeerDevVol(maxLen int) *PeerDevVol {
	return &PeerDevVol{
		OutOfSyncKiB:  &maxAvgCurrent{},
		PendingWrites: &maxAvgCurrent{},
		UnackedWrites: &maxAvgCurrent{},

		ReceivedKiB: &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
		SentKiB:     &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
	}
}
