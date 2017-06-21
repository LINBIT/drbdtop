package display

import "github.com/linbit/drbdtop/pkg/resource"

// Displayer provides information to the user via the screen, printing to a file,
// writing to the network, ect.
type Displayer interface {
	// The main loop for a displayer. Program exits when this function returns.
	Display(<-chan resource.Event, <-chan error)
}
