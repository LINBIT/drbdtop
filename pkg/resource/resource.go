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

// Set the default value of min to MaxUint64, otherwise it will always be zero.
func newMinMaxAvgCurrent() *minMaxAvgCurrent {
	return &minMaxAvgCurrent{Min: math.MaxUint64}
}

func (m *minMaxAvgCurrent) calculate(i uint64) {
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
	last    uint64
	new     bool

	Previous  previousFloat64
	PerSecond float64
	Total     uint64
}

func (r *rate) calculate(t time.Duration, i uint64) {
	// We have not been calculated before, set initial value
	// to account for the fact that we are seeing a partial dataset.
	if r.new {
		r.initial = i
		r.new = false
	}
	// A connection flapped and we're seeing a new dataset, reset initial to 0.
	if i < r.last {
		r.initial = 0
	}

	r.last = i

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

type Status struct {
	sync.RWMutex
	uptimer
	Name          string
	Role          string
	Suspended     string
	WriteOrdering string
	Connections   map[string]*Connection
	Devices       map[string]*Device
	PeerDevices   map[string]*PeerDevice

	// Calulated Values
	updateCount int

	numPeerDevs    int
	numDevs        int
	numConnections int
}

func (s *Status) Update(e Event) bool {
	s.Lock()
	defer s.Unlock()

	switch e.target {
	case "resource":
		return s.updateResource(e)
	case "connection":
		connName := e.fields[connKeys[connConnName]]
		if s.Connections[connName] == nil {
			s.Connections[connName] = &Connection{}
		}
		return s.Connections[connName].update(e)
	}
	return true
}

func (s *Status) updateResource(e Event) bool {
	s.Name = e.fields[resKeys[resName]]
	s.Role = e.fields[resKeys[resRole]]
	s.Suspended = e.fields[resKeys[resSuspended]]
	s.WriteOrdering = e.fields[resKeys[resWriteOrdering]]
	s.updateTimes(e.timeStamp)
	s.updateCount++

	return true
}

type Connection struct {
	uptimer
	peerNodeID       string
	connectionName   string
	connectionStatus string
	role             string
	congested        string
	// Calculated Values

	updateCount int
}

func (c *Connection) update(e Event) bool {
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
	volumes map[string]DevVolume
}

type DevVolume struct {
	uptimer
	minor                string
	diskState            string
	size                 uint64
	readKiB              uint64
	writtenKiB           uint64
	activityLogUpdates   uint64
	bitMapUpdates        uint64
	upperPending         uint64
	lowerPending         uint64
	activityLogSuspended string
	blocked              string

	// Calculated Values

	initialReadKiB   uint64
	totalReadKiB     uint64
	readKiBPerSecond float64

	initialActivityLogUpdates uint64
	totalActivityLogUpdates   uint64
	alUpdatesPerSecond        float64

	initialBitMapUpdates   uint64
	totalBitMapUpdates     uint64
	bitMapUpdatesPerSecond float64

	UpperPending minMaxAvgCurrent
	Pending      minMaxAvgCurrent
}

type PeerDevice struct {
	peerNodeID     string
	connectionName string
	volumes        map[string]PeerDevVol
}

type PeerDevVol struct {
	uptimer
	replicationStatus string
	diskState         string
	resyncSuspended   string
	receivedKiB       uint64
	sentKiB           uint64
	outOfSyncKiB      uint64
	pendingWrites     uint64
	unackedWrites     uint64

	// Calulated Values
	OutOfSyncKiB  minMaxAvgCurrent
	PendingWrites minMaxAvgCurrent
	UnackedWrites minMaxAvgCurrent

	initialReceivedKiB uint64
	totalReceivedKiB   uint64
	receivedKiBSecond  float64

	initialSentKiB uint64
	totalSentKiB   uint64
	sentKiBSecond  float64
}
