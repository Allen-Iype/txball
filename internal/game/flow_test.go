package game

import "testing"

// lose drops the ball out of the bottom on the next update.
func lose(g *Game) {
	g.Ball = Ball{Pos: Vec{40, Height + 1}, Vel: Vec{0, BallSpeed}}
	g.Update(testDT, Input{})
}

// clear removes every brick and lets the game notice.
func clearLevel(g *Game) {
	g.Bricks = Grid{}
	g.Update(testDT, Input{})
}

func TestStateTransitions(t *testing.T) {
	launch, pause := Input{Launch: true}, Input{Pause: true}

	type want struct {
		state State
		level int
		lives int
	}
	tests := []struct {
		name string
		do   func(g *Game)
		want want
	}{
		{"new game waits to serve", func(g *Game) {}, want{StateServe, 1, 3}},
		{"idle serve stays put", func(g *Game) { run(g, 200, Input{}) }, want{StateServe, 1, 3}},
		{"launch starts play", func(g *Game) { g.Update(testDT, launch) }, want{StatePlaying, 1, 3}},
		{"launch during play does nothing", func(g *Game) { run(g, 3, launch) }, want{StatePlaying, 1, 3}},

		{"losing the ball costs a life and re-serves", func(g *Game) {
			g.Update(testDT, launch)
			lose(g)
		}, want{StateServe, 1, 2}},
		{"losing the last ball ends the game", func(g *Game) {
			for range StartLives {
				g.Update(testDT, launch)
				lose(g)
			}
		}, want{StateGameOver, 1, 0}},
		{"game over ignores launch at first", func(g *Game) {
			g.Lives = 1
			g.Update(testDT, launch)
			lose(g)
			g.Update(testDT, launch)
		}, want{StateGameOver, 1, 0}},
		{"game over then launch starts afresh", func(g *Game) {
			g.Lives, g.Level = 1, 4
			g.Update(testDT, launch)
			lose(g)
			run(g, promptSteps, Input{})
			g.Update(testDT, launch)
		}, want{StateServe, 1, 3}},

		{"clearing the bricks ends the level", func(g *Game) {
			g.Update(testDT, launch)
			clearLevel(g)
		}, want{StateLevelClear, 1, 3}},
		{"level clear ignores launch at first", func(g *Game) {
			g.Update(testDT, launch)
			clearLevel(g)
			g.Update(testDT, launch)
		}, want{StateLevelClear, 1, 3}},
		{"level clear then launch serves the next level", func(g *Game) {
			g.Update(testDT, launch)
			clearLevel(g)
			run(g, promptSteps, Input{})
			g.Update(testDT, launch)
		}, want{StateServe, 2, 3}},
		{"clearing with the last ball going out still wins", func(g *Game) {
			g.Lives = 1
			g.Update(testDT, launch)
			g.Bricks = Grid{}
			lose(g)
		}, want{StateLevelClear, 1, 1}},

		{"pause from play", func(g *Game) {
			g.Update(testDT, launch)
			g.Update(testDT, pause)
		}, want{StatePaused, 1, 3}},
		{"pause is a toggle", func(g *Game) {
			g.Update(testDT, launch)
			g.Update(testDT, pause)
			g.Update(testDT, pause)
		}, want{StatePlaying, 1, 3}},
		{"pause from serve returns to serve", func(g *Game) {
			g.Update(testDT, pause)
			g.Update(testDT, pause)
		}, want{StateServe, 1, 3}},
		{"launch while paused is ignored", func(g *Game) {
			g.Update(testDT, pause)
			run(g, 3, launch)
		}, want{StatePaused, 1, 3}},
		{"pause on the game-over screen is ignored", func(g *Game) {
			g.Lives = 1
			g.Update(testDT, launch)
			lose(g)
			g.Update(testDT, pause)
		}, want{StateGameOver, 1, 0}},
		{"pause on the level-clear screen is ignored", func(g *Game) {
			g.Update(testDT, launch)
			clearLevel(g)
			g.Update(testDT, pause)
		}, want{StateLevelClear, 1, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New()
			tt.do(g)
			got := want{g.State, g.Level, g.Lives}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPauseFreezesEverything(t *testing.T) {
	g := playing()
	run(g, 10, Input{})
	g.Update(testDT, Input{Pause: true})
	before := *g

	run(g, 100, Input{PaddleX: 5, HasMouse: true, Move: 1, Launch: true})
	if *g != before {
		t.Errorf("state changed while paused:\n got %+v\nwant %+v", *g, before)
	}

	g.Update(testDT, Input{Pause: true})
	if g.Ball.Pos == before.Ball.Pos {
		t.Error("ball did not move after resuming")
	}
}

func TestServe(t *testing.T) {
	g := New()

	// The ball rides the paddle wherever it goes.
	for _, x := range []float64{20, 60, -100, 1000} {
		g.Update(testDT, Input{PaddleX: x, HasMouse: true})
		want := Vec{g.Paddle.X + serveOffset*g.Paddle.W/2, PaddleTop - ballHalf}
		if g.Ball.Pos != want || g.Ball.Vel != (Vec{}) {
			t.Errorf("paddle at %v: ball = %+v, want parked at %v", g.Paddle.X, g.Ball, want)
		}
	}

	g.Update(testDT, Input{Launch: true})
	if v := g.Ball.Vel; v.Y >= 0 || v.X <= 0 || !near(v.Len(), BallSpeed) {
		t.Errorf("launch velocity = %v, want up and to the right at speed %v", v, BallSpeed)
	}
}

func TestScoreAndSpeed(t *testing.T) {
	g := playing()
	g.Bricks = bricksAt([2]int{5, 3}, [2]int{19, 13})
	g.Ball = Ball{Vec{22, 12.6}, Vec{0, -BallSpeed}}
	g.Update(testDT, Input{})
	if g.Score != BrickScore {
		t.Errorf("score = %d, want %d", g.Score, BrickScore)
	}

	// Score survives a lost life and a level change, but not a new game.
	lose(g)
	if g.Score != BrickScore {
		t.Errorf("score after losing a life = %d, want %d", g.Score, BrickScore)
	}

	speeds := []struct {
		level int
		want  float64
	}{
		{1, BallSpeed},
		{2, BallSpeed * 1.08},
		{6, BallSpeed * 1.4},
		{11, BallSpeed * 1.8},
		{50, BallSpeed * 1.8}, // capped
	}
	for _, tt := range speeds {
		g.Level = tt.level
		if got := g.BallSpeed(); !near(got, tt.want) {
			t.Errorf("level %d speed = %v, want %v", tt.level, got, tt.want)
		}
	}
}
