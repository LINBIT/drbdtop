package collect

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/linbit/drbdtop/pkg/resource"
	"github.com/linbit/godrbdutils"
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
		remainingResources, err := allResources()
		if err != nil {
			errors <- err
		}
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
						delete(remainingResources, evt.Fields[resource.ResKeys.Name])
						events <- evt
					}
				}
			}
		}
		for res := range remainingResources {
			events <- resource.NewUnconfiguredRes(res)
		}
		<-ticker.C
	}
}

func allResources() (map[string]bool, error) {
	cmd, err := godrbdutils.NewDrbdCmd(godrbdutils.Drbdadm, godrbdutils.Connect, "all", "-d")
	if err != nil {
		return nil, fmt.Errorf("unable to find all reources: %v", err)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("unable to find all reources: %v", err)
	}
	return doAllResources(string(out))
}

func doAllResources(s string) (map[string]bool, error) {
	ret := map[string]bool{}
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		if len(f) == 4 {
			ret[f[2]] = true
		}
	}
	return ret, nil
}
