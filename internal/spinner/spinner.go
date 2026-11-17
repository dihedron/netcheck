package spinner

import (
	"context"
	"fmt"
	"time"
)

const (
	DefaultSpeed    = 100 * time.Millisecond
	DefaultSequence = Sequence7

	Sequence1  = "▁▂▃▄▅▆▇█▇▆▅▄▃▂"
	Sequence2  = "▁▂▃▄▅▆▇█"
	Sequence3  = "█▉▊▋▌▍▍▎▏"
	Sequence4  = "◇◈◆◈"
	Sequence5  = "◇□"
	Sequence6  = "◐◓◑◒"
	Sequence7  = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
	Sequence8  = "▲▴▶▸▼▾◀◂"
	Sequence9  = "◜◠◝◞◡◟"
	Sequence10 = "◇◊|◊"
	Sequence11 = "◦○◎◯◎○"
	Sequence12 = "←↖↑↗→↘↓↙"
	Sequence13 = "▖▘▝▗"
	Sequence14 = "◰◳◲◱"
	Sequence15 = "◴◷◶◵"
	Sequence16 = "⣾⣽⣻⢿⡿⣟⣯⣷"

	//P5                        = "◇□▣◈◆■◈"
)

type Spinner struct {
	sequence []rune
	speed    time.Duration
	cancel   context.CancelFunc
	done     chan struct{}
}

// Option is the type for functional options.
type Option func(*Spinner)

// New creates a new Spinner, applying all the provided functional options.
func New(options ...Option) *Spinner {
	s := &Spinner{
		speed:    DefaultSpeed,
		sequence: []rune(DefaultSequence),
	}
	for _, option := range options {
		option(s)
	}
	return s
}

// WithSequence sets the current sequence.
func WithSequence(sequence string) Option {
	return func(s *Spinner) {
		if s != nil {
			s.sequence = []rune(sequence)
		}
	}
}

func WithSpeed(speed time.Duration) Option {
	return func(s *Spinner) {
		if s != nil {
			s.speed = speed
		}
	}
}

func (s *Spinner) Start() {
	s.StartContext(context.Background())
}

func (s *Spinner) StartContext(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	ctx, s.cancel = context.WithCancel(ctx)
	s.done = make(chan struct{})
	go start(ctx, ticker, s.sequence, s.done)
}

func (s *Spinner) Stop() {
	if s.cancel != nil {
		s.cancel()
		<-s.done
	}
}

func start(ctx context.Context, ticker *time.Ticker, sequence []rune, done chan<- struct{}) {

	// hides the cursor
	fmt.Printf("\033[?25l")

	// restore the cursor upon exit
	defer func() {
		fmt.Printf(" \b\033[?25h")
		done <- struct{}{}
	}()

	current := 0
	for {
		select {
		case <-ticker.C: // tick!
			fmt.Printf("%c\b", sequence[current])
			current = (current + 1) % len(sequence)
		case <-ctx.Done(): // context has been cancelled
			ticker.Stop()
			return
		}
	}
}
