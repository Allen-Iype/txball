package game

import "testing"

const testDT = 1.0 / 120

// playing returns a game with the ball already in flight.
func playing() *Game {
	g := New()
	g.Update(testDT, Input{Launch: true})
	return g
}

// run feeds the same input for n steps.
func run(g *Game, n int, in Input) {
	for range n {
		g.Update(testDT, in)
	}
}

// promptSteps is enough updates to get past promptDelay.
const promptSteps = int(promptDelay/testDT) + 2

func TestPaddleInput(t *testing.T) {
	tests := []struct {
		name  string
		start float64
		in    Input
		want  float64
	}{
		{"mouse jumps to target", 40, Input{PaddleX: 20, HasMouse: true}, 20},
		{"mouse clamps at left wall", 40, Input{PaddleX: -5, HasMouse: true}, PaddleWidth / 2},
		{"mouse clamps at right wall", 40, Input{PaddleX: 500, HasMouse: true}, Width - PaddleWidth/2},
		{"stale mouse position is ignored", 40, Input{PaddleX: 20}, 40},
		{"key right moves by speed*dt", 40, Input{Move: 1}, 40 + PaddleKeySpeed*testDT},
		{"key left moves by speed*dt", 40, Input{Move: -1}, 40 - PaddleKeySpeed*testDT},
		{"key clamps at wall", PaddleWidth / 2, Input{Move: -1}, PaddleWidth / 2},
		{"oversized move is clamped to 1", 40, Input{Move: 7}, 40 + PaddleKeySpeed*testDT},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New()
			g.Paddle.X = tt.start
			g.Ball = Ball{Pos: Vec{40, 10}} // parked out of the way
			g.Update(testDT, tt.in)
			if !near(g.Paddle.X, tt.want) {
				t.Errorf("paddle X = %v, want %v", g.Paddle.X, tt.want)
			}
		})
	}
}

func TestBallPaddleCollision(t *testing.T) {
	// The paddle is centred at 40, so it spans 34..46.
	above := PaddleTop - ballHalf - 0.1 // one step from touching
	down := Vec{0, BallSpeed}

	tests := []struct {
		name     string
		ball     Ball
		in       Input
		wantHit  bool
		wantVelX int // sign of the outgoing X velocity when hit
	}{
		{"centre hit goes straight up", Ball{Vec{40, above}, down}, Input{}, true, 0},
		{"right half deflects right", Ball{Vec{44, above}, down}, Input{}, true, 1},
		{"left half deflects left", Ball{Vec{36, above}, down}, Input{}, true, -1},
		{"clipping the right corner counts", Ball{Vec{46.3, above}, down}, Input{}, true, 1},
		{"just past the right edge misses", Ball{Vec{46.6, above}, down}, Input{}, false, 0},
		{"just past the left edge misses", Ball{Vec{33.4, above}, down}, Input{}, false, 0},
		{"moving up through the paddle is ignored", Ball{Vec{40, PaddleTop + 0.2}, Vec{0, -BallSpeed}}, Input{}, false, 0},
		{"already below the top face falls through", Ball{Vec{40, PaddleTop + 0.3}, down}, Input{}, false, 0},
		{"paddle jumping under the ball catches it", Ball{Vec{70, above}, down}, Input{PaddleX: 70, HasMouse: true}, true, 0},
		{"paddle jumping away misses", Ball{Vec{40, above}, down}, Input{PaddleX: 70, HasMouse: true}, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := playing()
			g.Ball = tt.ball
			g.Update(testDT, tt.in)
			travel := BallSpeed * testDT

			hit := g.Ball.Vel.Y < 0 && tt.ball.Vel.Y > 0
			if hit != tt.wantHit {
				t.Fatalf("hit = %v, want %v (ball %+v)", hit, tt.wantHit, g.Ball)
			}
			if !hit {
				return
			}
			// It touched down 0.1 above the paddle's reach and spends
			// the rest of the step climbing away.
			if y := g.Ball.Pos.Y; y > PaddleTop-ballHalf || y < PaddleTop-ballHalf-travel {
				t.Errorf("ball Y = %v, want just above the paddle at %v", y, PaddleTop-ballHalf)
			}
			if !near(g.Ball.Vel.Len(), BallSpeed) {
				t.Errorf("speed = %v, want %v", g.Ball.Vel.Len(), BallSpeed)
			}
			if got := sign(g.Ball.Vel.X); got != tt.wantVelX {
				t.Errorf("X velocity sign = %d, want %d", got, tt.wantVelX)
			}
		})
	}
}

