package app

import (
	"math/rand/v2"
	"testing"

	"github.com/Allen-Iype/txball/internal/game"
	"github.com/Allen-Iype/txball/internal/level"
	"github.com/Allen-Iype/txball/levels"
)

func TestDemoTarget(t *testing.T) {
	down, up := game.Vec{Y: 40}, game.Vec{Y: -40}
	tests := []struct {
		name       string
		prep       func(g *game.Game)
		wantX      float64
		wantLaunch bool
	}{
		{"serving: stay put and launch", func(g *game.Game) {}, 40, true},
		{"follows a falling ball", func(g *game.Game) {
			g.Balls = []game.Ball{{Pos: game.Vec{X: 15, Y: 20}, Vel: down}}
		}, 15, false},
		{"prefers the lowest falling ball", func(g *game.Game) {
			g.Balls = []game.Ball{
				{Pos: game.Vec{X: 15, Y: 20}, Vel: down},
				{Pos: game.Vec{X: 60, Y: 35}, Vel: down},
				{Pos: game.Vec{X: 70, Y: 38}, Vel: up},
			}
		}, 60, false},
		{"ball rising, nothing falling: stay put", func(g *game.Game) {
			g.Balls = []game.Ball{{Pos: game.Vec{X: 15, Y: 20}, Vel: up}}
		}, 40, false},
		{"ball rising: fetch the lowest capsule", func(g *game.Game) {
			g.Balls = []game.Ball{{Pos: game.Vec{X: 15, Y: 20}, Vel: up}}
			g.Powerups = []game.Powerup{{Pos: game.Vec{X: 5, Y: 10}}, {Pos: game.Vec{X: 66, Y: 30}}}
		}, 66, false},
		{"a falling ball beats a capsule", func(g *game.Game) {
			g.Balls = []game.Ball{{Pos: game.Vec{X: 15, Y: 20}, Vel: down}}
			g.Powerups = []game.Powerup{{Pos: game.Vec{X: 66, Y: 30}}}
		}, 15, false},
		{"a ball held by the sticky paddle wants launching", func(g *game.Game) {
			g.Balls = []game.Ball{{Stuck: true}, {Pos: game.Vec{X: 22, Y: 20}, Vel: down}}
		}, 22, true},
		{"level-clear screen wants launching", func(g *game.Game) {
			g.Balls, g.State = nil, game.StateLevelClear
		}, 40, true},
		{"game-over screen wants launching", func(g *game.Game) {
			g.Balls, g.State = nil, game.StateGameOver
		}, 40, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := builtinGame(t)
			tt.prep(g)
			// At t=0 the aim offset is zero, so the target is exact.
			x, launch := demoTarget(g, 0)
			if x != tt.wantX || launch != tt.wantLaunch {
				t.Errorf("target = %v, launch = %v; want %v, %v", x, launch, tt.wantX, tt.wantLaunch)
			}
		})
	}
}

// The demo has to be able to play: over three simulated minutes it must
// score, clear at least one level, and move at a believable speed.
func TestDemoPlays(t *testing.T) {
	grids, err := level.LoadAll(levels.FS)
	if err != nil {
		t.Fatal(err)
	}
	g := game.New(grids, rand.New(rand.NewPCG(5, 8)))

	var (
		d      demo
		in     game.Input
		levels = 1
	)
	const frame = 1.0 / renderHz
	for i := range renderHz * 180 {
		prev := g.Paddle.X
		d.steer(g, &in, float64(i)*frame, frame)
		for range updateHz / renderHz {
			g.Update(dt, in)
			in.HasMouse, in.Launch, in.Pause = false, false, false
		}
		// Effects may resize the paddle and so shift its centre a little.
		if moved := g.Paddle.X - prev; moved > demoSpeed*frame+4 || moved < -demoSpeed*frame-4 {
			t.Fatalf("frame %d: paddle jumped %v px", i, moved)
		}
		levels = max(levels, g.Level)
	}
	if g.High < 500 || levels < 2 {
		t.Errorf("demo reached level %d with a best score of %d; want it to clear a level", levels, g.High)
	}
}
