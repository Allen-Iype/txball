// Package app owns the terminal: screen lifecycle, input translation and the
// fixed-timestep loop that drives game and render.
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/Allen-Iype/txball/internal/game"
	"github.com/Allen-Iype/txball/internal/render"
)

const (
	updateHz = 120
	renderHz = 60

	dt = 1.0 / updateHz
	// maxFrame caps how much simulated time one frame may catch up on, so a
	// suspended process doesn't fast-forward when it resumes.
	maxFrame = 0.25

	// keyHold is how long one key event keeps the paddle moving. Terminals
	// report key repeats but not releases, so each repeat extends the move.
	keyHold = 120 * time.Millisecond
)

// Options configure a run.
type Options struct {
	// Debug shows FPS/UPS counters.
	Debug bool
	// PanicAfter, if non-zero, panics after that long. It exists to check
	// that the terminal is restored when the game crashes.
	PanicAfter time.Duration
}

// Run takes over the terminal and plays until the user quits.
func Run(opts Options) (err error) {
	s, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	if err := s.Init(); err != nil {
		return fmt.Errorf("init terminal: %w", err)
	}
	// Restore the terminal before anything is printed, including a panic
	// trace. Re-panicking from the deferred call keeps the original frames.
	defer func() {
		r := recover()
		s.Fini()
		if r != nil {
			panic(r)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	return loop(ctx, s, opts)
}

// loop runs the game on an initialised screen until quit or ctx is done.
// All game state is touched from this goroutine only.
func loop(ctx context.Context, s tcell.Screen, opts Options) error {
	s.HideCursor()
	s.EnableMouse(tcell.MouseMotionEvents)

	done := make(chan struct{})
	defer close(done)
	events := make(chan tcell.Event, 64)
	go func() {
		for {
			ev := s.PollEvent()
			if ev == nil { // screen finalised
				return
			}
			select {
			case events <- ev:
			case <-done:
				return
			}
		}
	}()

	var (
		g     = game.New()
		r     = render.New()
		in    game.Input
		stats = render.Stats{Show: opts.Debug}

		start        = time.Now()
		last         = start
		acc          float64
		frames, upds int
		statsAt      = start

		keyDir   float64
		keyUntil time.Time
	)

	ticker := time.NewTicker(time.Second / renderHz)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case ev := <-events:
			switch ev := ev.(type) {
			case *tcell.EventKey:
				if isQuit(ev) {
					return nil
				}
				if dir := steer(ev); dir != 0 {
					keyDir, keyUntil = dir, time.Now().Add(keyHold)
				}
			case *tcell.EventMouse:
				col, _ := ev.Position()
				sw, sh := s.Size()
				in.PaddleX, in.HasMouse = render.WorldX(sw, sh, col), true
			case *tcell.EventResize:
				// The playfield re-centres, so drop what was drawn at
				// the old origin.
				s.Clear()
				s.Sync()
			}

		case <-ticker.C:
			now := time.Now()
			if opts.PanicAfter > 0 && now.Sub(start) >= opts.PanicAfter {
				panic("txball: deliberate panic (-panic-after)")
			}

			acc += min(now.Sub(last).Seconds(), maxFrame)
			last = now
			in.Move = 0
			if now.Before(keyUntil) {
				in.Move = keyDir
			}
			for acc >= dt {
				g.Update(dt, in)
				acc -= dt
				upds++
				// A mouse position is a one-shot jump; once applied, the
				// keyboard is free to move the paddle away from it.
				in.HasMouse = false
			}

			frames++
			if el := now.Sub(statsAt); el >= time.Second {
				stats.FPS = int(float64(frames)/el.Seconds() + 0.5)
				stats.UPS = int(float64(upds)/el.Seconds() + 0.5)
				frames, upds, statsAt = 0, 0, now
			}

			r.Draw(s, g, stats)
			s.Show()
		}
	}
}

func isQuit(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return true
	case tcell.KeyRune:
		return ev.Rune() == 'q' || ev.Rune() == 'Q'
	}
	return false
}

// steer maps a key to a paddle direction: -1 left, +1 right, 0 neither.
func steer(ev *tcell.EventKey) float64 {
	switch ev.Key() {
	case tcell.KeyLeft:
		return -1
	case tcell.KeyRight:
		return 1
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'a', 'A':
			return -1
		case 'd', 'D':
			return 1
		}
	}
	return 0
}
