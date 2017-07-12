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

package update

import (
	"testing"
	"time"

	"github.com/linbit/drbdtop/pkg/resource"
)

/*
 * New Events are created from strings generated from drbdsetup events2 such as:
 * 2017-03-27T08:28:17.072611-07:00 exists resource name:test0 role:Primary suspended:no write-ordering:flush
 * 2017-03-27T08:28:17.072611-07:00 exists device name:test0 volume:0 minor:0 disk:UpToDate client:no size:4056 read:1340 written:16 al-writes:1 bm-writes:0 upper-pending:0 lower-pending:0 al-suspended:no blocked:no
 * 2017-02-15T14:43:16.688437+00:00 exists connection name:test0 conn-name:peer connection:Connected role:Secondary congested:no
 * 2017-02-15T14:43:16.688437+00:00 exists peer-device name:test0 conn-name:peer volume:0 replication:SyncSource peer-disk:Inconsistent resync-suspended:no received:0 sent:2050743348 out-of-sync:205655500 pending:0 unacked:0
 */

// Spot checks for ByRes.Update, most of the logic is delegated to functions
// with their own tests, so we don't have to be too clinical here.
func TestByRes(t *testing.T) {
	evt, err := resource.NewEvent("2017-02-15T14:43:16.688437+00:00 exists connection " +
		"name:test0 conn-name:peer connection:Connected role:Secondary congested:no")
	if err != nil {
		t.Fatal(err)
	}
	br := NewByRes()
	br.Update(evt)

	if br.Connections["peer"].Role != "Secondary" {
		t.Errorf("TestByRes: Expected peer's role to be %q got %q", "Secondary", br.Connections["peer"].Role)
	}

	evt, err = resource.NewEvent("2017-02-15T14:43:16.688437+00:00 exists " +
		"peer-device name:test0 conn-name:peer volume:0 replication:SyncSource " +
		"	peer-disk:Inconsistent resync-suspended:no received:0 sent:2050743348 " +
		"out-of-sync:205655500 pending:0 unacked:0")
	if err != nil {
		t.Fatal(err)
	}
	br.Update(evt)

	if br.PeerDevices["peer"].Resource != "test0" {
		t.Errorf("TestByRes: Expected peerdevices' resource to be %q got %q", "test0", br.PeerDevices["peer"].Resource)
	}

	evt, err = resource.NewEvent("2017-03-27T08:28:17.072611-07:00 exists device " +
		"name:test0 volume:0 minor:0 disk:UpToDate client:no size:4056 read:1340 " +
		"	written:16 al-writes:1 bm-writes:0 upper-pending:0 lower-pending:0 al-suspended:no blocked:no")
	if err != nil {
		t.Fatal(err)
	}
	br.Update(evt)

	if br.Device.Volumes["0"].DiskState != "UpToDate" {
		t.Errorf("TestByRes: Expected devices volume 0's disk state to be %q got %q", "UpToDate", br.Device.Volumes["0"].DiskState)
	}

	br.prune(time.Now())

	if _, ok := br.Connections["peer"]; ok {
		t.Error("TestByRes: Expected byres's connection to peer to be pruned")
	}
}

// Spot checks for ResourceCollections.
func TestResourceCollection(t *testing.T) {

	evt, err := resource.NewEvent("8000-02-15T14:44:16.688437+00:00 exists resource name:test10 " +
		"role:Primary suspended:no write-ordering:flush")
	if err != nil {
		t.Fatal(err)
	}

	rc := NewResourceCollection(0) // Turn off pruning with zero.
	rc.Update(evt)

	if _, ok := rc.Map["test10"]; !ok {
		t.Error("TestResourceCollection: Expected test10 to exist")
	}

	evt, err = resource.NewEvent("8000-02-15T14:44:16.688437+00:00 exists resource name:test100 " +
		"role:Primary suspended:no write-ordering:flush")
	if err != nil {
		t.Fatal(err)
	}
	rc.Update(evt)

	if rc.List[1].Res.Name != "test100" {
		t.Errorf("TestResourceCollection: Expected test100 to be sorted last. Got %s", rc.List[1].Res.Name)
	}
}

func TestName(t *testing.T) {
	var nameTests = []struct {
		n1  *ByRes
		n2  *ByRes
		out bool
	}{
		{&ByRes{Res: &resource.Resource{Name: "z2a"}}, &ByRes{Res: &resource.Resource{Name: "z1a"}}, false},
		{&ByRes{Res: &resource.Resource{Name: "test10"}}, &ByRes{Res: &resource.Resource{Name: "test100"}}, true},
		{NewByRes(), &ByRes{Res: &resource.Resource{Name: "test100"}}, false},
	}
	for _, tt := range nameTests {
		b := Name(tt.n1, tt.n2)
		if b != tt.out {
			t.Errorf("Expected %q < %q to be %v", tt.n1.Res.Name, tt.n2.Res.Name, tt.out)
		}

		// Check the reverse sort too.
		b = NameReverse(tt.n1, tt.n2)
		if !b != tt.out {
			t.Errorf("Expected %q < %q to be %v", tt.n1.Res.Name, tt.n2.Res.Name, tt.out)
		}
	}
}
