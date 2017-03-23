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
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestUpdateTime(t *testing.T) {
	timeStamp, err := time.Parse(timeFormat, "2017-02-15T12:57:53.000000-08:00")
	if err != nil {
		t.Error(err)
	}

	up := uptimer{}

	up.updateTimes(timeStamp)

	if up.StartTime != timeStamp {
		t.Errorf("Expected StartTime to be %q, got %q", timeStamp, up.StartTime)
	}
	if up.CurrentTime != timeStamp {
		t.Errorf("Expected CurrentTime to be %q, got %q", timeStamp, up.CurrentTime)
	}
	if up.Uptime != 0 {
		t.Errorf("Expected Uptime to be %d, got %q", 0, up.Uptime)
	}

	nextTime := timeStamp.Add(time.Second * 4)
	up.updateTimes(nextTime)

	if up.StartTime != timeStamp {
		t.Errorf("Expected StartTime to be %q, got %q", timeStamp, up.StartTime)
	}
	if up.CurrentTime != nextTime {
		t.Errorf("Expected CurrentTime to be %q, got %q", nextTime, up.CurrentTime)
	}
	if up.Uptime != time.Second*4 {
		t.Errorf("Expected Uptime to be %q, got %q", time.Second*4, up.Uptime)
	}
}

func TestMaxAvgCurrent(t *testing.T) {
	stats := newMinMaxAvgCurrent()

	stats.calculate("5")

	if stats.Max != 5 {
		t.Errorf("Expected Max to be %d, got %d", 5, stats.Max)
	}
	if stats.Min != 5 {
		t.Errorf("Expected Min to be %d, got %d", 5, stats.Min)
	}
	if stats.Avg != float64(5) {
		t.Errorf("Expected Avg to be %f, got %f", float64(5), stats.Avg)
	}
	if stats.Current != 5 {
		t.Errorf("Expected Current to be %d, got %d", 5, stats.Current)
	}

	stats.calculate("10")

	if stats.Max != 10 {
		t.Errorf("Expected Max to be %d, got %d", 10, stats.Max)
	}
	if stats.Min != 5 {
		t.Errorf("Expected Min to be %d, got %d", 5, stats.Min)
	}
	if stats.Avg != float64(7.5) {
		t.Errorf("Expected Avg to be %f, got %f", float64(7.5), stats.Avg)
	}
	if stats.Current != 10 {
		t.Errorf("Expected Current to be %d, got %d", 10, stats.Current)
	}
}

func TestRate(t *testing.T) {
	r := &rate{Previous: &previousFloat64{maxLen: 5}, new: true}

	r.calculate(time.Second*0, "100")

	if r.initial != 100 {
		t.Errorf("Expected initial to be %d, got %d", 100, r.initial)
	}
	if r.last != 100 {
		t.Errorf("Expected current to be %d, got %d", 100, r.last)
	}
	if !reflect.DeepEqual(r.Previous.Values, []float64{0}) {
		t.Errorf("Expected Previous.Values to be %v, got %v", []float64{0}, r.Previous.Values)
	}
	if r.PerSecond != 0 {
		t.Errorf("Expected PerSecond to be %d, got %f", 0, r.PerSecond)
	}
	if r.Total != 0 {
		t.Errorf("Expected total to be %d, got %d", 0, r.Total)
	}

	// Deal with data not increasing.

	r.calculate(time.Second*1, "200")

	if r.initial != 100 {
		t.Errorf("Expected initial to be %d, got %d", 100, r.initial)
	}
	if r.last != 200 {
		t.Errorf("Expected current to be %d, got %d", 200, r.last)
	}
	if !reflect.DeepEqual(r.Previous.Values, []float64{0, 100}) {
		t.Errorf("Expected Previous.Values to be %v, got %v", []float64{0, 100}, r.Previous.Values)
	}
	if r.PerSecond != float64(100) {
		t.Errorf("Expected PerSecond to be %d, got %f", 100, r.PerSecond)
	}
	if r.Total != 100 {
		t.Errorf("Expected total to be %d, got %d", 100, r.Total)
	}

	r.calculate(time.Second*2, "200")

	if r.initial != 100 {
		t.Errorf("Expected initial to be %d, got %d", 100, r.initial)
	}
	if r.last != 200 {
		t.Errorf("Expected current to be %d, got %d", 200, r.last)
	}
	if !reflect.DeepEqual(r.Previous.Values, []float64{0, 100, 50}) {
		t.Errorf("Expected Previous.Values to be %v, got %v", []float64{0, 100, 50}, r.Previous.Values)
	}
	if r.PerSecond != float64(50) {
		t.Errorf("Expected PerSecond to be %d, got %f", 100, r.PerSecond)
	}
	if r.Total != 100 {
		t.Errorf("Expected total to be %d, got %d", 100, r.Total)
	}

	r.calculate(time.Second*3, "200")

	if r.initial != 100 {
		t.Errorf("Expected initial to be %d, got %d", 100, r.initial)
	}
	if r.last != 200 {
		t.Errorf("Expected current to be %d, got %d", 200, r.last)
	}
	if r.Total != 100 {
		t.Errorf("Expected total to be %d, got %d", 100, r.Total)
	}

	// Non-monotonic pattern, reset initial value and calulate total correctly.
	r.calculate(time.Second*4, "50")
	if r.Total != 150 {
		t.Errorf("Failed to reset total value, total is %d, expected %d: %v", r.Total, 150, r)
	}
}

