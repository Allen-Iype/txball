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

	StartLives = 3
	BrickScore = 10
	// LevelSpeedup is the fraction of BallSpeed added per level, up to
	// MaxSpeedup.
	LevelSpeedup = 0.08
	MaxSpeedup   = 0.8

	// serveOffset is where the ball waits on the paddle, as a fraction of
	// its half-width. Being off-centre gives the launch a slight angle.
	serveOffset = 0.25
	// promptDelay is how long the level-clear and game-over screens ignore
	// the launch button, so a click meant for the game doesn't dismiss them.
	promptDelay = 0.6
)

// State is the phase of play.
type State uint8

const (
	// StateServe: the ball rides the paddle until launched.
	StateServe State = iota
	StatePlaying
	StatePaused
	// StateLevelClear: every brick is gone; launch starts the next level.
	StateLevelClear
	// StateGameOver: no lives left; launch starts a new game.
	StateGameOver
)

func (s State) String() string {
	switch s {
	case StateServe:
		return "Serve"
	case StatePlaying:
		return "Playing"
	case StatePaused:
		return "Paused"
	case StateLevelClear:
		return "LevelClear"
	case StateGameOver:
		return "GameOver"
	}
	return "State(?)"
}

// Input is the player's intent for a single update step.
type Input struct {
	// PaddleX is the world X the paddle centre should jump to. It is only
	// read when HasMouse is set.
	PaddleX  float64
	HasMouse bool
	// Move is keyboard steering from -1 (left) to +1 (right).
	Move float64
	// Launch and Pause are button presses. The caller sets them for one
	// update only; they are edges, not held states.
	Launch bool
	Pause  bool
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
	// Ticks counts fixed updates since the game started, excluding pauses.
	Ticks uint64
	// Time is the simulated time in seconds.
	Time float64

	State State
	Level int // 1-based
	Lives int
	Score int

	Paddle Paddle
	Ball   Ball
	Bricks Grid

	// levels are the layouts, played in order and then repeated.
	levels []Grid
	// since is how long the game has been in State.
	since float64
	// resume is the state to return to from StatePaused.
	resume State
}

// New returns a game at the start of level 1, waiting to serve. The layouts
// are played in order, wrapping around after the last; there must be at
// least one.
func New(levels []Grid) *Game {
	if len(levels) == 0 {
		panic("game: New needs at least one level")
	}
	g := &Game{
		Lives:  StartLives,
		Paddle: Paddle{X: Width / 2, W: PaddleWidth},
		levels: levels,
	}
	g.JumpTo(1)
	return g
}

// JumpTo sets up the given 1-based level, ready to serve. Score and lives
// are kept.
func (g *Game) JumpTo(level int) {
	g.Level = max(level, 1)
	g.Bricks = g.levels[(g.Level-1)%len(g.levels)]
	g.parkBall()
	g.setState(StateServe)
}

// BallSpeed returns the launch speed for the current level.
func (g *Game) BallSpeed() float64 {
	return BallSpeed * (1 + min(float64(g.Level-1)*LevelSpeedup, MaxSpeedup))
}

func (g *Game) setState(s State) {
	g.State, g.since = s, 0
}

// parkBall rests the ball on the paddle at the serving spot.
func (g *Game) parkBall() {
	g.Ball = Ball{Pos: Vec{g.Paddle.X + serveOffset*g.Paddle.W/2, PaddleTop - ballHalf}}
}

// Update advances the simulation by one fixed step of dt seconds.
func (g *Game) Update(dt float64, in Input) {
	if in.Pause {
		switch g.State {
		case StatePaused:
			g.State = g.resume
		case StateServe, StatePlaying:
			g.resume, g.State = g.State, StatePaused
		}
	}
	if g.State == StatePaused {
		return
	}

	g.Ticks++
	g.Time += dt
	g.since += dt

	if in.HasMouse {
		g.Paddle.moveTo(in.PaddleX)
	}
	if in.Move != 0 {
		g.Paddle.moveTo(g.Paddle.X + clamp(in.Move, -1, 1)*PaddleKeySpeed*dt)
	}

	switch g.State {
	case StateServe:
		g.parkBall()
		if in.Launch {
			g.Ball.Vel = paddleBounce(serveOffset, g.BallSpeed())
			g.setState(StatePlaying)
		}

	case StatePlaying:
		g.moveBall(&g.Ball, dt)
		switch {
		case g.Bricks.Remaining() == 0:
			g.setState(StateLevelClear)
		case g.Ball.Pos.Y-ballHalf > Height:
			g.Lives--
			if g.Lives <= 0 {
				g.setState(StateGameOver)
			} else {
				g.parkBall()
				g.setState(StateServe)
			}
		}

	case StateLevelClear:
		if in.Launch && g.since >= promptDelay {
			g.JumpTo(g.Level + 1)
		}

	case StateGameOver:
		if in.Launch && g.since >= promptDelay {
			paddle := g.Paddle
			*g = *New(g.levels)
			g.Paddle = paddle // don't yank it away from the mouse
			g.parkBall()
		}
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
			if g.Bricks[hit.r][hit.c].hit() {
				g.Score += BrickScore
			}
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
			if !g.Bricks[r][c].Alive() {
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
