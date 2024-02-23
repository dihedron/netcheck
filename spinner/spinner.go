package spinner

import (
	"context"
	"fmt"
	"time"
)

const (
	DefaultSpeed    = 100 * time.Millisecond
	DefaultSequence = RotatingTrianglesSequence

	RotatingTrianglesSequence = "▲▴▶▸▼▾◀◂"
	RotatingArcsSequence      = "◜◠◝◞◡◟"
	PulsatingLozengeSequence  = "◇◊|◊"
	PulsatingCirclesSequence  = "◦○◎◯◎○"
	P1                        = "▁▂▃▄▅▆▇█▇▆▅▄▃▂"
	P2                        = "▁▂▃▄▅▆▇█"
	P3                        = "█▉▊▋▌▍▍▎▏"
	P4                        = "◇◈◆◈"
	P5                        = "◇□"
	P6                        = "◴◷◶◵"
	Braille                   = " ⠇"
	//P5                        = "◇□▣◈◆■◈"
)

type Spinner struct {
	sequence []rune
	speed    time.Duration
	cancel   context.CancelFunc
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
	go start(ctx, ticker, s.sequence)
}

func (s *Spinner) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func start(ctx context.Context, ticker *time.Ticker, sequence []rune) {
	current := 0
	for {
		select {
		case <-ticker.C: // tick!
			fmt.Printf("\r\r\r%c  ", sequence[current])
			current = (current + 1) % len(sequence)
		case <-ctx.Done(): // context has been cancelled
			ticker.Stop()
			return
		}
	}
}
