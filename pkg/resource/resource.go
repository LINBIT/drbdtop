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

type resKeys struct {
	Name          string
	Role          string
	Suspended     string
	WriteOrdering string
}

// ResKeys is a data container for the field keys of resource Events.
var ResKeys = resKeys{"name", "role", "suspended", "write-ordering"}

type connKeys struct {
	Name       string
	PeerNodeID string
	ConnName   string
	Connection string
	Role       string
	Congested  string
}

// ConnKeys is a data container for the field keys of connection Events.
var ConnKeys = connKeys{"name", "peer-node-id", "conn-name", "connection", "role", "congested"}

type devKeys struct {
	Name         string
	Volume       string
	Minor        string
	Disk         string
	Client       string
	Size         string
	Read         string
	Written      string
	ALWrites     string
	BMWrites     string
	UpperPending string
	LowerPending string
	ALSuspended  string
	Blocked      string
}

// DevKeys is a data container for the field keys of device Events.
var DevKeys = devKeys{"name", "volume", "minor", "disk", "client", "size", "read", "written", "al-writes", "bm-writes", "upper-pending", "lower-pending", "al-suspended", "blocked"}

type peerDevKeys struct {
	Name            string
	PeerNodeID      string
	ConnName        string
	Volume          string
	Replication     string
	PeerDisk        string
	ResyncSuspended string
	Received        string
	Sent            string
	OutOfSync       string
	Pending         string
	Unacked         string
}

// PeerDevKeys is a data container for the field keys of device Events.
var PeerDevKeys = peerDevKeys{"name", "peer-node-id", "conn-name", "volume", "replication", "peer-disk", "resync-suspended", "received", "sent", "out-of-sync", "pending", "unacked"}

var connDangerScores = map[string]uint64{
	"Connected":  0,
	"SyncSource": 1,
	"SyncTarget": 1,
	"StandAlone": 30,

	"default": 1,
}

var diskDangerScores = map[string]uint64{
	"UpToDate":   0,
	"Consistent": 1,
	"Diskless":   16,
	"Outdated":   1,
	"DUnknown":   2,

	"default": 1,
}

