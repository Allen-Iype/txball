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
}

// New returns a game ready for its first Update.
func New() *Game {
	g := &Game{Paddle: Paddle{X: Width / 2, W: PaddleWidth}}
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

	g.stepBall(dt)
}

func (g *Game) stepBall(dt float64) {
	b, p := &g.Ball, g.Paddle

	prevBottom := b.Pos.Y + ballHalf
	b.Pos = b.Pos.Add(b.Vel.Scale(dt))
	b.Pos, b.Vel = bounceWalls(b.Pos, b.Vel, ballHalf, Width)

	// The paddle only counts when the ball comes down onto its top face.
	// Testing the horizontal overlap against the paddle's current position
	// means a paddle that jumped under the ball this step still catches it.
	crossedTop := b.Vel.Y > 0 && prevBottom <= PaddleTop && b.Pos.Y+ballHalf >= PaddleTop
	if crossedTop && b.Pos.X+ballHalf > p.Left() && b.Pos.X-ballHalf < p.Right() {
		b.Pos.Y = PaddleTop - ballHalf
		b.Vel = paddleBounce((b.Pos.X-p.X)/(p.W/2), b.Vel.Len())
	}

	if b.Pos.Y-ballHalf > Height {
		g.serve()
	}
}
