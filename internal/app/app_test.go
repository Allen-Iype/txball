package app

import (
	"context"
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
	go func() { errc <- loop(ctx, s, game.New(grids), Options{Debug: true}) }()
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
