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

func TestStatusUpdate(t *testing.T) {
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

	// Update should populate an empty event.
	status.Update(event)

	if status.Name != event.fields["name"] {
		t.Errorf("Expected status.Name to be %s", event.fields["name"])
	}
	if status.Role != event.fields["role"] {
		t.Errorf("Expected status.Role to be %s", event.fields["role"])
	}
	if status.Suspended != event.fields["suspended"] {
		t.Errorf("Expected status.Suspended to be %s", event.fields["suspended"])
	}
	if status.Suspended != event.fields["write-ordering"] {
		t.Errorf("Expected status.WriteOrdering to be %s", event.fields["write-ordering"])
	}
	if status.StartTime != event.timeStamp {
		t.Errorf("Expected status.StartTime to be %s", event.timeStamp)
	}
	// Start and current time should match when first created.
	if status.CurrentTime != event.timeStamp {
		t.Errorf("Expected status.CurrentTime to be %s", event.timeStamp)
	}
}
