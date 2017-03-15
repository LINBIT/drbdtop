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
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const timeFormat = "2006-01-02T15:04:05.000000-07:00"

// EOF is the End Of File sentinel to signal no further Events are expected.
const EOF = "EOF"

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

var connDangerScores = map[string]uint64{
	"Connected":  0,
	"SyncSource": 500,
	"SyncTarget": 600,
	"StandAlone": 2500000,

	"default": 1000,
}

var diskDangerScores = map[string]uint64{
	"UpToDate":   0,
	"Consistent": 100,
	"Diskless":   250,
	"Outdated":   400,
	"DUnknown":   2000,

	"default": 1000,
}

var roleDangerScores = map[string]uint64{
	"Primary":   0,
	"Secondary": 0,
	"Unknown":   1000,

	"default": 1000,
}

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
	if len(p.Values) >= p.maxLen {
		p.Values = append(p.Values[1:], i)
	} else {
		p.Values = append(p.Values, i)
	}
}

type Event struct {
	TimeStamp time.Time
	EventType string
	Target    string
	Fields    map[string]string
}

// NewEvent parses the normal string output of drbdsetup events2 and returns an Event.
func NewEvent(e string) (Event, error) {
	e = strings.TrimSpace(e)

	if e == EOF {
		return newEOF()
	}

	data := strings.Split(e, " ")
	if len(data) < 3 {
		return Event{Fields: make(map[string]string)}, fmt.Errorf("Couldn't create an Event from %v", data)
	}

	// Dynamically assign event fields for all events, reguardless of event target.
	fields := make(map[string]string)
	for _, d := range data[3:] {
		f := strings.Split(d, ":")
		if len(f) != 2 {
			return Event{Fields: make(map[string]string)}, fmt.Errorf("Couldn't parse key/value pair from %q", d)
		}
		fields[f[0]] = f[1]
	}

	timeStamp, err := time.Parse(timeFormat, data[0])
	if err != nil {
		return Event{Fields: make(map[string]string)}, err
	}

	return Event{
		TimeStamp: timeStamp,
		EventType: data[1],
		Target:    data[2],
		Fields:    fields,
	}, nil
}
func newEOF() (Event, error) {
	return Event{
		Target: EOF,
	}, nil
}