var roleDangerScores = map[string]uint64{
	"Primary":   0,
	"Secondary": 0,
	"Unknown":   1,

	"default": 1,
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
	last    uint64
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
	// A connection flapped and we're seeing a new dataset,
	// reset initial to 0 and adjust total to account for old data.
	if i < r.last {
		r.initial = 0
		r.Total = (r.last - i)
	} else {
		r.Total = (i - r.initial)
	}

	r.last = i

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

// Event is a serialized update from the DRBD kernel module."
type Event struct {
	TimeStamp time.Time
	// EventType is the kind of update being relayed from DRBD: exists, change, call, etc.
	EventType string
	// Target is the kind of data contained within the Event: peer-device, connection, etc.
	Target string
	// Key/Value pairs separated by a ":"
	Fields map[string]string
}

// NewEvent parses the normal string output of drbdsetup events2 and returns an Event.
func NewEvent(e string) (Event, error) {
	e = strings.TrimSpace(e)

	// Dynamically assign event fields for all events, reguardless of event target.
	fields := make(map[string]string)

	data := strings.Split(e, " ")
	if len(data) < 3 {
		return Event{Fields: fields}, fmt.Errorf("Couldn't create an Event from %v", data)
	}

	timeStamp, err := time.Parse(timeFormat, data[0])
	if err != nil {
		return Event{Fields: fields}, err
	}

	for _, d := range data[3:] {
		// Splitting strings is expensive and this loop runs a lot, so we use the
		// index of ":" to break up the key value pairs.
		i := strings.Index(d, ":")
		if i < 0 {
			return Event{Fields: fields}, fmt.Errorf("Couldn't parse key/value pair from %q", d)
		}
		fields[d[:i]] = d[i+1:]
	}

	return Event{
		TimeStamp: timeStamp,
		EventType: data[1],
		Target:    data[2],
		Fields:    fields,
	}, nil
}

// NewEOF returns a special Event signaling that no further input should be expected.
func NewEOF() Event {
	return Event{Target: EOF}
}

// Updater modify their data based on incoming Events.
type Updater interface {
	Update(Event)
}

// Resource represents basic resource info.
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

// Update the values of Resource with a new Event.
func (r *Resource) Update(e Event) {
	r.Lock()
	defer r.Unlock()

	r.Name = e.Fields[ResKeys.Name]
	r.Role = e.Fields[ResKeys.Role]
	r.Suspended = e.Fields[ResKeys.Suspended]
	r.WriteOrdering = e.Fields[ResKeys.WriteOrdering]
	r.updateTimes(e.TimeStamp)
	r.updateCount++
}

// Connection represents the connection from the local resource to a remote resource.
type Connection struct {
	sync.RWMutex
	uptimer
	Resource         string
	PeerNodeID       string
	ConnectionName   string
	ConnectionStatus string
	// Long form explination of ConnectionStatus.
	ConnectionHint string
	Role           string
	Congested      string

	// Calculated Values
	Danger      uint64
	updateCount int
}

// Update the Connection with a new Event.
func (c *Connection) Update(e Event) {
	c.Lock()
	defer c.Unlock()

	c.Resource = e.Fields[ConnKeys.Name]
	c.PeerNodeID = e.Fields[ConnKeys.PeerNodeID]
	c.ConnectionName = e.Fields[ConnKeys.ConnName]
	c.ConnectionStatus = e.Fields[ConnKeys.Connection]
	c.Role = e.Fields[ConnKeys.Role]
	c.Congested = e.Fields[ConnKeys.Congested]
	c.updateTimes(e.TimeStamp)
	c.Danger = c.getDanger()
	c.connStatusExplination()
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
		d++
	}

	return d
}

func (c *Connection) connStatusExplination() {
	switch c.ConnectionStatus {
	case "StandAlone":
		c.ConnectionHint = fmt.Sprintf("dropped connection or disconnected manually. try running drbdadm connect %s", c.ConnectionName)
	case "Disconnecting":
		c.ConnectionHint = fmt.Sprintf("disconnecting from %s", c.ConnectionName)
	case "Unconnected":
		c.ConnectionHint = fmt.Sprintf("not yet connected to %s", c.ConnectionName)
	case "Timeout":
		c.ConnectionHint = fmt.Sprintf("connection to %s dropped after timeout", c.ConnectionName)
	case "BrokenPipe":
		fallthrough
	case "NetworkFailure":
		fallthrough
	case "ProtocolError":
		c.ConnectionHint = fmt.Sprintf("lost connection to %s", c.ConnectionName)
	case "TearDown":
		c.ConnectionHint = fmt.Sprintf("%s is closing the connection", c.ConnectionName)
	case "Connecting":
		c.ConnectionHint = fmt.Sprintf("establishing connection with %s", c.ConnectionName)
	case "Connected":
		c.ConnectionHint = fmt.Sprintf("connected to %s", c.ConnectionName)
	default:
		c.ConnectionHint = "unknown connection state!"

	}
}

// Device represents a local DRBD virtual block device.
type Device struct {
	sync.RWMutex
	Resource string
	Volumes  map[string]*DevVolume

	//Calculated Values.
	Danger uint64
}

// NewDevice returns a blank device with maps initialized.
func NewDevice() *Device {
	return &Device{
		Volumes: make(map[string]*DevVolume),
	}
}

