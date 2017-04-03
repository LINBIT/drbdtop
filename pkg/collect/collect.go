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
	Collect(events chan<- resource.Event, errors chan<- error)
}

// FileCollector gathers newline delimited events from a plaintext file.
type FileCollector struct {
	Path *string
}

func (c FileCollector) Collect(events chan<- resource.Event, errors chan<- error) {
	f, err := os.Open(*c.Path)
	if err != nil {
		errors <- err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		e := scanner.Text()
		evt, err := resource.NewEvent(e)
		if err != nil {
			errors <- err
		} else {
			events <- evt
		}
	}
	events <- resource.NewEOF()
}

// Events2Poll continuously calls drbdsetup events2 at a specified Interval.
type Events2Poll struct {
	// Interval to wait between calls to drbdsetup devents2
	Interval time.Duration
}

func (c Events2Poll) Collect(events chan<- resource.Event, errors chan<- error) {
	ticker := time.NewTicker(c.Interval)
	for {
		out, err := exec.Command("drbdsetup", "events2", "--timestamps", "--statistics", "--now").CombinedOutput()
		if err != nil {
			errors <- err
			events <- resource.NewEOF()
		} else {
			s := string(out)
			for _, e := range strings.Split(s, "\n") {
				if e != "" {
					evt, err := resource.NewEvent(e)
					if err != nil {
						errors <- err
					} else {
						events <- evt
					}
				}
			}
		}
		<-ticker.C
	}
}
