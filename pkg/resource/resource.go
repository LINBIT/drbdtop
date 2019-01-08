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

package resource

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// EOF is the End Of File sentinel to signal no further Events are expected.
const EOF = "EOF"

// DisplayEvent is the sentinel to signal a display update
const DisplayEvent = "DisplayEvent"

// PruneEvent is the sentinel to signal a prune operation
const PruneEvent = "PruneEvent"

type resKeys struct {
	Name          string
	Role          string
	Suspended     string
	WriteOrdering string
	Unconfigured  string
}

// ResKeys is a data container for the field keys of resource Events.
var ResKeys = resKeys{"name", "role", "suspended", "write-ordering", "unconfigured"}

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
	Quorum       string
}

// DevKeys is a data container for the field keys of device Events.
var DevKeys = devKeys{"name", "volume", "minor", "disk", "client", "size", "read", "written", "al-writes", "bm-writes", "upper-pending", "lower-pending", "al-suspended", "blocked", "quorum"}

type peerDevKeys struct {
	Name            string
	PeerNodeID      string
	ConnName        string
	Volume          string
	Replication     string
	PeerDisk        string
	PeerClient      string
	ResyncSuspended string
	Received        string
	Sent            string
	OutOfSync       string
	Pending         string
	Unacked         string
}

// PeerDevKeys is a data container for the field keys of device Events.
var PeerDevKeys = peerDevKeys{"name", "peer-node-id", "conn-name", "volume", "replication", "peer-disk", "peer-client", "resync-suspended", "received", "sent", "out-of-sync", "pending", "unacked"}

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
	"Down":      10,

	"default": 1,
}

var quorumDangerScores = map[string]uint64 {
	"yes":       0,
	"no":        30,

	"default":   0,
}

var quorumLostKeyword = "no"

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
	// This function is in a critical path and has been optimized. If you modify it,
	// please update the comments accordingly and make sure you're not degrading
	// performance.

	// Dynamically assign event fields for all events, reguardless of event target.
	fields := make(map[string]string)

	// Save a copy of the string we were passed for error reporting.
	original := e

	timeStamp, err := fastTimeParse(e)
	if err != nil {
		return Event{Fields: fields}, err
	}

	if len(e) < 33 {
		return Event{Fields: fields}, fmt.Errorf("Couldn't parse event from %q: too short", original)
	}

	// Chop off the date and the following space from the start of the string and use the rest of it.
	e = e[33:]

	// The event type is followed by a space, get the index of that space so we
	// know which index to assign to eType.
	end := strings.Index(e, " ")
	if end < 0 {
		return Event{Fields: fields}, fmt.Errorf("Couldn't parse event type from %q", original)
	}
	eType := e[:end]

	// Chop off the type and the following space from the start of the string and use the rest of it.
	e = e[end+1:]

	// This is an empty event without fields, we should return now.
	if e == "-" {
		return Event{TimeStamp: timeStamp, EventType: eType, Target: e}, nil
	}

	// The event target is also followed by a space, get the index of that space so we
	// know which index to assign to eTarget.
	end = strings.Index(e, " ")
	if end < 0 {
		return Event{Fields: fields}, fmt.Errorf("Couldn't parse event target from %q", original)
	}
	eTarget := e[:end]

	// Chop off the target and the following space from the start of the string and use the rest of it.
	// Now only the fields should be left.
	e = e[end+1:]
	end = strings.Index(e, " ")
	if end < 0 {
		return Event{Fields: fields}, fmt.Errorf("Couldn't parse event fields from %q", original)
	}

	// Loop until we can't find the next kvPair.
	for end != -1 {
		kvPair := e[:end]

		// Splitting strings is expensive and this loop runs a lot, so we use the
		// index of ":" to break up the key value pairs.
		i := strings.Index(kvPair, ":")
		if i < 0 {
			return Event{Fields: fields}, fmt.Errorf("Couldn't parse key/value pair from %q", kvPair)
		}
		fields[kvPair[:i]] = kvPair[i+1:]

		// Chop off the kvPair we just added to the fields and the following space
		// from the start of the string and use the rest of it.
		e = e[end+1:]

		end = strings.Index(e, " ")
	}

	// Parse the last kvPair.
	kvPair := e
	i := strings.Index(kvPair, ":")
	if i < 0 {
		return Event{Fields: fields}, fmt.Errorf("Couldn't parse key/value pair from %q", kvPair)
	}
	fields[kvPair[:i]] = kvPair[i+1:]

	return Event{
		TimeStamp: timeStamp,
		EventType: eType,
		Target:    eTarget,
		Fields:    fields,
	}, nil
}

