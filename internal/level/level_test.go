package level

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Allen-Iype/txball/internal/game"
	"github.com/Allen-Iype/txball/levels"
)

func normal(color uint8) game.Brick {
	return game.Brick{Kind: game.BrickNormal, Color: color, Hits: 1}
}

func armor(hits uint8) game.Brick { return game.Brick{Kind: game.BrickArmor, Hits: hits} }

var solid = game.Brick{Kind: game.BrickSolid}

func TestParse(t *testing.T) {
	type cell struct {
		col, row int
		want     game.Brick
	}
	tests := []struct {
		name      string
		in        string
		cells     []cell
		remaining int
	}{
		{
			name: "letters pick colours",
			in:   "abcdefg\n",
			cells: []cell{
				{0, 0, normal(0)}, {1, 0, normal(1)}, {2, 0, normal(2)}, {3, 0, normal(3)},
				{4, 0, normal(4)}, {5, 0, normal(5)}, {6, 0, normal(6)}, {7, 0, game.Brick{}},
			},
			remaining: 7,
		},
		{
			name:      "armour and solid",
			in:        "2#3.a\n",
			cells:     []cell{{0, 0, armor(2)}, {1, 0, solid}, {2, 0, armor(3)}, {3, 0, game.Brick{}}, {4, 0, normal(0)}},
			remaining: 3,
		},
		{
			name:      "rows map to grid rows in order",
			in:        "a\n.b\n..c\n",
			cells:     []cell{{0, 0, normal(0)}, {1, 1, normal(1)}, {2, 2, normal(2)}, {0, 1, game.Brick{}}},
			remaining: 3,
		},
		{
			name:      "comments and blank lines take no row",
			in:        "; title\n\na\n; mid comment\n\n   \nb\n",
			cells:     []cell{{0, 0, normal(0)}, {0, 1, normal(1)}},
			remaining: 2,
		},
		{
			name:      "empty rows are written with dots",
			in:        "a\n....\nb\n",
			cells:     []cell{{0, 0, normal(0)}, {0, 1, game.Brick{}}, {0, 2, normal(1)}},
			remaining: 2,
		},
		{
			name:      "trailing whitespace, CRLF and no final newline",
			in:        "ab  \r\n.c\t\r\nd",
			cells:     []cell{{1, 0, normal(1)}, {1, 1, normal(2)}, {0, 2, normal(3)}},
			remaining: 4,
		},
		{
			name:      "full width and full height",
			in:        strings.Repeat(strings.Repeat("a", game.BrickCols)+"\n", game.BrickRows),
			cells:     []cell{{0, 0, normal(0)}, {game.BrickCols - 1, game.BrickRows - 1, normal(0)}},
			remaining: game.BrickCols * game.BrickRows,
		},
		{
			name:      "a solid brick can start a row",
			in:        "#a\n",
			cells:     []cell{{0, 0, solid}, {1, 0, normal(0)}},
			remaining: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := Parse(strings.NewReader(tt.in))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			for _, c := range tt.cells {
				if got := g[c.row][c.col]; got != c.want {
					t.Errorf("cell (%d,%d) = %+v, want %+v", c.col, c.row, got, c.want)
				}
			}
			if got := g.Remaining(); got != tt.remaining {
				t.Errorf("breakable bricks = %d, want %d", got, tt.remaining)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	wide := strings.Repeat("a", game.BrickCols+1)
	tall := strings.Repeat("a\n", game.BrickRows+1)

	tests := []struct {
		name      string
		in        string
		line, col int
		msg       string
	}{
		{"unknown character", "ab\na?c\n", 2, 2, `unknown brick '?'`},
		{"uppercase is not a brick", "A\n", 1, 1, `unknown brick 'A'`},
		{"letter past the palette", "abh\n", 1, 3, `unknown brick 'h'`},
		{"armour of one", "1\n", 1, 1, `unknown brick '1'`},
		{"armour too thick", "a4\n", 1, 2, `unknown brick '4'`},
		{"space inside a row", "a b\n", 1, 2, `unknown brick ' '`},
		{"comment marker mid-row", "a;b\n", 1, 2, `unknown brick ';'`},
		{"line numbers count comments and blanks", "; one\n\nab\n.x\n", 4, 2, `unknown brick 'x'`},
		{"row too wide", "a\n" + wide + "\n", 2, game.BrickCols + 1, "row is too wide (limit 20)"},
		{"too many rows", tall, game.BrickRows + 1, 0, "too many rows (limit 14)"},
		{"empty file", "", 0, 0, "level has no breakable bricks"},
		{"only comments", "; nothing\n", 0, 0, "level has no breakable bricks"},
		{"only solid bricks", "###\n...\n", 0, 0, "level has no breakable bricks"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tt.in))
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("err = %v, want a *ParseError", err)
			}
			if pe.Line != tt.line || pe.Col != tt.col || pe.Msg != tt.msg {
				t.Errorf("got line %d col %d %q, want line %d col %d %q", pe.Line, pe.Col, pe.Msg, tt.line, tt.col, tt.msg)
			}
		})
	}
}

