package app

import (
	"math"

	"github.com/Allen-Iype/txball/internal/game"
)

const (
	// demoSpeed is how fast the self-playing paddle moves, in pixels per
	// second: quick enough to keep up, slow enough to look hand-driven.
	demoSpeed = 110.0
	// demoPress is how often, in seconds, the demo presses launch when
	// something is waiting on it.
	demoPress = 0.9
)

// demoTarget picks where the self-playing paddle wants to be at time t
// (seconds), and whether something is waiting for the launch button.
//
// It goes for the ball that will reach the paddle soonest, or failing
// that the lowest capsule, and aims to meet it off-centre by an amount
// that drifts over time so that the bounces vary.
func demoTarget(g *game.Game, t float64) (x float64, wantLaunch bool) {
	x = g.Paddle.X
	wantLaunch = g.State == game.StateLevelClear || g.State == game.StateGameOver

	lowest := math.Inf(-1)
	for _, b := range g.Balls {
		if b.Stuck {
			wantLaunch = true
			continue
		}
		if b.Vel.Y > 0 && b.Pos.Y > lowest {
			lowest, x = b.Pos.Y, b.Pos.X
		}
	}
	if math.IsInf(lowest, -1) {
		for _, p := range g.Powerups {
			if p.Pos.Y > lowest {
				lowest, x = p.Pos.Y, p.Pos.X
			}
		}
		if math.IsInf(lowest, -1) {
			return g.Paddle.X, wantLaunch
		}
		return x, wantLaunch
	}
	return x + 0.35*g.Paddle.W*math.Sin(1.7*t), wantLaunch
}

// demo drives the game by itself: it fills in for the mouse and the
// launch button.
type demo struct {
	lastPress float64
}

// steer sets in for the coming frame of length frame seconds.
func (d *demo) steer(g *game.Game, in *game.Input, t, frame float64) {
	target, wantLaunch := demoTarget(g, t)
	step := clampAbs(target-g.Paddle.X, demoSpeed*frame)
	in.PaddleX, in.HasMouse = g.Paddle.X+step, true
	if wantLaunch && t-d.lastPress >= demoPress {
		in.Launch, d.lastPress = true, t
	}
}

func clampAbs(v, limit float64) float64 {
	return max(-limit, min(v, limit))
}
