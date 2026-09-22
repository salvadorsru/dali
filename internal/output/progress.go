package output

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const barWidth = 28

// Progress is a stderr progress bar. On a terminal it redraws a bar; otherwise
// it prints periodic lines (or nothing when quiet).
type Progress struct {
	label    string
	total    int
	done     int
	reported int
	step     int
	tty      bool
	silent   bool
	start    time.Time
	last     time.Time
	finished bool

	mu sync.Mutex
}

func NewProgress(label string, total int) *Progress {
	step := total / 20
	if step < 1 {
		step = 1
	}
	return &Progress{
		label:  label,
		total:  total,
		step:   step,
		tty:    Interactive(),
		silent: Quiet(),
		start:  time.Now(),
	}
}

func (p *Progress) Add(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done += n
	p.render()
}

// Set reports an absolute count (for callbacks that track progress themselves).
func (p *Progress) Set(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done = n
	p.render()
}

func (p *Progress) Done() {
	if p.silent {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.finished {
		return
	}
	p.finished = true
	p.done = p.total
	p.render()
	if p.tty {
		fmt.Fprintln(os.Stderr)
	}
}

func (p *Progress) render() {
	if p.silent {
		return
	}
	if p.tty {
		now := time.Now()
		if now.Sub(p.last) < 80*time.Millisecond && p.done < p.total {
			return
		}
		p.last = now
		fmt.Fprintf(os.Stderr, "\r%s", p.line())
		return
	}
	if p.done-p.reported >= p.step || p.done >= p.total {
		fmt.Fprintf(os.Stderr, "%s: %d/%d\n", p.label, p.done, p.total)
		p.reported = p.done
	}
}

func (p *Progress) line() string {
	frac := 0.0
	if p.total > 0 {
		frac = float64(p.done) / float64(p.total)
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*barWidth + 0.5)
	bar := Success(strings.Repeat("█", filled)) + Dim(strings.Repeat("░", barWidth-filled))

	elapsed := time.Since(p.start).Seconds()
	rate := 0
	if elapsed > 0.05 {
		rate = int(float64(p.done) / elapsed)
	}
	eta := ""
	if rate > 0 && p.total > p.done {
		eta = "  " + time.Duration(float64(p.total-p.done)/float64(rate)*float64(time.Second)).Round(time.Second).String()
	}
	label := p.label
	if l := visibleLen(label); l < 12 {
		label += strings.Repeat(" ", 12-l)
	}
	return fmt.Sprintf("%s %s %3.0f%%  %s/%s  %s/s%s ",
		label, bar, frac*100, commas(p.done), commas(p.total), commas(rate), eta)
}

func commas(n int) string {
	s := fmt.Sprintf("%d", n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
