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