// A long unattended run must keep the ball inside the walls at constant speed.
func TestBallStaysInBounds(t *testing.T) {
	g := playing()
	for i := range 120 * 60 {
		// Keep the paddle under the ball so it stays in play.
		g.Update(testDT, Input{PaddleX: g.Ball.Pos.X + 2, HasMouse: true, Launch: true})
		if g.State != StatePlaying {
			continue // between levels
		}
		b := g.Ball
		if b.Pos.X < ballHalf-eps || b.Pos.X > Width-ballHalf+eps || b.Pos.Y < ballHalf-eps || b.Pos.Y > PaddleTop {
			t.Fatalf("tick %d: ball out of bounds at %+v", i, b.Pos)
		}
		if !near(b.Vel.Len(), g.BallSpeed()) {
			t.Fatalf("tick %d: speed drifted to %v, want %v", i, b.Vel.Len(), g.BallSpeed())
		}
	}
	if g.Lives != StartLives {
		t.Errorf("lives = %d, want %d: a tracked ball should never be lost", g.Lives, StartLives)
	}
}

// bricksAt builds a grid with bricks at the given (col, row) cells.
func bricksAt(cells ...[2]int) Grid {
	var g Grid
	for _, c := range cells {
		g[c[1]][c[0]] = Brick{Alive: true}
	}
	return g
}

func TestBallBrickCollision(t *testing.T) {
	// Brick (5,3) occupies x 20..24, y 10..12. Its neighbours: (4,3) is
	// x 16..20, (6,3) is x 24..28, (5,4) is y 12..14.
	type cell = [2]int
	const v = BallSpeed

	tests := []struct {
		name     string
		bricks   []cell
		ball     Ball
		dt       float64
		wantVel  Vec
		wantDead []cell
		wantLive []cell
	}{
		{
			name:     "underside flips Y only",
			bricks:   []cell{{5, 3}},
			ball:     Ball{Vec{22, 12.6}, Vec{0, -v}},
			wantVel:  Vec{0, v},
			wantDead: []cell{{5, 3}},
		},
		{
			name:     "top flips Y only",
			bricks:   []cell{{5, 3}},
			ball:     Ball{Vec{22, 9.4}, Vec{0, v}},
			wantVel:  Vec{0, -v},
			wantDead: []cell{{5, 3}},
		},
		{
			name:     "left side flips X only",
			bricks:   []cell{{5, 3}},
			ball:     Ball{Vec{19.4, 11}, Vec{v * 0.8, -v * 0.6}},
			wantVel:  Vec{-v * 0.8, -v * 0.6},
			wantDead: []cell{{5, 3}},
		},
		{
			name:     "right side flips X only",
			bricks:   []cell{{5, 3}},
			ball:     Ball{Vec{24.6, 11}, Vec{-v * 0.8, v * 0.6}},
			wantVel:  Vec{v * 0.8, v * 0.6},
			wantDead: []cell{{5, 3}},
		},
		{
			name:     "shallow diagonal into the underside is not a side hit",
			bricks:   []cell{{5, 3}},
			ball:     Ball{Vec{20.5, 12.6}, Vec{v * 0.8, -v * 0.6}},
			wantVel:  Vec{v * 0.8, v * 0.6},
			wantDead: []cell{{5, 3}},
		},
		{
			name:     "near miss leaves the brick alone",
			bricks:   []cell{{5, 3}},
			ball:     Ball{Vec{24.6, 12.6}, Vec{v * 0.6, -v * 0.8}},
			wantVel:  Vec{v * 0.6, -v * 0.8},
			wantLive: []cell{{5, 3}},
		},
		{
			// The ball skims left to right under a row and reaches the
			// seam between (4,3) and (5,3). It must bounce down off the
			// row, never sideways off (5,3)'s hidden left face.
			name:     "seam between two bricks is not an edge",
			bricks:   []cell{{4, 3}, {5, 3}},
			ball:     Ball{Vec{19.3, 12.55}, Vec{v * 0.96, -v * 0.28}},
			wantVel:  Vec{v * 0.96, v * 0.28},
			wantDead: []cell{{4, 3}},
			wantLive: []cell{{5, 3}},
		},
		{
			name:     "seam in a column is not an edge",
			bricks:   []cell{{5, 3}, {5, 4}},
			ball:     Ball{Vec{19.45, 11.3}, Vec{v * 0.28, v * 0.96}},
			wantVel:  Vec{-v * 0.28, v * 0.96},
			wantDead: []cell{{5, 3}},
			wantLive: []cell{{5, 4}},
		},
		{
			// The ball's corner arrives exactly at the seam, travelling
			// right to left, so the sweep reports a corner contact with
			// (4,3) at the same instant as the underside of (5,3). The
			// corner's sideways half belongs to (4,3)'s hidden right
			// face and must be discarded.
			name:     "exact corner contact at a seam ignores the hidden face",
			bricks:   []cell{{4, 3}, {5, 3}},
			ball:     Ball{Vec{21.5, 13.5}, Vec{-240, -240}},
			wantVel:  Vec{-240, 240},
			wantDead: []cell{{4, 3}},
			wantLive: []cell{{5, 3}},
		},
		{
			name:     "exact corner contact at a column seam ignores the hidden face",
			bricks:   []cell{{5, 3}, {5, 4}},
			ball:     Ball{Vec{18.5, 13.5}, Vec{240, -240}},
			wantVel:  Vec{-240, -240},
			wantDead: []cell{{5, 3}},
			wantLive: []cell{{5, 4}},
		},
		{
			name:     "exact corner of a lone brick turns the ball right back",
			bricks:   []cell{{5, 3}},
			ball:     Ball{Vec{18.5, 13.5}, Vec{240, -240}},
			wantVel:  Vec{-240, 240},
			wantDead: []cell{{5, 3}},
		},
		{
			name:     "square on a seam breaks one brick, not two",
			bricks:   []cell{{4, 3}, {5, 3}},
			ball:     Ball{Vec{20, 12.6}, Vec{0, -v}},
			wantVel:  Vec{0, v},
			wantDead: []cell{{4, 3}},
			wantLive: []cell{{5, 3}},
		},
		{
			// An inside corner: (5,3) overhead and (6,4) to the right.
			// Both are struck in the same step and the ball comes straight
			// back out.
			name:     "inside corner hits both bricks in one step",
			bricks:   []cell{{5, 3}, {6, 4}},
			ball:     Ball{Vec{23.3, 12.7}, Vec{v * 0.7071, -v * 0.7071}},
			wantVel:  Vec{-v * 0.7071, v * 0.7071},
			wantDead: []cell{{5, 3}, {6, 4}},
		},
		{
			// 4000 px/s covers 33 px in a step: far more than a brick.
			name:     "no tunnelling at extreme speed",
			bricks:   []cell{{5, 3}},
			ball:     Ball{Vec{22, 30}, Vec{0, -4000}},
			wantVel:  Vec{0, 4000},
			wantDead: []cell{{5, 3}},
		},
		{
			// One step long enough to cross both bricks: the ball takes
			// the first and bounces away instead of ploughing through.
			name:     "extreme speed takes bricks one at a time",
			bricks:   []cell{{5, 3}, {5, 2}},
			ball:     Ball{Vec{22, 13}, Vec{0, -200}},
			dt:       0.5,
			wantVel:  Vec{0, 200},
			wantDead: []cell{{5, 3}},
			wantLive: []cell{{5, 2}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New()
			g.Paddle.X = 70 // out of the way
			g.Bricks = bricksAt(append(tt.bricks, cell{19, 13})...)
			g.Ball = tt.ball
			dt := tt.dt
			if dt == 0 {
				dt = testDT
			}
			g.moveBall(&g.Ball, dt)

			if got := g.Ball.Vel; !near(got.X, tt.wantVel.X) || !near(got.Y, tt.wantVel.Y) {
				t.Errorf("velocity = %v, want %v", got, tt.wantVel)
			}
			for _, c := range tt.wantDead {
				if g.Bricks.alive(c[0], c[1]) {
					t.Errorf("brick %v survived", c)
				}
			}
			for _, c := range tt.wantLive {
				if !g.Bricks.alive(c[0], c[1]) {
					t.Errorf("brick %v was destroyed", c)
				}
			}
		})
	}
}

