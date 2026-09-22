package output

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Status is a stderr spinner for steps without a known total. On a terminal it
// animates; otherwise it prints the message once.
type Status struct {
	live bool
	stop chan struct{}
	wait chan struct{}
	once sync.Once
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Start begins a status spinner, or prints the message once when not a terminal.
func Start(msg string) *Status {
	s := &Status{stop: make(chan struct{}), wait: make(chan struct{})}
	if Quiet() {
		return s
	}
	if !Interactive() {
		fmt.Fprintf(os.Stderr, "dali: %s\n", msg)
		return s
	}
	s.live = true
	go s.loop(msg)
	return s
}

// Stop ends the spinner and clears its line.
func (s *Status) Stop() {
	if s.live {
		s.once.Do(func() {
			close(s.stop)
			<-s.wait
		})
	}
}

func (s *Status) loop(msg string) {
	defer close(s.wait)
	ticker := time.NewTicker(90 * time.Millisecond)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-s.stop:
			fmt.Fprint(os.Stderr, "\r\x1b[K")
			return
		case <-ticker.C:
			fmt.Fprintf(os.Stderr, "\r\x1b[K%s %s", Label(spinnerFrames[i%len(spinnerFrames)]), msg)
			i++
		}
	}
}