// Update the devices data with a new Event.
func (d *Device) Update(e Event) {
	d.Lock()
	defer d.Unlock()

	d.Resource = e.Fields[DevKeys.Name]

	// If this volume doesn't exist, create a fresh one.
	_, ok := d.Volumes[e.Fields[DevKeys.Volume]]
	if !ok {
		d.Volumes[e.Fields[DevKeys.Volume]] = NewDevVolume(200)
	}

	vol := d.Volumes[e.Fields[DevKeys.Volume]]

	vol.uptimer.updateTimes(e.TimeStamp)
	vol.Minor = e.Fields[DevKeys.Minor]
	vol.DiskState = e.Fields[DevKeys.Disk]
	vol.Client = e.Fields[DevKeys.Client]
	vol.diskStateExplination()
	vol.ActivityLogSuspended = e.Fields[DevKeys.ALSuspended]
	vol.Blocked = e.Fields[DevKeys.Blocked]

	// Only update size if we can parse the field correctly.
	if size, err := strconv.ParseUint(e.Fields[DevKeys.Size], 10, 64); err == nil {
		vol.Size = size
	}

	vol.ReadKiB.calculate(vol.uptimer.Uptime, e.Fields[DevKeys.Read])
	vol.WrittenKiB.calculate(vol.uptimer.Uptime, e.Fields[DevKeys.Written])
	vol.ActivityLogUpdates.calculate(vol.uptimer.Uptime, e.Fields[DevKeys.ALWrites])
	vol.BitMapUpdates.calculate(vol.uptimer.Uptime, e.Fields[DevKeys.BMWrites])
	vol.UpperPending.calculate(e.Fields[DevKeys.UpperPending])
	vol.LowerPending.calculate(e.Fields[DevKeys.LowerPending])

	d.Danger = d.getDanger()
}

func (d *Device) getDanger() uint64 {
	var score uint64

	for _, v := range d.Volumes {
		i, ok := diskDangerScores[v.DiskState]
		if !ok {
			score += diskDangerScores["default"]
			// If we're diskless on purpose, then everything is normal.
		} else if !(v.DiskState == "Diskless" && v.Client == "yes") {
			score += i
		}
	}

	return score
}

// DevVolume represents a single volume of a local DRBD virtual block device.
type DevVolume struct {
	uptimer
	Minor     string
	DiskState string
	// Long from explination of DiskState.
	DiskHint             string
	Client               string
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

// NewDevVolume returns a DevVolume with internal structures initialized.
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

func (v *DevVolume) diskStateExplination() {
	switch v.DiskState {
	case "Diskless":
		v.DiskHint = "detached from local backing disk"
	case "Attaching":
		v.DiskHint = "reading metadata"
	case "Failed":
		v.DiskHint = "I/O failure reported by local backing disk"
	case "Negotiating":
		v.DiskHint = "communicating with peer..."
	case "Inconsistent":
		v.DiskHint = "local data is not accessible or usable until resync is complete"
	case "Outdated":
		v.DiskHint = "data is usable, but a peer has newer data"
	case "Consistent":
		v.DiskHint = "data is usable, but we have no network connection"
	case "UpToDate":
		v.DiskHint = "normal disk state"
	default:
		v.DiskHint = "unknown disk state!"
	}
}

// PeerDevice represents the virtual DRBD block device of a remote resource.
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

// NewPweerDevice returns a PeerDevice with internal maps initialized.
func NewPeerDevice() *PeerDevice {
	return &PeerDevice{
		Volumes: make(map[string]*PeerDevVol),
	}
}

// Update the PeerDevice with a new Event.
func (p *PeerDevice) Update(e Event) {
	p.Lock()
	defer p.Unlock()

	p.Resource = e.Fields[PeerDevKeys.Name]
	p.PeerNodeID = e.Fields[PeerDevKeys.PeerNodeID]
	p.ConnectionName = e.Fields[PeerDevKeys.ConnName]
	p.updateTimes(e.TimeStamp)

	// If this volume doesn't exist, create a fresh one.
	_, ok := p.Volumes[e.Fields[PeerDevKeys.Volume]]
	if !ok {
		p.Volumes[e.Fields[PeerDevKeys.Volume]] = NewPeerDevVol(200)
	}

	vol := p.Volumes[e.Fields[PeerDevKeys.Volume]]

	vol.updateTimes(e.TimeStamp)

	vol.ReplicationStatus = e.Fields[PeerDevKeys.Replication]
	vol.ReplicationHint = p.replicationExplination(vol)
	vol.DiskState = e.Fields[PeerDevKeys.PeerDisk]
	vol.ResyncSuspended = e.Fields[PeerDevKeys.ResyncSuspended]

	vol.OutOfSyncKiB.calculate(e.Fields[PeerDevKeys.OutOfSync])
	vol.PendingWrites.calculate(e.Fields[PeerDevKeys.Pending])
	vol.UnackedWrites.calculate(e.Fields[PeerDevKeys.Unacked])

	vol.ReceivedKiB.calculate(vol.Uptime, e.Fields[PeerDevKeys.Received])
	vol.SentKiB.calculate(vol.Uptime, e.Fields[PeerDevKeys.Sent])

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

		// Resources can be up to 1 PiB, so this will be at most 12.
		if v.OutOfSyncKiB.Current != 0 {
			score += uint64(math.Log(float64(v.OutOfSyncKiB.Current)))
		}
	}

	return score
}

