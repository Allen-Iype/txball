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

// BrickKind says how a brick behaves when hit.
type BrickKind uint8

const (
	// BrickNone is an empty cell.
	BrickNone BrickKind = iota
	// BrickNormal breaks in one hit.
	BrickNormal
	// BrickArmor takes Hits hits to break.
	BrickArmor
	// BrickSolid never breaks and doesn't count towards clearing a level.
	BrickSolid
)

// MaxArmor is the most hits an armoured brick can take.
const MaxArmor = 3

// Brick is one cell of the grid. The zero value is an empty cell.
type Brick struct {
	Kind BrickKind
	// Color is a palette index for normal bricks; the renderer decides
	// what it looks like.
	Color uint8
	// Hits is how many more hits a breakable brick survives.
	Hits uint8
}

// Alive reports whether the cell holds a brick.
func (b Brick) Alive() bool { return b.Kind != BrickNone }

// Breakable reports whether the brick can be destroyed.
func (b Brick) Breakable() bool { return b.Kind == BrickNormal || b.Kind == BrickArmor }

// hit applies one ball strike and reports whether the brick took damage.
func (b *Brick) hit() bool {
	if !b.Breakable() {
		return false
	}
	if b.Hits--; b.Hits == 0 {
		*b = Brick{}
	}
	return true
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
	return c >= 0 && c < BrickCols && r >= 0 && r < BrickRows && g[r][c].Alive()
}

// Remaining counts the breakable bricks still standing.
func (g *Grid) Remaining() int {
	n := 0
	for r := range g {
		for c := range g[r] {
			if g[r][c].Breakable() {
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
