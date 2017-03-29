package convert

import "testing"

func TestKiB2Human(t *testing.T) {
	var conversionTests = []struct {
		in  float64
		out string
	}{
		{float64(1024), "1.0MiB"},
		{float64(1048576), "1.0GiB"},
		{float64(1073741824), "1.0TiB"},
		{float64(1099511627776), "1.0PiB"},
		{float64(1125899906842624), "1.0EiB"},
		{float64(123456), "120.6MiB"},
		{float64(-1024), "-1.0MiB"},
		{float64(-1048576), "-1.0GiB"},
		{float64(0), "0.0KiB"},
	}

	for _, tt := range conversionTests {
		s := KiB2Human(tt.in)
		if s != tt.out {
			t.Errorf("Expected %f kilobytes to convert to %s, got %s", tt.in, tt.out, s)
		}
	}
}