type Updater interface {
	Update(Event)
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

func (r *Resource) Update(e Event) {
	r.Lock()
	defer r.Unlock()

	r.Name = e.Fields[resKeys[resName]]
	r.Role = e.Fields[resKeys[resRole]]
	r.Suspended = e.Fields[resKeys[resSuspended]]
	r.WriteOrdering = e.Fields[resKeys[resWriteOrdering]]
	r.updateTimes(e.TimeStamp)
	r.updateCount++
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

	Danger      uint64
	updateCount int
}

func (c *Connection) Update(e Event) {
	c.Lock()
	defer c.Unlock()

	c.Resource = e.Fields[connKeys[connName]]
	c.PeerNodeID = e.Fields[connKeys[connPeerNodeID]]
	c.ConnectionName = e.Fields[connKeys[connConnName]]
	c.ConnectionStatus = e.Fields[connKeys[connConnection]]
	c.Role = e.Fields[connKeys[connRole]]
	c.Congested = e.Fields[connKeys[connCongested]]
	c.updateTimes(e.TimeStamp)
	c.Danger = c.getDanger()
	c.updateCount++
}

func (c *Connection) getDanger() uint64 {
	var d uint64

	i, ok := connDangerScores[c.ConnectionStatus]
	if !ok {
		d += connDangerScores["default"]
	} else {
		d += i
	}

	i, ok = roleDangerScores[c.Role]
	if !ok {
		d += connDangerScores["default"]
	} else {
		d += i
	}

	if c.Congested != "no" {
		d += 400
	}

	return d
}

type Device struct {
	sync.RWMutex
	Resource string
	Volumes  map[string]*DevVolume

	//Calculated Values.
	Danger uint64
}

func NewDevice() *Device {
	return &Device{
		Volumes: make(map[string]*DevVolume),
	}
}

func (d *Device) Update(e Event) {
	d.Lock()
	defer d.Unlock()

	d.Resource = e.Fields[devKeys[devName]]

	// If this volume doesn't exist, create a fresh one.
	_, ok := d.Volumes[e.Fields[devKeys[devVolume]]]
	if !ok {
		d.Volumes[e.Fields[devKeys[devVolume]]] = NewDevVolume(200)
	}

	vol := d.Volumes[e.Fields[devKeys[devVolume]]]

	vol.uptimer.updateTimes(e.TimeStamp)
	vol.Minor = e.Fields[devKeys[devMinor]]
	vol.DiskState = e.Fields[devKeys[devDisk]]
	vol.ActivityLogSuspended = e.Fields[devKeys[devALSuspended]]
	vol.Blocked = e.Fields[devKeys[devBlocked]]

	// Only update size if we can parse the field correctly.
	if size, err := strconv.ParseUint(e.Fields[devKeys[devSize]], 10, 64); err == nil {
		vol.Size = size
	}

	vol.ReadKiB.calculate(vol.uptimer.Uptime, e.Fields[devKeys[devRead]])
	vol.WrittenKiB.calculate(vol.uptimer.Uptime, e.Fields[devKeys[devWritten]])
	vol.ActivityLogUpdates.calculate(vol.uptimer.Uptime, e.Fields[devKeys[devALWrites]])
	vol.BitMapUpdates.calculate(vol.uptimer.Uptime, e.Fields[devKeys[devBMWrites]])

	vol.UpperPending.calculate(e.Fields[devKeys[devUpperPending]])
	vol.LowerPending.calculate(e.Fields[devKeys[devLowerPending]])

	d.Danger = d.getDanger()
}

func (d *Device) getDanger() uint64 {
	var score uint64

	for _, v := range d.Volumes {
		i, ok := diskDangerScores[v.DiskState]
		if !ok {
			score += diskDangerScores["default"]
		} else {
			score += i
		}
	}

	return score
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
}

func NewDevVolume(maxLen int) *DevVolume {
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

	// Calulated values.
	Danger uint64
}

func NewPeerDevice() *PeerDevice {
	return &PeerDevice{
		Volumes: make(map[string]*PeerDevVol),
	}
}

func (p *PeerDevice) Update(e Event) {
	p.Lock()
	defer p.Unlock()

	p.Resource = e.Fields[peerDevKeys[peerDevName]]
	p.PeerNodeID = e.Fields[peerDevKeys[peerDevNodeID]]
	p.ConnectionName = e.Fields[peerDevKeys[peerDevConnName]]
	p.updateTimes(e.TimeStamp)

	// If this volume doesn't exist, create a fresh one.
	_, ok := p.Volumes[e.Fields[peerDevKeys[peerDevVolume]]]
	if !ok {
		p.Volumes[e.Fields[peerDevKeys[peerDevVolume]]] = NewPeerDevVol(200)
	}

	vol := p.Volumes[e.Fields[peerDevKeys[peerDevVolume]]]

	vol.ReplicationStatus = e.Fields[peerDevKeys[peerDevReplication]]
	vol.DiskState = e.Fields[peerDevKeys[peerDevPeerDisk]]
	vol.ResyncSuspended = e.Fields[peerDevKeys[peerDevResyncSuspended]]

	vol.OutOfSyncKiB.calculate(e.Fields[peerDevKeys[peerDevOutOfSync]])
	vol.PendingWrites.calculate(e.Fields[peerDevKeys[peerDevPending]])
	vol.UnackedWrites.calculate(e.Fields[peerDevKeys[peerDevUnacked]])

	vol.ReceivedKiB.calculate(p.Uptime, e.Fields[peerDevKeys[peerDevReceived]])
	vol.SentKiB.calculate(p.Uptime, e.Fields[peerDevKeys[peerDevSent]])

	p.Danger = p.getDanger()
}

func (p *PeerDevice) getDanger() uint64 {
	var score uint64

	for _, v := range p.Volumes {
		i, ok := diskDangerScores[v.DiskState]
		if !ok {
			score += diskDangerScores["default"]
		} else {
			score += i
		}

		// One point of danger per Mebibyte Out of Sync
		score += v.OutOfSyncKiB.Current / 1024
	}

	return score
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

func NewPeerDevVol(maxLen int) *PeerDevVol {
	return &PeerDevVol{
		OutOfSyncKiB:  newMinMaxAvgCurrent(),
		PendingWrites: newMinMaxAvgCurrent(),
		UnackedWrites: newMinMaxAvgCurrent(),

		ReceivedKiB: &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
		SentKiB:     &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
	}
}