// NewEOF returns a special Event signaling that no further input should be expected.
func NewEOF() Event {
	return Event{Target: EOF}
}

// NewDisplayEvent returns a special Event signaling that the display should be updated
func NewDisplayEvent() Event {
	return Event{Target: DisplayEvent}
}

// NewPruneEvent returns a special Event signaling that the display should prune outdated data
func NewPruneEvent() Event {
	return Event{Target: PruneEvent}
}

// NewUnconfiguredRes returns a special Event signaling that this resource is down(unconfigured).
func NewUnconfiguredRes(name string) Event {
	return Event{
		TimeStamp: time.Now(),
		EventType: "exists",
		Target:    "resource",
		Fields: map[string]string{
			ResKeys.Unconfigured: "true",
			ResKeys.Name:         name,
			ResKeys.Role:         "Down",
		},
	}
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
	Unconfigured  bool

	// Calulated Values
	Danger uint64
}

// Update the values of Resource with a new Event.
func (r *Resource) Update(e Event) {
	r.Lock()
	defer r.Unlock()

	r.Name = e.Fields[ResKeys.Name]
	r.Role = e.Fields[ResKeys.Role]
	r.Suspended = e.Fields[ResKeys.Suspended]
	r.WriteOrdering = e.Fields[ResKeys.WriteOrdering]
	if _, ok := e.Fields[ResKeys.Unconfigured]; ok {
		r.Unconfigured = true
	} else {
		r.Unconfigured = false
	}
	r.updateTimes(e.TimeStamp)

	i, ok := roleDangerScores[r.Role]
	if !ok {
		r.Danger = roleDangerScores["default"]
	} else {
		r.Danger = i
	}
}

// Connection represents the connection from the local resource to a remote resource.
type Connection struct {
	sync.RWMutex
	uptimer
	Resource         string
	PeerNodeID       string
	ConnectionName   string
	ConnectionStatus string
	// Long form explanation of ConnectionStatus.
	ConnectionHint string
	Role           string
	Congested      string

	// Calculated Values
	Danger uint64
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
	c.setDanger()
	c.connStatusExplanation()
}

func (c *Connection) setDanger() {
	var score uint64

	i, ok := connDangerScores[c.ConnectionStatus]
	if !ok {
		score += connDangerScores["default"]
	} else {
		score += i
	}

	i, ok = roleDangerScores[c.Role]
	if !ok {
		score += roleDangerScores["default"]
	} else {
		score += i
	}

	if c.Congested != "no" {
		score++
	}

	c.Danger = score
}

