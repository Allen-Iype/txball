package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/Allen-Iype/txball/internal/game"
	"github.com/Allen-Iype/txball/internal/level"
	"github.com/Allen-Iype/txball/internal/render"
	"github.com/Allen-Iype/txball/levels"
)

func newSim(t *testing.T) tcell.SimulationScreen {
	t.Helper()
	s := tcell.NewSimulationScreen("")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(render.MinCols, render.MinRows)
	t.Cleanup(s.Fini)
	return s
}

func runLoop(s tcell.Screen, ctx context.Context) <-chan error {
	errc := make(chan error, 1)
	grids, err := level.LoadAll(levels.FS)
	if err != nil {
		panic(err)
	}
	go func() { errc <- loop(ctx, s, game.New(grids, nil), Options{Debug: true}) }()
	return errc
}

func TestLoopQuitKeys(t *testing.T) {
	tests := []struct {
		name string
		key  tcell.Key
		r    rune
	}{
		{"q", tcell.KeyRune, 'q'},
		{"Q", tcell.KeyRune, 'Q'},
		{"esc", tcell.KeyEscape, 0},
		{"ctrl-c", tcell.KeyCtrlC, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSim(t)
			errc := runLoop(s, context.Background())
			s.InjectKey(tt.key, tt.r, tcell.ModNone)
			select {
			case err := <-errc:
				if err != nil {
					t.Fatalf("loop: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("loop did not exit")
			}
		})
	}
}

func TestLoopDrawsAndStopsOnCancel(t *testing.T) {
	s := newSim(t)
	ctx, cancel := context.WithCancel(context.Background())
	errc := runLoop(s, ctx)

	time.Sleep(150 * time.Millisecond) // a few frames
	cancel()
	select {
	case <-errc:
	case <-time.After(2 * time.Second):
		t.Fatal("loop did not exit on cancel")
	}

	cells, w, _ := s.GetContents()
	var top strings.Builder
	for _, c := range cells[:w] {
		if len(c.Runes) > 0 {
			top.WriteRune(c.Runes[0])
		}
	}
	if !strings.Contains(top.String(), "TX-Ball") {
		t.Errorf("HUD row = %q, want title", top.String())
	}
	if got := cells[w].Runes; len(got) == 0 || got[0] != '┌' {
		t.Errorf("border corner = %q, want ┌", string(got))
	}
}

// paddleCols reports which columns of the paddle's screen row are lit.
func paddleCols(s tcell.SimulationScreen) (first, last int) {
	cells, w, _ := s.GetContents()
	_, oy := render.Origin(s.Size())
	row := cells[(oy+20)*w:][:w] // PaddleTop=40 is cell row 20
	// The row below the paddle is always empty, so it gives the background.
	bg, _, _ := cells[(oy+21)*w+1].Style.Decompose()
	first, last = -1, -1
	for x := 1; x < w-1; x++ {
		if fg, _, _ := row[x].Style.Decompose(); fg != bg {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	return first, last
}

func TestLoopSteersPaddle(t *testing.T) {
	tests := []struct {
		name      string
		inject    func(s tcell.SimulationScreen)
		wantFirst int
		wantLast  int
	}{
		// Field origin is column 1; the paddle is 12 wide and starts centred
		// on world x=40, i.e. columns 35..46.
		{"untouched", func(tcell.SimulationScreen) {}, 35, 46},
		{"mouse", func(s tcell.SimulationScreen) { s.InjectMouse(21, 5, tcell.ButtonNone, tcell.ModNone) }, 16, 27},
		{"mouse past left wall clamps", func(s tcell.SimulationScreen) { s.InjectMouse(0, 5, tcell.ButtonNone, tcell.ModNone) }, 1, 12},
		{"mouse past right wall clamps", func(s tcell.SimulationScreen) { s.InjectMouse(81, 5, tcell.ButtonNone, tcell.ModNone) }, 69, 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSim(t)
			ctx, cancel := context.WithCancel(context.Background())
			errc := runLoop(s, ctx)
			tt.inject(s)
			time.Sleep(150 * time.Millisecond)
			cancel()
			<-errc
			first, last := paddleCols(s)
			if first != tt.wantFirst || last != tt.wantLast {
				t.Errorf("paddle columns = %d..%d, want %d..%d", first, last, tt.wantFirst, tt.wantLast)
			}
		})
	}
}

func TestLoopKeyboardMovesPaddle(t *testing.T) {
	tests := []struct {
		name  string
		key   tcell.Key
		r     rune
		right bool
	}{
		{"left arrow", tcell.KeyLeft, 0, false},
		{"right arrow", tcell.KeyRight, 0, true},
		{"a", tcell.KeyRune, 'a', false},
		{"d", tcell.KeyRune, 'd', true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSim(t)
			ctx, cancel := context.WithCancel(context.Background())
			errc := runLoop(s, ctx)
			s.InjectKey(tt.key, tt.r, tcell.ModNone)
			time.Sleep(300 * time.Millisecond)
			cancel()
			<-errc
			first, _ := paddleCols(s)
			if moved := first > 35; first == 35 || moved != tt.right {
				t.Errorf("paddle starts at column %d, want it moved %s of 35", first, map[bool]string{true: "right", false: "left"}[tt.right])
			}
		})
	}
}

// screenText returns the simulated screen as one string per row.
func screenText(s tcell.SimulationScreen) []string {
	cells, w, h := s.GetContents()
	out := make([]string, h)
	for y := range h {
		var b strings.Builder
		for _, c := range cells[y*w:][:w] {
			if len(c.Runes) > 0 {
				b.WriteRune(c.Runes[0])
			} else {
				b.WriteByte(' ')
			}
		}
		out[y] = b.String()
	}
	return out
}

func TestLoopGameFlowKeys(t *testing.T) {
	space := func(s tcell.SimulationScreen) { s.InjectKey(tcell.KeyRune, ' ', tcell.ModNone) }
	click := func(s tcell.SimulationScreen) {
		s.InjectMouse(40, 10, tcell.ButtonPrimary, tcell.ModNone)
		s.InjectMouse(40, 10, tcell.ButtonNone, tcell.ModNone)
	}
	pause := func(s tcell.SimulationScreen) { s.InjectKey(tcell.KeyRune, 'p', tcell.ModNone) }

	tests := []struct {
		name    string
		steps   []func(tcell.SimulationScreen)
		want    string // text expected on the overlay row; "" for none
		wantNot string
	}{
		{"starts waiting to launch", nil, "click or space to launch", ""},
		{"space launches", []func(tcell.SimulationScreen){space}, "", "launch"},
		{"click launches", []func(tcell.SimulationScreen){click}, "", "launch"},
		{"p pauses", []func(tcell.SimulationScreen){space, pause}, "PAUSED", ""},
		{"p again resumes", []func(tcell.SimulationScreen){space, pause, pause}, "", "PAUSED"},
		{"pause while serving", []func(tcell.SimulationScreen){pause}, "PAUSED", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSim(t)
			ctx, cancel := context.WithCancel(context.Background())
			errc := runLoop(s, ctx)
			for _, step := range tt.steps {
				step(s)
				time.Sleep(80 * time.Millisecond)
			}
			time.Sleep(80 * time.Millisecond)
			cancel()
			<-errc

			_, oy := render.Origin(s.Size())
			row := screenText(s)[oy+17]
			if tt.want != "" && !strings.Contains(row, tt.want) {
				t.Errorf("overlay row = %q, want it to contain %q", row, tt.want)
			}
			if tt.wantNot != "" && strings.Contains(row, tt.wantNot) {
				t.Errorf("overlay row = %q, want no %q", row, tt.wantNot)
			}
			if testing.Verbose() && tt.name == "p pauses" {
				t.Log("\n" + strings.Join(screenText(s), "\n"))
			}
		})
	}
}

