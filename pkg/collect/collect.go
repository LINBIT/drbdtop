package collect

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"time"

	"drbdtop.io/drbdtop/pkg/resource"
)

// Collector sends raw event strings into a channel.
type Collector interface {
	Collect(rawEvents chan<- string, errors chan<- error)
}

// FileCollector gathers newline delimited events from a plaintext file.
type FileCollector struct {
	Path *string
}

func (c FileCollector) Collect(rawEvents chan<- string, errors chan<- error) {
	f, err := os.Open(*c.Path)
	if err != nil {
		errors <- err
	}
	defer f.Close()
	defer close(rawEvents)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		rawEvents <- scanner.Text()
	}
	rawEvents <- resource.EOF
}

// Events2Poll continuously calls drbdsetup events2 at a specified Interval.
type Events2Poll struct {
	// Interval to wait between calls to drbdsetup devents2
	Interval time.Duration
}

func (c Events2Poll) Collect(rawEvents chan<- string, errors chan<- error) {
	for {
		out, err := exec.Command("drbdsetup", "events2", "--timestamps", "--statistics", "--now").CombinedOutput()
		if err != nil {
			errors <- err
			rawEvents <- resource.EOF
		} else {
			s := string(out)
			for _, rawEvent := range strings.Split(s, "\n") {
				rawEvents <- rawEvent
			}
		}
		time.Sleep(c.Interval)
	}
}
