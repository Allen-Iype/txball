package render

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/Allen-Iype/txball/internal/game"
)

// draw renders g on a minimum-size simulated screen and returns its rows.
func draw(t *testing.T, g *game.Game) []string {
	t.Helper()
	s := tcell.NewSimulationScreen("")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(MinCols, MinRows)
	New().Draw(s, g, Stats{})
	s.Show()

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

func newGame() *game.Game {
	var grid game.Grid
	grid[0][0] = game.Brick{Kind: game.BrickNormal, Hits: 1}
	return game.New([]game.Grid{grid}, nil)
}

// cellAt returns the rune at a playfield cell: the field starts at screen
// column 1, row 2 on a minimum-size screen.
func cellAt(rows []string, cx, cy int) rune {
	return []rune(rows[cy+2])[cx+1]
}

func TestCapsulesAreLetteredPills(t *testing.T) {
	tests := []struct {
		kind   game.PowerKind
		letter rune
	}{
		{game.PowerMulti, 'M'},
		{game.PowerLong, 'L'},
		{game.PowerShort, 'S'},
		{game.PowerFire, 'F'},
		{game.PowerSticky, 'G'},
		{game.PowerSlow, 'W'},
		{game.PowerLife, '+'},
	}
	for _, tt := range tests {
		g := newGame()
		g.Update(1.0/120, game.Input{Launch: true})
		// Centre x=41.5 covers pixels 40..42; y=21 is cell row 10.
		g.Powerups = []game.Powerup{{Kind: tt.kind, Pos: game.Vec{X: 41.5, Y: 21}}}
		rows := draw(t, g)
		got := string([]rune{cellAt(rows, 40, 10), cellAt(rows, 41, 10), cellAt(rows, 42, 10)})
		if want := " " + string(tt.letter) + " "; got != want {
			t.Errorf("kind %d: capsule cells = %q, want %q", tt.kind, got, want)
		}
		if l, r := cellAt(rows, 39, 10), cellAt(rows, 43, 10); l != '▀' || r != '▀' {
			t.Errorf("kind %d: capsule spills into neighbours %q %q", tt.kind, l, r)
		}
	}
}

func TestCapsuleClipsAtEdges(t *testing.T) {
	g := newGame()
	g.Powerups = []game.Powerup{
		{Kind: game.PowerFire, Pos: game.Vec{X: 0.2, Y: 21}},
		{Kind: game.PowerLong, Pos: game.Vec{X: 79.8, Y: 21}},
		{Kind: game.PowerSlow, Pos: game.Vec{X: 40, Y: game.Height + 0.5}},
		{Kind: game.PowerSlow, Pos: game.Vec{X: 40, Y: -3}},
	}
	rows := draw(t, g) // must not panic
	if got := cellAt(rows, 0, 10); got != 'F' {
		t.Errorf("left-edge capsule shows %q at the wall, want its letter", got)
	}
	if got := cellAt(rows, 79, 10); got != 'L' {
		t.Errorf("right-edge capsule shows %q at the wall, want its letter", got)
	}
}

func TestHUD(t *testing.T) {
	g := newGame()
	g.Score, g.Lives, g.Level = 1230, 2, 7
	g.Timers[game.PowerFire] = 7.2
	g.Timers[game.PowerLong] = 19.01
	hud := draw(t, g)[0]

	g.High = 45670
	hud = draw(t, g)[0]
	if !strings.HasSuffix(hud, "♥♥  001230  HI 045670 ") { // the last column is the border's
		t.Errorf("HUD %q does not end with lives, score and high score", hud)
	}
	for _, want := range []string{"TX-Ball", "LEVEL 7", "♥♥  001230", "L20", "F8"} {
		if !strings.Contains(hud, want) {
			t.Errorf("HUD %q is missing %q", hud, want)
		}
	}
	if strings.Contains(hud, "♥♥♥") {
		t.Errorf("HUD %q shows too many lives", hud)
	}
	for _, absent := range []string{"M", "S", "G", "W"} {
		if strings.Contains(strings.TrimPrefix(strings.TrimSpace(hud), "TX-Ball"), absent+"0") {
			t.Errorf("HUD %q shows inactive effect %s", hud, absent)
		}
	}
}

func TestOverlays(t *testing.T) {
	const dt = 1.0 / 120
	tests := []struct {
		name string
		prep func(g *game.Game)
		want string
	}{
		{"serve", func(g *game.Game) {}, "click or space to launch"},
		{"playing", func(g *game.Game) { g.Update(dt, game.Input{Launch: true}) }, ""},
		{"paused", func(g *game.Game) { g.Update(dt, game.Input{Pause: true}) }, "PAUSED  p to resume"},
		{"level clear", func(g *game.Game) {
			g.Update(dt, game.Input{Launch: true})
			g.Bricks = game.Grid{}
			g.Update(dt, game.Input{})
		}, "LEVEL 1 CLEARED  click or space for the next level"},
		{"game over", func(g *game.Game) {
			g.Lives, g.Score = 1, 40
			g.Update(dt, game.Input{Launch: true})
			g.Balls[0].Pos.Y = game.Height + 5
			g.Update(dt, game.Input{})
		}, "GAME OVER  score 40 · click or space to play again"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newGame()
			tt.prep(g)
			row := strings.Trim(draw(t, g)[overlayRow+2], "│▀")
			if row != tt.want {
				t.Errorf("overlay = %q, want %q", row, tt.want)
			}
		})
	}
}

func TestFits(t *testing.T) {
	tests := []struct {
		w, h int
		want bool
	}{
		{MinCols, MinRows, true},
		{200, 60, true},
		{MinCols - 1, MinRows, false},
		{MinCols, MinRows - 1, false},
		{0, 0, false},
	}
	for _, tt := range tests {
		if got := Fits(tt.w, tt.h); got != tt.want {
			t.Errorf("Fits(%d, %d) = %v, want %v", tt.w, tt.h, got, tt.want)
		}
	}
}

func TestDrawTooSmall(t *testing.T) {
	for _, size := range [][2]int{{60, 20}, {40, 8}, {12, 3}, {1, 1}} {
		s := tcell.NewSimulationScreen("")
		if err := s.Init(); err != nil {
			t.Fatal(err)
		}
		s.SetSize(size[0], size[1])
		DrawTooSmall(s) // must not panic however small
		s.Show()

		cells, w, h := s.GetContents()
		var all strings.Builder
		for y := range h {
			for _, c := range cells[y*w:][:w] {
				if len(c.Runes) > 0 {
					all.WriteRune(c.Runes[0])
				}
			}
			all.WriteByte('\n')
		}
		s.Fini()
		if size[0] >= 40 && size[1] >= 8 {
			for _, want := range []string{"Terminal too small", "need 82×25, have " + itoa(size[0]) + "×" + itoa(size[1]), "q quit"} {
				if !strings.Contains(all.String(), want) {
					t.Errorf("%dx%d: notice is missing %q:\n%s", size[0], size[1], want, all.String())
				}
			}
		}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
