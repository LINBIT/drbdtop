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

/*
 * New Events are created from strings generated from drbdsetup events2 such as:
 * 2017-03-27T08:28:17.072611-07:00 exists resource name:test0 role:Primary suspended:no write-ordering:flush
 * 2017-03-27T08:28:17.072611-07:00 exists device name:test0 volume:0 minor:0 disk:UpToDate client:no size:4056 read:1340 written:16 al-writes:1 bm-writes:0 upper-pending:0 lower-pending:0 al-suspended:no blocked:no
 * 2017-02-15T14:43:16.688437+00:00 exists connection name:test0 conn-name:peer connection:Connected role:Secondary congested:no
 * 2017-02-15T14:43:16.688437+00:00 exists peer-device name:test0 conn-name:peer volume:0 replication:SyncSource peer-disk:Inconsistent resync-suspended:no received:0 sent:2050743348 out-of-sync:205655500 pending:0 unacked:0
 */

func TestResourceUpdate(t *testing.T) {

	status := Resource{}
	event, err := NewEvent("2017-02-15T12:57:53.000000-08:00 exists resource name:test0 role:Primary suspended:no write-ordering:flush")
	if err != nil {
		t.Fatal(err)
	}

	// Update should populate an empty Status.
	status.Update(event)

	if status.Name != event.Fields[ResKeys.name] {
		t.Errorf("Expected status.Name to be %q, got %q", event.Fields["name"], status.Name)
	}
	if status.Role != event.Fields[ResKeys.role] {
		t.Errorf("Expected status.Role to be %q, got %q", event.Fields["role"], status.Role)
	}
	if status.Suspended != event.Fields[ResKeys.suspended] {
		t.Errorf("Expected status.Suspended to be %q, got %q", event.Fields["suspended"], status.Suspended)
	}
	if status.WriteOrdering != event.Fields[ResKeys.writeOrdering] {
		t.Errorf("Expected status.WriteOrdering to be %q, got %q", event.Fields["write-ordering"], status.WriteOrdering)
	}

	// Update should update an exsisting Status.
	event, err = NewEvent("2017-02-15T12:57:55.000000-08:00 exists resource name:test0 role:Secondary suspended:no write-ordering:drain")
	if err != nil {
		t.Fatal(err)
	}

	status.Update(event)

	if status.Name != event.Fields[ResKeys.name] {
		t.Errorf("Expected status.Name to be %q, got %q", event.Fields["name"], status.Name)
	}
	if status.Role != event.Fields[ResKeys.role] {
		t.Errorf("Expected status.Role to be %q, got %q", event.Fields["role"], status.Role)
	}
	if status.Suspended != event.Fields[ResKeys.suspended] {
		t.Errorf("Expected status.Suspended to be %q, got %q", event.Fields["suspended"], status.Suspended)
	}
	if status.WriteOrdering != event.Fields[ResKeys.writeOrdering] {
		t.Errorf("Expected status.WriteOrdering to be %q, got %q", event.Fields["write-ordering"], status.WriteOrdering)
	}
}

func TestConnectionUpdate(t *testing.T) {
	conn := Connection{}
	event, err := NewEvent("2017-02-15T14:43:16.688437+00:00 exists connection name:test0 conn-name:bob connection:Connected role:Secondary congested:no")
	if err != nil {
		t.Fatal(err)
	}

	// Update should create a new connection if there isn't one.
	conn.Update(event)

	name := event.Fields[ConnKeys.ConnName]

	if conn.ConnectionName != event.Fields[ConnKeys.ConnName] {
		t.Errorf("Expected status.Connections[%q].connectionName to be %q, got %q", name, event.Fields[ConnKeys.ConnName], conn.ConnectionName)
	}
	if conn.PeerNodeID != event.Fields[ConnKeys.PeerNodeID] {
		t.Errorf("Expected status.Connections[%q].peerNodeID to be %q, got %q", name, event.Fields[ConnKeys.PeerNodeID], conn.PeerNodeID)
	}
	if conn.ConnectionStatus != event.Fields[ConnKeys.Connection] {
		t.Errorf("Expected status.Connections[%q].connectionStatus to be %q, got %q", name, event.Fields[ConnKeys.Connection], conn.ConnectionStatus)
	}
	if conn.Role != event.Fields[ConnKeys.Role] {
		t.Errorf("Expected status.Connections[%q].role to be %q, got %q", name, event.Fields[ConnKeys.Role], conn.Role)
	}
	if conn.Congested != event.Fields[ConnKeys.Congested] {
		t.Errorf("Expected status.Connections[%q].congested to be %q, got %q", name, event.Fields[ConnKeys.Congested], conn.Congested)
	}
	if conn.updateCount != 1 {
		t.Errorf("Expected status.Connections[%q].updateCount to be %d, got %d", name, 1, conn.updateCount)
	}

	event, err = NewEvent("2017-02-15T14:43:26.688437+00:00 exists connection name:test0 conn-name:bob connection:Connected role:Primary congested:yes")
	if err != nil {
		t.Fatal(err)
	}

	// Update should update a new connection if one exists.
	conn.Update(event)

	name = event.Fields[ConnKeys.ConnName]

	if conn.ConnectionName != event.Fields[ConnKeys.ConnName] {
		t.Errorf("Expected status.Connections[%q].connectionName to be %q, got %q", name, event.Fields[ConnKeys.ConnName], conn.ConnectionName)
	}
	if conn.PeerNodeID != event.Fields[ConnKeys.PeerNodeID] {
		t.Errorf("Expected status.Connections[%q].peerNodeID to be %q, got %q", name, event.Fields[ConnKeys.PeerNodeID], conn.PeerNodeID)
	}
	if conn.ConnectionStatus != event.Fields[ConnKeys.Connection] {
		t.Errorf("Expected status.Connections[%q].connectionStatus to be %q, got %q", name, event.Fields[ConnKeys.Connection], conn.ConnectionStatus)
	}
	if conn.Role != event.Fields[ConnKeys.Role] {
		t.Errorf("Expected status.Connections[%q].role to be %q, got %q", name, event.Fields[ConnKeys.Role], conn.Role)
	}
	if conn.Congested != event.Fields[ConnKeys.Congested] {
		t.Errorf("Expected status.Connections[%q].congested to be %q, got %q", name, event.Fields[ConnKeys.Congested], conn.Congested)
	}
	if conn.updateCount != 2 {
		t.Errorf("Expected status.Connections[%q].updateCount to be %d, got %d", name, 1, conn.updateCount)
	}
}

