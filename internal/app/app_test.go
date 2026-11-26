package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/Allen-Iype/txball/internal/render"
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
	go func() { errc <- loop(ctx, s, Options{Debug: true}) }()
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
