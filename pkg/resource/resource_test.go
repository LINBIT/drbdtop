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

	if status.Name != event.fields[resourceFieldKeys[resourceName]] {
		t.Errorf("Expected status.Name to be %q, got %q", event.fields["name"], status.Name)
	}
	if status.Role != event.fields[resourceFieldKeys[resourceRole]] {
		t.Errorf("Expected status.Role to be %q, got %q", event.fields["role"], status.Role)
	}
	if status.Suspended != event.fields[resourceFieldKeys[resourceSuspended]] {
		t.Errorf("Expected status.Suspended to be %q, got %q", event.fields["suspended"], status.Suspended)
	}
	if status.WriteOrdering != event.fields[resourceFieldKeys[resourceWriteOrdering]] {
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

	if status.Name != event.fields[resourceFieldKeys[resourceName]] {
		t.Errorf("Expected status.Name to be %q, got %q", event.fields["name"], status.Name)
	}
	if status.Role != event.fields[resourceFieldKeys[resourceRole]] {
		t.Errorf("Expected status.Role to be %q, got %q", event.fields["role"], status.Role)
	}
	if status.Suspended != event.fields[resourceFieldKeys[resourceSuspended]] {
		t.Errorf("Expected status.Suspended to be %q, got %q", event.fields["suspended"], status.Suspended)
	}
	if status.WriteOrdering != event.fields[resourceFieldKeys[resourceWriteOrdering]] {
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