func TestPreviousFloat64(t *testing.T) {
	prev := previousFloat64{maxLen: 2}

	prev.Push(10.10)
	if !reflect.DeepEqual(prev.Values, []float64{10.10}) {
		t.Errorf("Expected Values to be %v, got %v", []float64{10.10}, prev.Values)
	}
	prev.Push(15.9)
	prev.Push(200.5)
	if !reflect.DeepEqual(prev.Values, []float64{15.9, 200.5}) {
		t.Errorf("Expected Values to be %v, got %v", []float64{15.9, 200.5}, prev.Values)
	}
}

func TestResourceUpdate(t *testing.T) {
	timeStamp, err := time.Parse(timeFormat, "2017-02-15T12:57:53.000000-08:00")
	if err != nil {
		t.Error(err)
	}

	status := Resource{}
	event := Event{
		TimeStamp: timeStamp,
		Target:    "resource",
		Fields: map[string]string{
			"name":           "test0",
			"role":           "Primary",
			"suspended":      "no",
			"write-ordering": "flush",
		},
	}

	// Update should populate an empty Status.
	status.Update(event)

	if status.Name != event.Fields[resKeys[resName]] {
		t.Errorf("Expected status.Name to be %q, got %q", event.Fields["name"], status.Name)
	}
	if status.Role != event.Fields[resKeys[resRole]] {
		t.Errorf("Expected status.Role to be %q, got %q", event.Fields["role"], status.Role)
	}
	if status.Suspended != event.Fields[resKeys[resSuspended]] {
		t.Errorf("Expected status.Suspended to be %q, got %q", event.Fields["suspended"], status.Suspended)
	}
	if status.WriteOrdering != event.Fields[resKeys[resWriteOrdering]] {
		t.Errorf("Expected status.WriteOrdering to be %q, got %q", event.Fields["write-ordering"], status.WriteOrdering)
	}
	if status.StartTime != event.TimeStamp {
		t.Errorf("Expected status.StartTime to be %q, got %q", event.TimeStamp, status.StartTime)
	}
	// Start and current time should match when first created.
	if status.CurrentTime != event.TimeStamp {
		t.Errorf("Expected status.CurrentTime to be %q, got %q", event.TimeStamp, status.StartTime)
	}

	// Update should update an exsisting Status.
	event = Event{
		TimeStamp: timeStamp.Add(time.Millisecond * 500),
		Target:    "resource",
		Fields: map[string]string{
			"name":           "test0",
			"role":           "Secondary",
			"suspended":      "no",
			"write-ordering": "drain",
		},
	}

	status.Update(event)

	if status.Name != event.Fields[resKeys[resName]] {
		t.Errorf("Expected status.Name to be %q, got %q", event.Fields["name"], status.Name)
	}
	if status.Role != event.Fields[resKeys[resRole]] {
		t.Errorf("Expected status.Role to be %q, got %q", event.Fields["role"], status.Role)
	}
	if status.Suspended != event.Fields[resKeys[resSuspended]] {
		t.Errorf("Expected status.Suspended to be %q, got %q", event.Fields["suspended"], status.Suspended)
	}
	if status.WriteOrdering != event.Fields[resKeys[resWriteOrdering]] {
		t.Errorf("Expected status.WriteOrdering to be %q, got %q", event.Fields["write-ordering"], status.WriteOrdering)
	}
	if status.StartTime != timeStamp {
		t.Errorf("Expected status.StartTime to be %q, got %q", timeStamp, status.StartTime)
	}
	if status.CurrentTime != event.TimeStamp {
		t.Errorf("Expected status.CurrentTime to be %q, got %q", event.TimeStamp, status.CurrentTime)
	}
	// Start and current time should match when first created.
	if status.CurrentTime == status.StartTime {
		t.Errorf("Expected status.CurrentTime %q, and status.startTime %q to differ.", status.CurrentTime, status.StartTime)
	}
}