// However fast the ball goes, it must never end a step inside a brick or
// outside the walls.
func TestNoTunnellingSoak(t *testing.T) {
	for _, speed := range []float64{40, 400, 4000, 40000} {
		g := playing()
		g.Ball.Vel = paddleBounce(0.37, speed)
		for i := range 5000 {
			g.Update(testDT, Input{PaddleX: g.Ball.Pos.X + 1, HasMouse: true})
			if g.State != StatePlaying {
				// Cleared the level: rebuild and keep hammering.
				g.Bricks = demoGrid()
				g.State = StatePlaying
				g.Ball = Ball{Pos: Vec{40, 38}, Vel: paddleBounce(0.37, speed)}
				continue
			}
			p := g.Ball.Pos
			if p.X < ballHalf-eps || p.X > Width-ballHalf+eps || p.Y < ballHalf-eps {
				t.Fatalf("speed %v tick %d: ball outside walls at %+v", speed, i, p)
			}
			for r := range g.Bricks {
				for c := range g.Bricks[r] {
					b := BrickRect(c, r)
					const in = ballHalf - 1e-6
					if g.Bricks[r][c].Alive && p.X > b.Min.X-in && p.X < b.Max.X+in && p.Y > b.Min.Y-in && p.Y < b.Max.Y+in {
						t.Fatalf("speed %v tick %d: ball at %+v is inside brick (%d,%d)", speed, i, p, c, r)
					}
				}
			}
		}
	}
}

func sign(v float64) int {
	switch {
	case v > eps:
		return 1
	case v < -eps:
		return -1
	}
	return 0
}
