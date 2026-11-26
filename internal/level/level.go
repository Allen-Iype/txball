// Package level reads brick layouts from text.
//
// A level is a picture of the brick grid, one character per brick:
//
//	.    empty
//	a-g  one-hit brick; the letter picks the colour
//	2 3  armoured brick that takes that many hits
//	#    indestructible brick
//
// Lines starting with ';' are comments and blank lines are ignored. A
// level has at most game.BrickRows rows of at most game.BrickCols columns;
// short rows are padded with empty cells on the right.
package level

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"

	"github.com/Allen-Iype/txball/internal/game"
)

// ParseError locates a problem in a level file. Line and Col are 1-based;
// Col is 0 when the problem is with the line or the file as a whole.
type ParseError struct {
	Line, Col int
	Msg       string
}

func (e *ParseError) Error() string {
	switch {
	case e.Line == 0:
		return e.Msg
	case e.Col == 0:
		return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
	}
	return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Col, e.Msg)
}

// Parse reads one level.
func Parse(r io.Reader) (game.Grid, error) {
	var (
		grid game.Grid
		row  int
		line int
	)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line++
		text := strings.TrimRight(sc.Text(), " \t\r")
		if text == "" || text[0] == ';' {
			continue
		}
		if row == game.BrickRows {
			return game.Grid{}, &ParseError{Line: line, Msg: fmt.Sprintf("too many rows (limit %d)", game.BrickRows)}
		}
		col := 0
		for _, ch := range text {
			if col == game.BrickCols {
				return game.Grid{}, &ParseError{Line: line, Col: col + 1, Msg: fmt.Sprintf("row is too wide (limit %d)", game.BrickCols)}
			}
			b, ok := brick(ch)
			if !ok {
				return game.Grid{}, &ParseError{Line: line, Col: col + 1, Msg: fmt.Sprintf("unknown brick %q", ch)}
			}
			grid[row][col] = b
			col++
		}
		row++
	}
	if err := sc.Err(); err != nil {
		return game.Grid{}, err
	}
	if grid.Remaining() == 0 {
		return game.Grid{}, &ParseError{Msg: "level has no breakable bricks"}
	}
	return grid, nil
}

func brick(ch rune) (game.Brick, bool) {
	switch {
	case ch == '.':
		return game.Brick{}, true
	case ch >= 'a' && ch <= 'g':
		return game.Brick{Kind: game.BrickNormal, Color: uint8(ch - 'a'), Hits: 1}, true
	case ch >= '2' && ch <= '0'+game.MaxArmor:
		return game.Brick{Kind: game.BrickArmor, Hits: uint8(ch - '0')}, true
	case ch == '#':
		return game.Brick{Kind: game.BrickSolid}, true
	}
	return game.Brick{}, false
}

// LoadAll parses every *.txt file at the root of fsys, in name order.
func LoadAll(fsys fs.FS) ([]game.Grid, error) {
	names, err := fs.Glob(fsys, "*.txt")
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no level files found")
	}
	slices.Sort(names)
	grids := make([]game.Grid, 0, len(names))
	for _, name := range names {
		f, err := fsys.Open(name)
		if err != nil {
			return nil, err
		}
		g, err := Parse(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		grids = append(grids, g)
	}
	return grids, nil
}