func TestConnectionUpdate(t *testing.T) {
	timeStamp, err := time.Parse(timeFormat, "2017-02-15T12:57:53.000000-08:00")
	if err != nil {
		t.Error(err)
	}

	conn := Connection{}
	event := Event{
		TimeStamp: timeStamp,
		Target:    "connection",
		Fields: map[string]string{
			connKeys[connName]:       "test0",
			connKeys[connPeerNodeID]: "1",
			connKeys[connConnName]:   "bob",
			connKeys[connConnection]: "connected",
			connKeys[connRole]:       "secondary",
			connKeys[connCongested]:  "no",
		},
	}

	// Update should create a new connection if there isn't one.
	conn.Update(event)

	name := event.Fields[connKeys[connConnName]]

	if conn.ConnectionName != event.Fields[connKeys[connConnName]] {
		t.Errorf("Expected status.Connections[%q].connectionName to be %q, got %q", name, event.Fields[connKeys[connName]], conn.ConnectionName)
	}
	if conn.PeerNodeID != event.Fields[connKeys[connPeerNodeID]] {
		t.Errorf("Expected status.Connections[%q].peerNodeID to be %q, got %q", name, event.Fields[connKeys[connPeerNodeID]], conn.PeerNodeID)
	}
	if conn.ConnectionStatus != event.Fields[connKeys[connConnection]] {
		t.Errorf("Expected status.Connections[%q].connectionStatus to be %q, got %q", name, event.Fields[connKeys[connConnection]], conn.ConnectionStatus)
	}
	if conn.Role != event.Fields[connKeys[connRole]] {
		t.Errorf("Expected status.Connections[%q].role to be %q, got %q", name, event.Fields[connKeys[connRole]], conn.Role)
	}
	if conn.Congested != event.Fields[connKeys[connCongested]] {
		t.Errorf("Expected status.Connections[%q].congested to be %q, got %q", name, event.Fields[connKeys[connCongested]], conn.Congested)
	}
	if conn.updateCount != 1 {
		t.Errorf("Expected status.Connections[%q].updateCount to be %d, got %d", name, 1, conn.updateCount)
	}

	event = Event{
		TimeStamp: timeStamp,
		Target:    "connection",
		Fields: map[string]string{
			connKeys[connName]:       "test0",
			connKeys[connPeerNodeID]: "1",
			connKeys[connConnName]:   "bob",
			connKeys[connConnection]: "connected",
			connKeys[connRole]:       "Primary",
			connKeys[connCongested]:  "yes",
		},
	}

	// Update should update a new connection if one exists.
	conn.Update(event)

	name = event.Fields[connKeys[connConnName]]

	if conn.ConnectionName != event.Fields[connKeys[connConnName]] {
		t.Errorf("Expected status.Connections[%q].connectionName to be %q, got %q", name, event.Fields[connKeys[connName]], conn.ConnectionName)
	}
	if conn.PeerNodeID != event.Fields[connKeys[connPeerNodeID]] {
		t.Errorf("Expected status.Connections[%q].peerNodeID to be %q, got %q", name, event.Fields[connKeys[connPeerNodeID]], conn.PeerNodeID)
	}
	if conn.ConnectionStatus != event.Fields[connKeys[connConnection]] {
		t.Errorf("Expected status.Connections[%q].connectionStatus to be %q, got %q", name, event.Fields[connKeys[connConnection]], conn.ConnectionStatus)
	}
	if conn.Role != event.Fields[connKeys[connRole]] {
		t.Errorf("Expected status.Connections[%q].role to be %q, got %q", name, event.Fields[connKeys[connRole]], conn.Role)
	}
	if conn.Congested != event.Fields[connKeys[connCongested]] {
		t.Errorf("Expected status.Connections[%q].congested to be %q, got %q", name, event.Fields[connKeys[connCongested]], conn.Congested)
	}
	if conn.updateCount != 2 {
		t.Errorf("Expected status.Connections[%q].updateCount to be %d, got %d", name, 1, conn.updateCount)
	}
}