func TestParseErrorString(t *testing.T) {
	tests := []struct {
		err  ParseError
		want string
	}{
		{ParseError{3, 5, "unknown brick 'x'"}, "line 3, column 5: unknown brick 'x'"},
		{ParseError{15, 0, "too many rows"}, "line 15: too many rows"},
		{ParseError{0, 0, "level has no breakable bricks"}, "level has no breakable bricks"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}

func TestLoadAll(t *testing.T) {
	t.Run("name order", func(t *testing.T) {
		fsys := fstest.MapFS{
			"10.txt":    {Data: []byte("c\n")},
			"02.txt":    {Data: []byte("b\n")},
			"01.txt":    {Data: []byte("a\n")},
			"notes.md":  {Data: []byte("not a level")},
			"sub/x.txt": {Data: []byte("?")},
		}
		grids, err := LoadAll(fsys)
		if err != nil {
			t.Fatal(err)
		}
		if len(grids) != 3 {
			t.Fatalf("loaded %d levels, want 3", len(grids))
		}
		for i, want := range []uint8{0, 1, 2} {
			if got := grids[i][0][0]; got != normal(want) {
				t.Errorf("level %d first brick = %+v, want colour %d", i, got, want)
			}
		}
	})
	t.Run("error names the file", func(t *testing.T) {
		fsys := fstest.MapFS{"01.txt": {Data: []byte("a\n")}, "02.txt": {Data: []byte("a\nzz\n")}}
		_, err := LoadAll(fsys)
		if err == nil || err.Error() != "02.txt: line 2, column 1: unknown brick 'z'" {
			t.Errorf("err = %v", err)
		}
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Error("error does not wrap a *ParseError")
		}
	})
	t.Run("no files", func(t *testing.T) {
		if _, err := LoadAll(fstest.MapFS{}); err == nil {
			t.Error("want an error for an empty filesystem")
		}
	})
}

// Every built-in level must parse, and every breakable brick in it must be
// reachable by the ball: not sealed in behind indestructible bricks.
func TestBuiltinLevels(t *testing.T) {
	grids, err := LoadAll(levels.FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(grids) < 5 {
		t.Errorf("found %d built-in levels, want at least 5", len(grids))
	}
	for i, g := range grids {
		if sealed := unreachable(&g); len(sealed) > 0 {
			t.Errorf("level %d: bricks sealed in at (col,row) %v", i+1, sealed)
		}
	}
}

// unreachable flood-fills from outside the grid through everything that
// isn't solid and returns the breakable bricks the fill never touched.
func unreachable(g *game.Grid) [][2]int {
	// Pad by one cell all round: the space above, below and (as a
	// simplification, since the grid spans the field) beside the bricks.
	const w, h = game.BrickCols + 2, game.BrickRows + 2
	var seen [h][w]bool
	passable := func(x, y int) bool {
		if x == 0 || y == 0 || x == w-1 || y == h-1 {
			return y == 0 || y == h-1 // only above and below are really open
		}
		return g[y-1][x-1].Kind != game.BrickSolid
	}
	stack := [][2]int{{1, 0}, {1, h - 1}}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := p[0], p[1]
		if x < 0 || y < 0 || x >= w || y >= h || seen[y][x] || !passable(x, y) {
			continue
		}
		seen[y][x] = true
		stack = append(stack, [2]int{x + 1, y}, [2]int{x - 1, y}, [2]int{x, y + 1}, [2]int{x, y - 1})
	}
	var out [][2]int
	for r := range g {
		for c, b := range g[r] {
			if b.Breakable() && !seen[r+1][c+1] {
				out = append(out, [2]int{c, r})
			}
		}
	}
	return out
}

func TestUnreachableDetectsSealedBricks(t *testing.T) {
	sealed, err := Parse(strings.NewReader("a....\n.###.\n.#b#.\n.###.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := unreachable(&sealed); len(got) != 1 || got[0] != [2]int{2, 2} {
		t.Errorf("unreachable = %v, want just (2,2)", got)
	}
	open, _ := Parse(strings.NewReader("a....\n.###.\n.#b#.\n.#.#.\n"))
	if got := unreachable(&open); len(got) != 0 {
		t.Errorf("unreachable = %v, want none", got)
	}
}