func TestDeviceUpdate(t *testing.T) {
	dev := Device{Volumes: make(map[string]*DevVolume)}

	event, err := NewEvent(
		"2017-03-27T08:28:17.072611-07:00 exists device name:test0 volume:0 minor:0 disk:UpToDate " +
			"client:no size:4056 read:1340 written:16 al-writes:1 bm-writes:0 upper-pending:0 " +
			"lower-pending:0 al-suspended:no blocked:no")
	if err != nil {
		t.Fatal(err)
	}

	dev.Update(event)

	vol := event.Fields[DevKeys.Volume]

	if dev.Resource != event.Fields[DevKeys.Name] {
		t.Errorf("Expected dev.resource to be %q, got %q", event.Fields[DevKeys.Name], dev.Resource)
	}
	if dev.Volumes[vol].Minor != event.Fields[DevKeys.Minor] {
		t.Errorf("Expected dev.volumes[%q].minor to be %q, got %q", vol, event.Fields[DevKeys.Minor], dev.Volumes[vol].Minor)
	}
	if dev.Volumes[vol].DiskState != event.Fields[DevKeys.Disk] {
		t.Errorf("Expected dev.volumes[%q].DiskState to be %q, got %q", vol, event.Fields[DevKeys.Disk], dev.Volumes[vol].DiskState)
	}

	size, _ := strconv.ParseUint(event.Fields[DevKeys.Size], 10, 64)
	if dev.Volumes[vol].Size != size {
		t.Errorf("Expected dev.volumes[%q].size to be %q, got %d", vol, event.Fields[DevKeys.Size], dev.Volumes[vol].Size)
	}
	if dev.Volumes[event.Fields[DevKeys.Volume]].ReadKiB.Total != 0 {
		t.Errorf("Expected dev.volumes[%q].ReadKib.Total to be %q, got %q", vol, 0, dev.Volumes[vol].ReadKiB.Total)
	}
}