func TestDeviceUpdate(t *testing.T) {
	timeStamp, err := time.Parse(timeFormat, "2017-02-15T12:57:53.000000-08:00")
	if err != nil {
		t.Error(err)
	}

	dev := Device{Volumes: make(map[string]*DevVolume)}

	event := Event{
		TimeStamp: timeStamp,
		Target:    "device",
		Fields: map[string]string{
			devKeys[devName]:         "test0",
			devKeys[devVolume]:       "0",
			devKeys[devMinor]:        "0",
			devKeys[devDisk]:         "UpToDate",
			devKeys[devSize]:         "5533366723",
			devKeys[devRead]:         "100001",
			devKeys[devWritten]:      "10012",
			devKeys[devALWrites]:     "30032",
			devKeys[devBMWrites]:     "0",
			devKeys[devUpperPending]: "2",
			devKeys[devLowerPending]: "2",
			devKeys[devALSuspended]:  "no",
			devKeys[devBlocked]:      "no",
		},
	}

	dev.Update(event)

	vol := event.Fields[devKeys[devVolume]]

	if dev.Resource != event.Fields[devKeys[devName]] {
		t.Errorf("Expected dev.resource to be %q, got %q", event.Fields[devKeys[devName]], dev.Resource)
	}
	if dev.Volumes[vol].Minor != event.Fields[devKeys[devMinor]] {
		t.Errorf("Expected dev.volumes[%q].minor to be %q, got %q", vol, event.Fields[devKeys[devMinor]], dev.Volumes[vol].Minor)
	}
	if dev.Volumes[vol].DiskState != event.Fields[devKeys[devDisk]] {
		t.Errorf("Expected dev.volumes[%q].DiskState to be %q, got %q", vol, event.Fields[devKeys[devDisk]], dev.Volumes[vol].DiskState)
	}

	size, _ := strconv.ParseUint(event.Fields[devKeys[devSize]], 10, 64)
	if dev.Volumes[vol].Size != size {
		t.Errorf("Expected dev.volumes[%q].size to be %q, got %d", vol, event.Fields[devKeys[devSize]], dev.Volumes[vol].Size)
	}
	if dev.Volumes[event.Fields[devKeys[devVolume]]].ReadKiB.Total != 0 {
		t.Errorf("Expected dev.volumes[%q].ReadKib.Total to be %q, got %q", vol, 0, dev.Volumes[vol].ReadKiB.Total)
	}
}

func TestPeerDeviceUpdate(t *testing.T) {
	timeStamp, err := time.Parse(timeFormat, "2017-02-15T12:57:53.000000-08:00")
	if err != nil {
		t.Error(err)
	}

	dev := PeerDevice{Volumes: make(map[string]*PeerDevVol)}

	event := Event{
		TimeStamp: timeStamp,
		Target:    "peer-device",
		Fields: map[string]string{
			peerDevKeys[peerDevName]:            "test0",
			peerDevKeys[peerDevConnName]:        "peer",
			peerDevKeys[peerDevVolume]:          "0",
			peerDevKeys[peerDevReplication]:     "SyncSource",
			peerDevKeys[peerDevPeerDisk]:        "Inconsistent",
			peerDevKeys[peerDevResyncSuspended]: "no",
			peerDevKeys[peerDevReceived]:        "100",
			peerDevKeys[peerDevSent]:            "500",
			peerDevKeys[peerDevOutOfSync]:       "200000",
			peerDevKeys[peerDevPending]:         "0",
			peerDevKeys[peerDevUnacked]:         "0",
		},
	}

	dev.Update(event)

	vol := event.Fields[peerDevKeys[peerDevVolume]]

	if dev.Resource != event.Fields[devKeys[devName]] {
		t.Errorf("Expected dev.resource to be %q, got %q", event.Fields[peerDevKeys[peerDevName]], dev.Resource)
	}
	if dev.Volumes[vol].ReplicationStatus != event.Fields[peerDevKeys[peerDevReplication]] {
		t.Errorf("Expected dev.volumes[%q].replicationStatus to be %q, got %q", vol, event.Fields[peerDevKeys[peerDevReplication]], dev.Volumes[vol].ReplicationStatus)
	}
	oos, _ := strconv.ParseUint(event.Fields[peerDevKeys[peerDevOutOfSync]], 10, 64)
	if dev.Volumes[vol].OutOfSyncKiB.Current != oos {
		t.Errorf("Expected dev.volumes[%q].OutOfSyncKiB.Current to be %d, got %d", vol, oos, dev.Volumes[vol].OutOfSyncKiB.Current)
	}
}

