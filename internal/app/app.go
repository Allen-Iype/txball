// Package app owns the terminal: screen lifecycle, input translation and the
// fixed-timestep loop that drives game and render.
package app

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/Allen-Iype/txball/internal/game"
	"github.com/Allen-Iype/txball/internal/level"
	"github.com/Allen-Iype/txball/internal/render"
	"github.com/Allen-Iype/txball/internal/score"
	"github.com/Allen-Iype/txball/levels"
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

	// bellGap is the least time between bells, so a ball rattling between
	// bricks doesn't turn into a buzz.
	bellGap = 70 * time.Millisecond
)

// Options configure a run.
type Options struct {
	// Debug shows FPS/UPS counters and lets keys 1-7 drop a power-up.
	Debug bool
	// Level is the 1-based level to start on; 0 means the first.
	Level int
	// Bell rings the terminal bell when a ball hits something.
	Bell bool
	// ScoreFile is where the high score is saved; empty means don't save.
	// Run fills it in when it is empty.
	ScoreFile string
	// PanicAfter, if non-zero, panics after that long. It exists to check
	// that the terminal is restored when the game crashes.
	PanicAfter time.Duration
}

// Run takes over the terminal and plays until the user quits.
func Run(opts Options) (err error) {
	// Load levels before touching the terminal, so a bad level file is
	// reported on a normal screen.
	grids, err := level.LoadAll(levels.FS)
	if err != nil {
		return fmt.Errorf("load levels: %w", err)
	}
	g := game.New(grids, rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
	if opts.Level > 1 {
		g.JumpTo(opts.Level)
	}

	if opts.ScoreFile == "" {
		if opts.ScoreFile, err = score.Path(); err != nil {
			return err
		}
	}
	if g.High, err = score.Load(opts.ScoreFile); err != nil {
		// Not worth refusing to play over. This lands on the normal
		// screen, where it is still there after the game exits.
		fmt.Fprintf(os.Stderr, "txball: ignoring unreadable high score: %v\n", err)
	}

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

	return loop(ctx, s, g, opts)
}

// loop runs g on an initialised screen until quit or ctx is done. All game
// state is touched from this goroutine only.
func loop(ctx context.Context, s tcell.Screen, g *game.Game, opts Options) (err error) {
	s.HideCursor()
	s.EnableMouse(tcell.MouseMotionEvents)

	// The high score is written when a game ends and again on the way
	// out, so quitting mid-game keeps a new record too.
	saved := g.High
	saveHigh := func() error {
		if opts.ScoreFile == "" || g.High <= saved {
			return nil
		}
		if err := score.Save(opts.ScoreFile, g.High); err != nil {
			return fmt.Errorf("save high score: %w", err)
		}
		saved = g.High
		return nil
	}
	defer func() {
		if e := saveHigh(); err == nil {
			err = e
		}
	}()

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
		// mouseDown tracks the primary button so a held click launches once.
		mouseDown bool
		// fits is whether the terminal was big enough last frame.
		fits     = true
		lastBell time.Time
		// screenW and screenH are the size the last frame was drawn at.
		screenW, screenH = s.Size()
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
				if ev.Key() == tcell.KeyRune {
					switch ev.Rune() {
					case ' ':
						in.Launch = true
					case 'p', 'P':
						in.Pause = true
					}
					if opts.Debug {
						dropPower(g, ev.Rune())
					}
				}
			case *tcell.EventMouse:
				col, _ := ev.Position()
				sw, sh := s.Size()
				in.PaddleX, in.HasMouse = render.WorldX(sw, sh, col), true
				down := ev.Buttons()&tcell.ButtonPrimary != 0
				if down && !mouseDown {
					in.Launch = true
				}
				mouseDown = down
			case *tcell.EventResize:
				s.Sync()
			}

		case <-ticker.C:
			now := time.Now()
			if opts.PanicAfter > 0 && now.Sub(start) >= opts.PanicAfter {
				panic("txball: deliberate panic (-panic-after)")
			}

			// Frames only write the cells they own, so when the size
			// changes (and the playfield re-centres) drop what was
			// drawn for the old layout.
			if w, h := s.Size(); w != screenW || h != screenH {
				screenW, screenH = w, h
				s.Clear()
			}

			// Too small to draw: pause, and hold the simulation still
			// until the window grows again.
			wasFitting := fits
			if fits = render.Fits(screenW, screenH); !fits {
				if wasFitting {
					g.Pause()
				}
				in.HasMouse, in.Launch, in.Pause = false, false, false
				last, acc = now, 0
				render.DrawTooSmall(s)
				s.Show()
				continue
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
				// These are one-shot: a mouse position is a jump, after
				// which the keyboard is free to move the paddle away, and
				// button presses are edges.
				in.HasMouse, in.Launch, in.Pause = false, false, false
			}

			frames++
			if el := now.Sub(statsAt); el >= time.Second {
				stats.FPS = int(float64(frames)/el.Seconds() + 0.5)
				stats.UPS = int(float64(upds)/el.Seconds() + 0.5)
				frames, upds, statsAt = 0, 0, now
			}

			const audible = game.EventBrick | game.EventPaddle | game.EventLifeLost
			if opts.Bell && g.Events&audible != 0 && now.Sub(lastBell) >= bellGap {
				s.Beep()
				lastBell = now
			}
			g.Events = 0

			if g.State == game.StateGameOver {
				if err := saveHigh(); err != nil {
					return err
				}
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

// dropPower is a debugging aid: keys 1-7 release the corresponding
// power-up capsule from the middle of the field.
func dropPower(g *game.Game, key rune) {
	k := int(key - '1')
	if k < 0 || k >= game.NumPowers || g.State != game.StatePlaying {
		return
	}
	g.Powerups = append(g.Powerups, game.Powerup{
		Kind: game.PowerKind(k),
		Pos:  game.Vec{X: g.Paddle.X, Y: game.Height / 2},
	})
}