// start runs the loop on g and returns a function that stops it and
// reports its error.
func start(t *testing.T, s tcell.Screen, g *game.Game, opts Options) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- loop(ctx, s, g, opts) }()
	return func() error {
		cancel()
		select {
		case err := <-errc:
			return err
		case <-time.After(2 * time.Second):
			t.Fatal("loop did not stop")
			return nil
		}
	}
}

func builtinGame(t *testing.T) *game.Game {
	t.Helper()
	grids, err := level.LoadAll(levels.FS)
	if err != nil {
		t.Fatal(err)
	}
	return game.New(grids, nil)
}

func TestLoopResizeRecentres(t *testing.T) {
	s := newSim(t)
	stop := start(t, s, builtinGame(t), Options{})
	time.Sleep(100 * time.Millisecond)

	s.SetSize(render.MinCols+20, render.MinRows+10)
	time.Sleep(150 * time.Millisecond)
	if err := stop(); err != nil {
		t.Fatal(err)
	}

	rows := screenText(s)
	// Centred: 10 spare columns and 5 spare rows on each side.
	if got := []rune(rows[6]); got[10] != '┌' || got[10+render.MinCols-1] != '┐' {
		t.Errorf("top border not at its centred position: %q", rows[6])
	}
	if !strings.Contains(rows[5], "TX-Ball") {
		t.Errorf("HUD not on the row above the border: %q", rows[5])
	}
	for y, row := range rows[:5] {
		if strings.TrimSpace(row) != "" {
			t.Errorf("stale content left on row %d after resize: %q", y, row)
		}
	}
}