func TestNewEvent(t *testing.T) {

	resTimeStamp0, err := time.Parse(timeFormat, "2017-02-22T19:53:58.445263-08:00")
	if err != nil {
		t.Fatal("Unable to parse time format")
	}
	var drbd9Tests = []struct {
		in  string
		out Event
	}{
		{"2017-02-22T19:53:58.445263-08:00 exists resource name:test3 role:Secondary suspended:no write-ordering:flush",
			Event{
				TimeStamp: resTimeStamp0,
				EventType: "exists",
				Target:    "resource",
				Fields: map[string]string{
					resKeys[resName]:          "test3",
					resKeys[resRole]:          "Secondary",
					resKeys[resSuspended]:     "no",
					resKeys[resWriteOrdering]: "flush",
				}},
		},
		{"2017-02-22T19:53:58.445263-08:00 exists connection name:test3 peer-node-id:1 conn-name:tom connection:Connected role:Secondary congested:no",
			Event{
				TimeStamp: resTimeStamp0,
				EventType: "exists",
				Target:    "connection",
				Fields: map[string]string{
					connKeys[connName]:       "test3",
					connKeys[connPeerNodeID]: "1",
					connKeys[connConnName]:   "tom",
					connKeys[connConnection]: "Connected",
					connKeys[connRole]:       "Secondary",
					connKeys[connCongested]:  "no",
				}},
		},
		{"2017-02-22T19:53:58.445263-08:00 exists device name:test3 volume:0 minor:150 disk:UpToDate size:1048576 read:912 written:0 al-writes:0 bm-writes:0 upper-pending:0 lower-pending:0 al-suspended:no blocked:no",
			Event{
				TimeStamp: resTimeStamp0,
				EventType: "exists",
				Target:    "device",
				Fields: map[string]string{
					devKeys[devName]:         "test3",
					devKeys[devVolume]:       "0",
					devKeys[devMinor]:        "150",
					devKeys[devDisk]:         "UpToDate",
					devKeys[devSize]:         "1048576",
					devKeys[devRead]:         "912",
					devKeys[devWritten]:      "0",
					devKeys[devALWrites]:     "0",
					devKeys[devBMWrites]:     "0",
					devKeys[devUpperPending]: "0",
					devKeys[devLowerPending]: "0",
					devKeys[devALSuspended]:  "no",
					devKeys[devBlocked]:      "no",
				}},
		},
		{"2017-02-22T19:53:58.445263-08:00 exists peer-device name:test3 peer-node-id:1 conn-name:tom volume:0 replication:Established peer-disk:UpToDate resync-suspended:no received:10 sent:100 out-of-sync:1000 pending:10000 unacked:100000",
			Event{
				TimeStamp: resTimeStamp0,
				EventType: "exists",
				Target:    "peer-device",
				Fields: map[string]string{
					peerDevKeys[peerDevName]:            "test3",
					peerDevKeys[peerDevNodeID]:          "1",
					peerDevKeys[peerDevConnName]:        "tom",
					peerDevKeys[peerDevVolume]:          "0",
					peerDevKeys[peerDevReplication]:     "Established",
					peerDevKeys[peerDevPeerDisk]:        "UpToDate",
					peerDevKeys[peerDevResyncSuspended]: "no",
					peerDevKeys[peerDevReceived]:        "10",
					peerDevKeys[peerDevSent]:            "100",
					peerDevKeys[peerDevOutOfSync]:       "1000",
					peerDevKeys[peerDevPending]:         "10000",
					peerDevKeys[peerDevUnacked]:         "100000",
				}},
		},
	}

	for _, tt := range drbd9Tests {
		e, _ := NewEvent(tt.in)
		if !reflect.DeepEqual(e, tt.out) {
			t.Errorf("Called: NewEvent(%q)\nExpected: %v\nGot: %v", tt.in, tt.out, e)
		}
	}
}

func TestConnectionDanger(t *testing.T) {
	timeStamp, err := time.Parse(timeFormat, "2017-02-15T12:57:53.000000-08:00")
	if err != nil {
		t.Error(err)
	}

	conn := Connection{}
	event := Event{
		TimeStamp: timeStamp,
		Target:    "connection",
		Fields: map[string]string{
			connKeys[connConnection]: "StandAlone",
			connKeys[connRole]:       "Secondary",
			connKeys[connCongested]:  "no",
		},
	}

	// Update should update the danger level.
	conn.Update(event)

	expectedDanger := uint64(2500000)

	if conn.Danger != expectedDanger {
		t.Errorf("Expected StandAlone to have a danger level of %d, got %d", expectedDanger, conn.Danger)
	}

	event = Event{
		TimeStamp: timeStamp,
		Target:    "connection",
		Fields: map[string]string{
			connKeys[connConnection]: "Connected",
			connKeys[connRole]:       "Unknown",
			connKeys[connCongested]:  "yes",
		},
	}

	conn.Update(event)

	expectedDanger = 1400

	if conn.Danger != expectedDanger {
		t.Errorf("Expected StandAlone to have a danger level of %d, got %d", expectedDanger, conn.Danger)
	}
}
