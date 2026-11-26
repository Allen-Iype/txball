// Package game holds TX-Ball's state and update logic. It knows nothing about
// the terminal: coordinates are float64 world units, where one unit is one
// half-block "pixel" on screen. X grows right, Y grows down.
package game

import "math"

// World dimensions in pixels. The playfield is fixed-size; the renderer
// centres it in whatever terminal it gets.
const (
	Width  = 80
	Height = 44
)

// Tuning. Distances in pixels, speeds in pixels per second.
const (
	PaddleWidth  = 12.0
	PaddleHeight = 2.0
	PaddleTop    = Height - 4.0
	// PaddleKeySpeed is how fast the keyboard moves the paddle.
	PaddleKeySpeed = 90.0

	BallSize  = 1.0
	BallSpeed = 40.0
	// MaxBounceAngle is the steepest deflection off the paddle's edge,
	// measured from straight up.
	MaxBounceAngle = 60 * math.Pi / 180

	ballHalf = BallSize / 2

	// maxBounces bounds the contacts resolved for one ball in one step.
	maxBounces = 8
)

// Input is the player's intent for a single update step.
type Input struct {
	// PaddleX is the world X the paddle centre should jump to. It is only
	// read when HasMouse is set.
	PaddleX  float64
	HasMouse bool
	// Move is keyboard steering from -1 (left) to +1 (right).
	Move float64
}

// Paddle is the player's bat. X is its centre.
type Paddle struct {
	X, W float64
}

func (p Paddle) Left() float64  { return p.X - p.W/2 }
func (p Paddle) Right() float64 { return p.X + p.W/2 }

// moveTo centres the paddle on x, keeping it inside the field.
func (p *Paddle) moveTo(x float64) {
	p.X = clamp(x, p.W/2, Width-p.W/2)
}

// Ball is a BallSize square; Pos is its centre.
type Ball struct {
	Pos, Vel Vec
}

// Game is the whole simulation state.
type Game struct {
	// Ticks counts fixed updates since the game started.
	Ticks uint64
	// Time is the simulated time in seconds.
	Time float64

	Paddle Paddle
	Ball   Ball
	Bricks Grid
}

// New returns a game ready for its first Update.
func New() *Game {
	g := &Game{Paddle: Paddle{X: Width / 2, W: PaddleWidth}, Bricks: demoGrid()}
	g.serve()
	return g
}

// serve puts the ball on the paddle and sends it up, slightly off vertical
// so it doesn't just bounce in place.
func (g *Game) serve() {
	g.Ball = Ball{
		Pos: Vec{g.Paddle.X, PaddleTop - ballHalf},
		Vel: paddleBounce(0.3, BallSpeed),
	}
}

// Update advances the simulation by one fixed step of dt seconds.
func (g *Game) Update(dt float64, in Input) {
	g.Ticks++
	g.Time += dt

	if in.HasMouse {
		g.Paddle.moveTo(in.PaddleX)
	}
	if in.Move != 0 {
		g.Paddle.moveTo(g.Paddle.X + clamp(in.Move, -1, 1)*PaddleKeySpeed*dt)
	}

	g.moveBall(&g.Ball, dt)

	if g.Ball.Pos.Y-ballHalf > Height {
		g.serve()
	}
	if g.Bricks.Remaining() == 0 {
		g.Bricks = demoGrid()
		g.serve()
	}
}

// contact is the first thing a moving ball touches.
type contact struct {
	t      float64 // fraction of the displacement travelled
	n      Vec     // surface normal
	paddle bool
	brick  bool
	c, r   int // brick cell, when brick is set
}

// moveBall advances b by dt, resolving every contact along the way. The
// move is swept rather than stepped: each pass finds the earliest contact
// on the remaining path, moves the ball exactly there, bounces it, and
// carries on with the time left. Nothing can be tunnelled through, whatever
// the speed.
func (g *Game) moveBall(b *Ball, dt float64) {
	for range maxBounces {
		d := b.Vel.Scale(dt)
		hit, ok := g.firstContact(b.Pos, d)
		if !ok {
			b.Pos = b.Pos.Add(d)
			return
		}
		b.Pos = b.Pos.Add(d.Scale(hit.t))
		dt *= 1 - hit.t

		switch {
		case hit.paddle:
			b.Vel = paddleBounce((b.Pos.X-g.Paddle.X)/(g.Paddle.W/2), b.Vel.Len())
		case hit.brick:
			b.Vel = reflect(b.Vel, hit.n)
			g.Bricks[hit.r][hit.c].Alive = false
		default:
			b.Vel = reflect(b.Vel, hit.n)
		}
	}
}

// firstContact finds the earliest contact for a ball at pos moving by d.
// Ties go to walls, then the paddle, then bricks in row order; whatever
// loses a tie is still in the way on the next pass and is hit at t=0.
func (g *Game) firstContact(pos, d Vec) (best contact, ok bool) {
	if t, n, hit := sweepWalls(pos, ballHalf, d, Width); hit {
		best, ok = contact{t: t, n: n}, true
	}
	if t, hit := sweepPaddle(pos, d, g.Paddle); hit && (!ok || t < best.t) {
		best, ok = contact{t: t, paddle: true}, true
	}
	for r := range g.Bricks {
		for c := range g.Bricks[r] {
			if !g.Bricks[r][c].Alive {
				continue
			}
			t, n, hit := sweepAABB(pos, ballHalf, d, BrickRect(c, r))
			if !hit || (ok && t >= best.t) {
				continue
			}
			if n = g.Bricks.exposed(c, r, n); n == (Vec{}) {
				continue
			}
			best, ok = contact{t: t, n: n, brick: true, c: c, r: r}, true
		}
	}
	return best, ok
}

// sweepPaddle reports when a ball at pos moving by d lands on the paddle.
// Only the top face counts, and only from above. The horizontal test uses
// the paddle's current position, so a paddle that jumped under the ball
// this step still catches it.
func sweepPaddle(pos, d Vec, p Paddle) (t float64, ok bool) {
	const plane = PaddleTop - ballHalf
	if d.Y <= 0 || pos.Y > plane || pos.Y+d.Y < plane {
		return 0, false
	}
	t = (plane - pos.Y) / d.Y
	x := pos.X + d.X*t
	return t, x+ballHalf > p.Left() && x-ballHalf < p.Right()
}
