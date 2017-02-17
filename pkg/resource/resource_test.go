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

func TestResourceUpdate(t *testing.T) {
	timeStamp, err := time.Parse(timeFormat, "2017-02-15T12:57:53.000000-08:00")
	if err != nil {
		t.Error(err)
	}

	status := Status{}
	event := Event{
		timeStamp: timeStamp,
		target:    "resource",
		fields: map[string]string{
			"name":           "test0",
			"role":           "Primary",
			"suspended":      "no",
			"write-ordering": "flush",
		},
	}

	// Update should populate an empty Status.
	status.Update(event)

	if status.Name != event.fields[resKeys[resName]] {
		t.Errorf("Expected status.Name to be %q, got %q", event.fields["name"], status.Name)
	}
	if status.Role != event.fields[resKeys[resRole]] {
		t.Errorf("Expected status.Role to be %q, got %q", event.fields["role"], status.Role)
	}
	if status.Suspended != event.fields[resKeys[resSuspended]] {
		t.Errorf("Expected status.Suspended to be %q, got %q", event.fields["suspended"], status.Suspended)
	}
	if status.WriteOrdering != event.fields[resKeys[resWriteOrdering]] {
		t.Errorf("Expected status.WriteOrdering to be %q, got %q", event.fields["write-ordering"], status.WriteOrdering)
	}
	if status.StartTime != event.timeStamp {
		t.Errorf("Expected status.StartTime to be %q, got %q", event.timeStamp, status.StartTime)
	}
	// Start and current time should match when first created.
	if status.CurrentTime != event.timeStamp {
		t.Errorf("Expected status.CurrentTime to be %q, got %q", event.timeStamp, status.StartTime)
	}

	// Update should update an exsisting Status.
	event = Event{
		timeStamp: timeStamp.Add(time.Millisecond * 500),
		target:    "resource",
		fields: map[string]string{
			"name":           "test0",
			"role":           "Secondary",
			"suspended":      "no",
			"write-ordering": "drain",
		},
	}

	status.Update(event)

	if status.Name != event.fields[resKeys[resName]] {
		t.Errorf("Expected status.Name to be %q, got %q", event.fields["name"], status.Name)
	}
	if status.Role != event.fields[resKeys[resRole]] {
		t.Errorf("Expected status.Role to be %q, got %q", event.fields["role"], status.Role)
	}
	if status.Suspended != event.fields[resKeys[resSuspended]] {
		t.Errorf("Expected status.Suspended to be %q, got %q", event.fields["suspended"], status.Suspended)
	}
	if status.WriteOrdering != event.fields[resKeys[resWriteOrdering]] {
		t.Errorf("Expected status.WriteOrdering to be %q, got %q", event.fields["write-ordering"], status.WriteOrdering)
	}
	if status.StartTime != timeStamp {
		t.Errorf("Expected status.StartTime to be %q, got %q", timeStamp, status.StartTime)
	}
	if status.CurrentTime != event.timeStamp {
		t.Errorf("Expected status.CurrentTime to be %q, got %q", event.timeStamp, status.CurrentTime)
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

	status := Status{
		Connections: make(map[string]*Connection),
	}
	event := Event{
		timeStamp: timeStamp,
		target:    "connection",
		fields: map[string]string{
			connKeys[connName]:       "test0",
			connKeys[connPeerNodeID]: "1",
			connKeys[connConnName]:   "bob",
			connKeys[connConnection]: "connected",
			connKeys[connRole]:       "secondary",
			connKeys[connCongested]:  "no",
		},
	}

	// Update should create a new connection if there isn't one.
	status.Update(event)

	name := event.fields[connKeys[connConnName]]
	conn := status.Connections[name]

	if conn.connectionName != event.fields[connKeys[connConnName]] {
		t.Errorf("Expected status.Connections[%q].connectionName to be %q, got %q", name, event.fields[connKeys[connName]], conn.connectionName)
	}
	if conn.peerNodeID != event.fields[connKeys[connPeerNodeID]] {
		t.Errorf("Expected status.Connections[%q].peerNodeID to be %q, got %q", name, event.fields[connKeys[connPeerNodeID]], conn.peerNodeID)
	}
	if conn.connectionStatus != event.fields[connKeys[connConnection]] {
		t.Errorf("Expected status.Connections[%q].connectionStatus to be %q, got %q", name, event.fields[connKeys[connConnection]], conn.connectionStatus)
	}
	if conn.role != event.fields[connKeys[connRole]] {
		t.Errorf("Expected status.Connections[%q].role to be %q, got %q", name, event.fields[connKeys[connRole]], conn.role)
	}
	if conn.congested != event.fields[connKeys[connCongested]] {
		t.Errorf("Expected status.Connections[%q].congested to be %q, got %q", name, event.fields[connKeys[connCongested]], conn.congested)
	}
	if conn.updateCount != 1 {
		t.Errorf("Expected status.Connections[%q].updateCount to be %d, got %d", name, 1, conn.updateCount)
	}

	event = Event{
		timeStamp: timeStamp,
		target:    "connection",
		fields: map[string]string{
			connKeys[connName]:       "test0",
			connKeys[connPeerNodeID]: "1",
			connKeys[connConnName]:   "bob",
			connKeys[connConnection]: "connected",
			connKeys[connRole]:       "Primary",
			connKeys[connCongested]:  "yes",
		},
	}

	// Update should update a new connection if one exists.
	status.Update(event)

	name = event.fields[connKeys[connConnName]]
	conn = status.Connections[name]

	if conn.connectionName != event.fields[connKeys[connConnName]] {
		t.Errorf("Expected status.Connections[%q].connectionName to be %q, got %q", name, event.fields[connKeys[connName]], conn.connectionName)
	}
	if conn.peerNodeID != event.fields[connKeys[connPeerNodeID]] {
		t.Errorf("Expected status.Connections[%q].peerNodeID to be %q, got %q", name, event.fields[connKeys[connPeerNodeID]], conn.peerNodeID)
	}
	if conn.connectionStatus != event.fields[connKeys[connConnection]] {
		t.Errorf("Expected status.Connections[%q].connectionStatus to be %q, got %q", name, event.fields[connKeys[connConnection]], conn.connectionStatus)
	}
	if conn.role != event.fields[connKeys[connRole]] {
		t.Errorf("Expected status.Connections[%q].role to be %q, got %q", name, event.fields[connKeys[connRole]], conn.role)
	}
	if conn.congested != event.fields[connKeys[connCongested]] {
		t.Errorf("Expected status.Connections[%q].congested to be %q, got %q", name, event.fields[connKeys[connCongested]], conn.congested)
	}
	if conn.updateCount != 2 {
		t.Errorf("Expected status.Connections[%q].updateCount to be %d, got %d", name, 1, conn.updateCount)
	}
}