func TestPeerDeviceUpdate(t *testing.T) {
	dev := PeerDevice{Volumes: make(map[string]*PeerDevVol)}

	event, err := NewEvent(
		"2017-02-15T14:43:16.688437+00:00 exists peer-device name:test0 conn-name:peer " +
			"volume:0 replication:SyncSource peer-disk:Inconsistent resync-suspended:no " +
			"received:0 sent:2050743348 out-of-sync:205655500 pending:0 unacked:0")
	if err != nil {
		t.Fatal(err)
	}

	dev.Update(event)

	vol := event.Fields[PeerDevKeys.Volume]

	if dev.Resource != event.Fields[DevKeys.Name] {
		t.Errorf("Expected dev.resource to be %q, got %q", event.Fields[PeerDevKeys.Name], dev.Resource)
	}
	if dev.Volumes[vol].ReplicationStatus != event.Fields[PeerDevKeys.Replication] {
		t.Errorf("Expected dev.volumes[%q].replicationStatus to be %q, got %q", vol, event.Fields[PeerDevKeys.Replication], dev.Volumes[vol].ReplicationStatus)
	}
	oos, _ := strconv.ParseUint(event.Fields[PeerDevKeys.OutOfSync], 10, 64)
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
					ResKeys.name:          "test3",
					ResKeys.role:          "Secondary",
					ResKeys.suspended:     "no",
					ResKeys.writeOrdering: "flush",
				}},
		},
		{"2017-02-22T19:53:58.445263-08:00 exists connection name:test3 peer-node-id:1 conn-name:tom connection:Connected role:Secondary congested:no",
			Event{
				TimeStamp: resTimeStamp0,
				EventType: "exists",
				Target:    "connection",
				Fields: map[string]string{
					ConnKeys.Name:       "test3",
					ConnKeys.PeerNodeID: "1",
					ConnKeys.ConnName:   "tom",
					ConnKeys.Connection: "Connected",
					ConnKeys.Role:       "Secondary",
					ConnKeys.Congested:  "no",
				}},
		},
		{"2017-02-22T19:53:58.445263-08:00 exists device name:test3 volume:0 minor:150 disk:UpToDate size:1048576 read:912 written:0 al-writes:0 bm-writes:0 upper-pending:0 lower-pending:0 al-suspended:no blocked:no",
			Event{
				TimeStamp: resTimeStamp0,
				EventType: "exists",
				Target:    "device",
				Fields: map[string]string{
					DevKeys.Name:         "test3",
					DevKeys.Volume:       "0",
					DevKeys.Minor:        "150",
					DevKeys.Disk:         "UpToDate",
					DevKeys.Size:         "1048576",
					DevKeys.Read:         "912",
					DevKeys.Written:      "0",
					DevKeys.ALWrites:     "0",
					DevKeys.BMWrites:     "0",
					DevKeys.UpperPending: "0",
					DevKeys.LowerPending: "0",
					DevKeys.ALSuspended:  "no",
					DevKeys.Blocked:      "no",
				}},
		},
		{"2017-02-22T19:53:58.445263-08:00 exists peer-device name:test3 peer-node-id:1 conn-name:tom volume:0 replication:Established peer-disk:UpToDate resync-suspended:no received:10 sent:100 out-of-sync:1000 pending:10000 unacked:100000",
			Event{
				TimeStamp: resTimeStamp0,
				EventType: "exists",
				Target:    "peer-device",
				Fields: map[string]string{
					PeerDevKeys.Name:            "test3",
					PeerDevKeys.PeerNodeID:      "1",
					PeerDevKeys.ConnName:        "tom",
					PeerDevKeys.Volume:          "0",
					PeerDevKeys.Replication:     "Established",
					PeerDevKeys.PeerDisk:        "UpToDate",
					PeerDevKeys.ResyncSuspended: "no",
					PeerDevKeys.Received:        "10",
					PeerDevKeys.Sent:            "100",
					PeerDevKeys.OutOfSync:       "1000",
					PeerDevKeys.Pending:         "10000",
					PeerDevKeys.Unacked:         "100000",
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
	conn := Connection{}
	event, err := NewEvent("2017-02-15T14:43:16.688437+00:00 exists connection " +
		"name:test0 conn-name:peer connection:StandAlone role:Secondary congested:no")
	if err != nil {
		t.Fatal(err)
	}

	// Update should update the danger level.
	conn.Update(event)

	expectedDanger := uint64(30)

	if conn.Danger != expectedDanger {
		t.Errorf("Expected connection to have a danger level of %d, got %d", expectedDanger, conn.Danger)
	}

	event, err = NewEvent("2017-02-15T14:43:16.688437+00:00 exists connection " +
		"name:test0 conn-name:peer connection:Connected role:Unknown congested:yes")
	if err != nil {
		t.Fatal(err)
	}
	conn.Update(event)

	expectedDanger = 2

	if conn.Danger != expectedDanger {
		t.Errorf("Expected connection to have a danger level of %d, got %d", expectedDanger, conn.Danger)
	}
}

func TestDeviceDanger(t *testing.T) {
	dev := NewDevice()

	event, err := NewEvent("2017-03-27T08:28:17.072611-07:00 exists device name:test0 " +
		"volume:0 minor:0 disk:Diskless client:yes size:4056 read:1340 written:16 " +
		"al-writes:1 bm-writes:0 upper-pending:0 lower-pending:0 al-suspended:no blocked:no")
	if err != nil {
		t.Fatal(err)
	}
	dev.Update(event)

	expectedDanger := uint64(0)

	if dev.Danger != expectedDanger {
		t.Errorf("Expected intentionally diskless device to have a danger level of %d, got %d", expectedDanger, dev.Danger)
	}
}

func TestPeerDeviceDanger(t *testing.T) {
	dev := NewPeerDevice()

	event, err := NewEvent("2017-03-27T12:39:29.346495-07:00 exists peer-device " +
		"name:r0 peer-node-id:1 conn-name:mussorgsky volume:0 replication:Established " +
		"peer-disk:UpToDate peer-client:no resync-suspended:no received:0 sent:6278868 " +
		"out-of-sync:0 pending:0 unacked:0")
	if err != nil {
		t.Fatal(err)
	}
	dev.Update(event)

	expectedDanger := uint64(0)

	if dev.Danger != expectedDanger {
		t.Errorf("Expected healthy device to have a danger level of %d, got %d", expectedDanger, dev.Danger)
	}
}
