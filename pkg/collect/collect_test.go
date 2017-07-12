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

package collect

import (
	"reflect"
	"testing"
)

func TestDoAllResources(t *testing.T) {
	in := "drbdsetup connect .drbdctrl 1\ndrbdsetup connect r0 0\ndrbdsetup connect r1 1\ndrbdsetup connect r2 1\ndrbdsetup connect r3 1\ndrbdsetup connect r4 1\ndrbdsetup connect r5 1\ndrbdsetup connect r6 1\ndrbdsetup connect r7 1\n"

	expected := map[string]bool{
		".drbdctrl": true,
		"r0":        true,
		"r1":        true,
		"r2":        true,
		"r3":        true,
		"r4":        true,
		"r5":        true,
		"r6":        true,
		"r7":        true,
	}

	out, err := doAllResources(in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, expected) {
		t.Errorf("Expected: %v Got: %v", expected, out)
	}

}