func TestLoopTooSmallPausesAndRecovers(t *testing.T) {
	s := newSim(t)
	g := builtinGame(t)
	stop := start(t, s, g, Options{})
	s.InjectKey(tcell.KeyRune, ' ', tcell.ModNone) // launch
	time.Sleep(120 * time.Millisecond)

	s.SetSize(60, 20)
	time.Sleep(150 * time.Millisecond)
	small := strings.Join(screenText(s), "\n")
	for _, want := range []string{"Terminal too small", "need 82×25, have 60×20"} {
		if !strings.Contains(small, want) {
			t.Errorf("too-small screen is missing %q:\n%s", want, small)
		}
	}
	if strings.ContainsRune(small, '▀') {
		t.Error("playfield still drawn on a too-small screen")
	}

	s.SetSize(render.MinCols, render.MinRows)
	time.Sleep(150 * time.Millisecond)
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if g.State != game.StatePaused {
		t.Errorf("state after shrinking and regrowing = %v, want Paused", g.State)
	}
	big := strings.Join(screenText(s), "\n")
	if !strings.Contains(big, "PAUSED") || strings.Contains(big, "too small") {
		t.Errorf("after regrowing, want the game back with PAUSED showing:\n%s", big)
	}
}

func TestLoopQuitWorksWhenTooSmall(t *testing.T) {
	s := newSim(t)
	s.SetSize(30, 10)
	stop := start(t, s, builtinGame(t), Options{})
	time.Sleep(80 * time.Millisecond)
	s.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	time.Sleep(80 * time.Millisecond)
	if err := stop(); err != nil {
		t.Fatal(err)
	}
}

func TestLoopSavesHighScore(t *testing.T) {
	readScore := func(t *testing.T, path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	tests := []struct {
		name  string
		high  int // loaded high score
		score int // score reached this session
		want  string
	}{
		{"new record is saved on quit", 100, 250, "250\n"},
		{"first ever score is saved", 0, 30, "30\n"},
		{"no record, nothing written", 100, 40, ""},
		{"no points, nothing written", 0, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cfg", "highscore")
			g := builtinGame(t)
			g.High, g.Score = tt.high, tt.score
			s := newSim(t)
			stop := start(t, s, g, Options{ScoreFile: path})
			time.Sleep(100 * time.Millisecond)
			if err := stop(); err != nil {
				t.Fatal(err)
			}
			if got := readScore(t, path); got != tt.want {
				t.Errorf("score file = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("saved at game over without quitting", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "highscore")
		g := builtinGame(t)
		g.Score, g.Lives = 70, 1
		s := newSim(t)
		stop := start(t, s, g, Options{ScoreFile: path})
		defer stop()

		s.InjectMouse(1, 5, tcell.ButtonNone, tcell.ModNone) // park the paddle at the left wall
		s.InjectKey(tcell.KeyRune, ' ', tcell.ModNone)       // launch; the ball will come down elsewhere
		deadline := time.Now().Add(10 * time.Second)
		for readScore(t, path) == "" {
			if time.Now().After(deadline) {
				t.Fatal("high score not written after the game ended")
			}
			time.Sleep(50 * time.Millisecond)
		}
		if got := readScore(t, path); !strings.HasSuffix(got, "0\n") || got == "0\n" {
			t.Errorf("score file = %q, want the final score", got)
		}
	})

	t.Run("unwritable location is reported", func(t *testing.T) {
		blocker := filepath.Join(t.TempDir(), "blocker")
		if err := os.WriteFile(blocker, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		g := builtinGame(t)
		g.Score = 10
		s := newSim(t)
		stop := start(t, s, g, Options{ScoreFile: filepath.Join(blocker, "highscore")})
		time.Sleep(80 * time.Millisecond)
		if err := stop(); err == nil || !strings.Contains(err.Error(), "save high score") {
			t.Errorf("err = %v, want a save failure", err)
		}
	})
}
