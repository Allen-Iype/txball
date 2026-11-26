package game

import "testing"

const testDT = 1.0 / 120

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
			g := New()
			g.Ball = tt.ball
			g.Update(testDT, tt.in)

			hit := g.Ball.Vel.Y < 0 && tt.ball.Vel.Y > 0
			if hit != tt.wantHit {
				t.Fatalf("hit = %v, want %v (ball %+v)", hit, tt.wantHit, g.Ball)
			}
			if !hit {
				return
			}
			if !near(g.Ball.Pos.Y, PaddleTop-ballHalf) {
				t.Errorf("ball Y = %v, want resting on paddle at %v", g.Ball.Pos.Y, PaddleTop-ballHalf)
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

func TestBallLostIsReserved(t *testing.T) {
	g := New()
	g.Paddle.X = 20
	g.Ball = Ball{Pos: Vec{60, Height + ballHalf}, Vel: Vec{0, BallSpeed}}
	g.Update(testDT, Input{})

	if !near(g.Ball.Pos.X, 20) || g.Ball.Pos.Y > PaddleTop {
		t.Errorf("ball = %+v, want re-served on the paddle at x=20", g.Ball.Pos)
	}
	if g.Ball.Vel.Y >= 0 {
		t.Errorf("ball Vel.Y = %v, want upward", g.Ball.Vel.Y)
	}
}

// A long unattended run must keep the ball inside the walls at constant speed.
func TestBallStaysInBounds(t *testing.T) {
	g := New()
	for i := range 120 * 60 {
		// Keep the paddle under the ball so it stays in play.
		g.Update(testDT, Input{PaddleX: g.Ball.Pos.X + 2, HasMouse: true})
		b := g.Ball
		if b.Pos.X < ballHalf-eps || b.Pos.X > Width-ballHalf+eps || b.Pos.Y < ballHalf-eps || b.Pos.Y > PaddleTop {
			t.Fatalf("tick %d: ball out of bounds at %+v", i, b.Pos)
		}
		if !near(b.Vel.Len(), BallSpeed) {
			t.Fatalf("tick %d: speed drifted to %v", i, b.Vel.Len())
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
