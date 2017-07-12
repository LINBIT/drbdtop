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

package convert

import (
	"fmt"
	"math"
)

// KiB2Human takes a size in KiB and returns a human readable size with suffix.
func KiB2Human(kiBytes float64) string {
	var sign string
	if kiBytes < 0 {
		kiBytes = -kiBytes
		sign = "-"
	}
	sizes := []string{"K", "M", "G", "T", "P", "E", "Z", "Y"}
	unit := float64(1024)

	// Too few kiBytes to meaningfully convert.
	if kiBytes < unit {
		return fmt.Sprintf("%s%.1f%siB", sign, kiBytes, sizes[0])
	}

	exp := int(math.Log(kiBytes) / math.Log(unit))
	return fmt.Sprintf("%s%.1f%siB", sign, (kiBytes / (math.Pow(unit, float64(exp)))), sizes[exp])
}