func (p *PeerDevice) replicationExplination(v *PeerDevVol) string {
	switch v.ReplicationStatus {
	case "Off":
		return fmt.Sprintf("not replicating to %s", p.Resource)
	case "Established":
		return fmt.Sprintf("healthy connection to %s — mirroring active", p.ConnectionName)
	case "StartingSyncS":
		return fmt.Sprintf("full resync of local data to %s due to admin", p.ConnectionName)
	case "StartingSyncT":
		return fmt.Sprintf("full resync from %s due to admin", p.ConnectionName)
	case "WFBitMapS":
		return fmt.Sprintf("resync to %s starting", p.ConnectionName)
	case "WFBitMapT":
		return fmt.Sprintf("resync from %s starting", p.ConnectionName)
	case "WFSyncUUID":
		return fmt.Sprintf("resync from %s starting", p.ConnectionName)
	case "SyncSource":
		return fmt.Sprintf("synchronizing %s with local data", p.ConnectionName)
	case "SyncTarget":
		return fmt.Sprintf("local volume is being synchronized with data from %s", p.ConnectionName)
	case "VerifyS":
		return fmt.Sprintf("verifying %s with local data", p.ConnectionName)
	case "VerifyT":
		return fmt.Sprintf("local volume is being verified with data from %s", p.ConnectionName)
	case "PausedSyncS":
		return fmt.Sprintf("synchronizing %s with local data is paused", p.ConnectionName)
	case "PausedSyncT":
		return fmt.Sprintf("synchronization with data from %s is paused", p.ConnectionName)
	case "Ahead":
		return fmt.Sprintf("temporarily disconnected from %s to preserve local I/O performance", p.ConnectionName)
	case "Behind":
		return fmt.Sprintf("temporarily disconnected from %s to preserve %[1]s's I/O performance", p.ConnectionName)
	default:
		return "unknown replication status!"
	}
}

// PeerDevVol represents a single volume of a remote resources virtual block device.
type PeerDevVol struct {
	uptimer
	ReplicationStatus string
	// Long form explination of Replication Status.
	ReplicationHint string
	DiskState       string
	ResyncSuspended string

	// Calulated Values
	OutOfSyncKiB  *minMaxAvgCurrent
	PendingWrites *minMaxAvgCurrent
	UnackedWrites *minMaxAvgCurrent

	ReceivedKiB *rate
	SentKiB     *rate
}

// NewPeerDevVol returns a PeerDevVol with internal structs initialized.
func NewPeerDevVol(maxLen int) *PeerDevVol {
	return &PeerDevVol{
		OutOfSyncKiB:  newMinMaxAvgCurrent(),
		PendingWrites: newMinMaxAvgCurrent(),
		UnackedWrites: newMinMaxAvgCurrent(),

		ReceivedKiB: &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
		SentKiB:     &rate{Previous: &previousFloat64{maxLen: maxLen}, new: true},
	}
}
