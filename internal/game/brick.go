package game

// Brick grid geometry, in pixels. A brick is one terminal cell tall.
const (
	BrickCols = 20
	BrickRows = 14
	BrickW    = 4.0
	BrickH    = 2.0
	// BrickTop is the gap between the top wall and the first brick row.
	BrickTop = 4.0
)

// Brick is one cell of the grid.
type Brick struct {
	// Color is a palette index; the renderer decides what it looks like.
	Color uint8
	Alive bool
}

// Grid is the brick layout, indexed [row][col].
type Grid [BrickRows][BrickCols]Brick

// BrickRect returns the world-space box of the brick at column c, row r.
func BrickRect(c, r int) AABB {
	x, y := float64(c)*BrickW, BrickTop+float64(r)*BrickH
	return AABB{Vec{x, y}, Vec{x + BrickW, y + BrickH}}
}

// alive reports whether there is a brick at (c, r); out of range is empty.
func (g *Grid) alive(c, r int) bool {
	return c >= 0 && c < BrickCols && r >= 0 && r < BrickRows && g[r][c].Alive
}

// Remaining counts the bricks still standing.
func (g *Grid) Remaining() int {
	n := 0
	for r := range g {
		for c := range g[r] {
			if g[r][c].Alive {
				n++
			}
		}
	}
	return n
}

// exposed strips from a contact normal any face of brick (c, r) that is
// covered by a neighbouring brick. Such a face cannot really be struck: the
// ball meets the neighbour first. The sweep can still report one when the
// ball's corner arrives exactly on the seam between two bricks, as a corner
// contact that is half real face and half hidden one; honouring the hidden
// half would bounce the ball sideways off an edge that isn't there. A zero
// result means the contact was entirely with hidden faces and should be
// ignored.
func (g *Grid) exposed(c, r int, n Vec) Vec {
	if (n.X < 0 && g.alive(c-1, r)) || (n.X > 0 && g.alive(c+1, r)) {
		n.X = 0
	}
	if (n.Y < 0 && g.alive(c, r-1)) || (n.Y > 0 && g.alive(c, r+1)) {
		n.Y = 0
	}
	return n
}

// demoGrid is the stand-in layout used until levels are loaded from files:
// seven coloured rows with a channel up the middle and down each side, so
// the ball can get above the bricks and at their sides.
func demoGrid() Grid {
	var g Grid
	for r := 1; r <= 7; r++ {
		for c := 1; c < BrickCols-1; c++ {
			if c == BrickCols/2-1 || c == BrickCols/2 {
				continue
			}
			g[r][c] = Brick{Color: uint8(r - 1), Alive: true}
		}
	}
	return g
}