func (c *Connection) connStatusExplanation() {
	switch c.ConnectionStatus {
	case "StandAlone":
		c.ConnectionHint = fmt.Sprintf("dropped connection or disconnected manually. try running drbdadm connect %s", c.Resource)
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
	vol, ok := d.Volumes[e.Fields[DevKeys.Volume]]
	if !ok {
		vol = NewDevVolume(200)
		d.Volumes[e.Fields[DevKeys.Volume]] = vol
	}

	vol.uptimer.updateTimes(e.TimeStamp)
	vol.Minor = e.Fields[DevKeys.Minor]
	vol.DiskState = e.Fields[DevKeys.Disk]
	vol.DiskHint = diskStateExplanation(vol.DiskState)
	vol.Client = e.Fields[DevKeys.Client]
	vol.Quorum = e.Fields[DevKeys.Quorum]
	vol.ActivityLogSuspended = e.Fields[DevKeys.ALSuspended]
	vol.Blocked = e.Fields[DevKeys.Blocked]

	if vol.Quorum == quorumLostKeyword {
		vol.QuorumAlert = true
	} else {
		vol.QuorumAlert = false
	}

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

	d.setDanger()
}

func (d *Device) setDanger() {
	var score uint64

	for _, v := range d.Volumes {
		i, ok := diskDangerScores[v.DiskState]
		if !ok {
			score += diskDangerScores["default"]
			// If we're diskless on purpose, then everything is normal.
		} else if !(v.DiskState == "Diskless" && v.Client == "yes") {
			score += i
		}
		i, ok = quorumDangerScores[v.Quorum]
		if ok {
			score += i
		}
	}

	d.Danger = score
}

// DevVolume represents a single volume of a local DRBD virtual block device.
type DevVolume struct {
	uptimer
	Minor     string
	DiskState string
	// Long from explanation of DiskState.
	DiskHint             string
	Client               string
	Quorum               string
	Size                 uint64
	ActivityLogSuspended string
	Blocked              string
	QuorumAlert          bool

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

func diskStateExplanation(dState string) string {
	switch dState {
	case "Diskless":
		return "detached from backing disk"
	case "Attaching":
		return "reading metadata"
	case "Failed":
		return "I/O failure reported by backing disk"
	case "Negotiating":
		return "communicating with peer..."
	case "Inconsistent":
		return "data is not accessible or usable until resync is complete"
	case "Outdated":
		return "data is usable, but a peer has newer data"
	case "Consistent":
		return "data is usable, but we have no network connection"
	case "UpToDate":
		return "normal disk state"
	default:
		return "unknown disk state!"
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
	vol, ok := p.Volumes[e.Fields[PeerDevKeys.Volume]]
	if !ok {
		vol = NewPeerDevVol(200)
		p.Volumes[e.Fields[PeerDevKeys.Volume]] = vol
	}

	vol.updateTimes(e.TimeStamp)

	vol.ReplicationStatus = e.Fields[PeerDevKeys.Replication]
	vol.ReplicationHint = p.replicationExplanation(vol)
	vol.DiskState = e.Fields[PeerDevKeys.PeerDisk]
	vol.DiskHint = diskStateExplanation(vol.DiskState)
	vol.Client = e.Fields[PeerDevKeys.PeerClient]
	vol.ResyncSuspended = e.Fields[PeerDevKeys.ResyncSuspended]

	vol.OutOfSyncKiB.calculate(e.Fields[PeerDevKeys.OutOfSync])
	vol.PendingWrites.calculate(e.Fields[PeerDevKeys.Pending])
	vol.UnackedWrites.calculate(e.Fields[PeerDevKeys.Unacked])

	vol.ReceivedKiB.calculate(vol.Uptime, e.Fields[PeerDevKeys.Received])
	vol.SentKiB.calculate(vol.Uptime, e.Fields[PeerDevKeys.Sent])

	p.setDanger()
}

func (p *PeerDevice) setDanger() {
	var score uint64

	for _, v := range p.Volumes {
		i, ok := diskDangerScores[v.DiskState]
		if !ok {
			score += diskDangerScores["default"]
		} else if !(v.DiskState == "Diskless" && v.Client == "yes") {
			score += i
		}

		// Resources can be up to 1 PiB, so this will be at most 12.
		if v.OutOfSyncKiB.Current != 0 {
			score += uint64(math.Log(float64(v.OutOfSyncKiB.Current)))
		}
	}

	p.Danger = score
}

func (p *PeerDevice) replicationExplanation(v *PeerDevVol) string {
	switch v.ReplicationStatus {
	case "Off":
		return fmt.Sprintf("not replicating to %s", p.ConnectionName)
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
	// Long form explanation of Replication Status.
	ReplicationHint string
	DiskState       string
	DiskHint        string
	Client          string
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

// Significantly faster than using time.Parse since we have a fixed format: "2006-01-02T15:04:05.000000-07:00"
// Adapted from http://stackoverflow.com/questions/27216457/best-way-of-parsing-date-and-time-in-golang
func fastTimeParse(date string) (time.Time, error) {
	// Our time format is 32 characters long, even if we only use chars 0-25 to parse it
	// we should make sure the date string at least conforms to that.
	if len(date) < 32 {
		return time.Time{}, fmt.Errorf("Can't parse date from %q", date)
	}
	year := (((int(date[0])-'0')*10+int(date[1])-'0')*10+int(date[2])-'0')*10 + int(date[3]) - '0'
	month := time.Month((int(date[5])-'0')*10 + int(date[6]) - '0')
	day := (int(date[8])-'0')*10 + int(date[9]) - '0'
	hour := (int(date[11])-'0')*10 + int(date[12]) - '0'
	minute := (int(date[14])-'0')*10 + int(date[15]) - '0'
	second := (int(date[17])-'0')*10 + int(date[18]) - '0'
	nano := (((((int(date[20])-'0')*10+int(date[21])-'0')*10+int(date[22])-'0')*10+int(date[23])-'0')*10+int(date[24])-'0')*10 + int(date[25]) - '0'
	return time.Date(year, month, day, hour, minute, second, nano, time.Local), nil
}
