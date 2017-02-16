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
	Uptime      time.Time
}

func (u *uptimer) updateTimes(t time.Time) {
	u.CurrentTime = t
	// Init timestamp for new resources.
	if u.StartTime.IsZero() {
		u.StartTime = u.CurrentTime
	}

	diff := u.CurrentTime.Sub(u.StartTime)
	u.Uptime = u.StartTime.Add(diff)
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
	updateCount int

	initialReadKiB   uint64
	totalReadKiB     uint64
	readKiBPerSecond float64

	initialActivityLogUpdates uint64
	totalActivityLogUpdates   uint64
	alUpdatesPerSecond        float64

	initialBitMapUpdates   uint64
	totalBitMapUpdates     uint64
	bitMapUpdatesPerSecond float64

	maxUpperPending   uint64
	minUpperPending   uint64
	avgUpperPending   float64
	totalUpperPending uint64

	maxLowerPending   uint64
	minLowerPending   uint64
	avgLowerPending   float64
	totalLowerPending uint64
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
	updateCount int

	maxOutOfSyncKiB   uint64
	minOutOfSyncKiB   uint64
	avgOutOfSyncKiB   float64
	totalOutOfSyncKiB uint64

	maxPendingWrites   uint64
	minPendingWrites   uint64
	avgPendingWrites   float64
	totalPendingWrites uint64

	maxUnackedWrites   uint64
	minUnackedWrites   uint64
	avgUnackedWrites   float64
	totalUnackedWrites uint64

	initialReceivedKiB uint64
	totalReceivedKiB   uint64
	receivedKiBSecond  float64

	initialSentKiB uint64
	totalSentKiB   uint64
	sentKiBSecond  float64
}
