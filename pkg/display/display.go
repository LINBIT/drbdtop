package display

import "github.com/hayswim/drbdtop/pkg/resource"

// Displayer provides information to the user via the screen, printing to a file,
// writing to the network, ect.
type Displayer interface {
	// The main loop for a displayer.
	Display(<-chan resource.Event, <-chan error)
}
