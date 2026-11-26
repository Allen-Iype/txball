package game

import (
	"math"
	"math/rand/v2"
	"testing"
)

// track plays for the given time with the paddle kept under the first
// ball, so nothing is lost.
func track(g *Game, seconds float64) {
	for range int(seconds / testDT) {
		g.Update(testDT, Input{PaddleX: g.Balls[0].Pos.X, HasMouse: true})
	}
}

func TestTimedPowerups(t *testing.T) {
	type snapshot struct {
		paddleW float64
		speed   float64
	}
	read := func(g *Game) snapshot { return snapshot{g.Paddle.W, g.Balls[0].Vel.Len()} }
	normal := snapshot{PaddleWidth, BallSpeed}

	tests := []struct {
		name     string
		powers   []PowerKind
		during   snapshot
		duration float64
	}{
		{"long paddle", []PowerKind{PowerLong}, snapshot{PaddleWidth * 1.5, BallSpeed}, 20},
		{"short paddle", []PowerKind{PowerShort}, snapshot{PaddleWidth * 0.6, BallSpeed}, 20},
		{"slow ball", []PowerKind{PowerSlow}, snapshot{PaddleWidth, BallSpeed * 0.6}, 15},
		{"short cancels long", []PowerKind{PowerLong, PowerShort}, snapshot{PaddleWidth * 0.6, BallSpeed}, 20},
		{"long cancels short", []PowerKind{PowerShort, PowerLong}, snapshot{PaddleWidth * 1.5, BallSpeed}, 20},
		{"slow and long combine", []PowerKind{PowerSlow, PowerLong}, snapshot{PaddleWidth * 1.5, BallSpeed * 0.6}, 20},
		{"fire leaves paddle and speed alone", []PowerKind{PowerFire}, normal, 10},
		{"sticky leaves paddle and speed alone", []PowerKind{PowerSticky}, normal, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := playing()
			g.Bricks = bricksAt([2]int{0, 0}) // one brick, well out of reach
			g.Balls[0] = Ball{Pos: Vec{40, 30}, Vel: Vec{0, -BallSpeed}}
			for _, k := range tt.powers {
				g.applyPower(k)
			}
			last := tt.powers[len(tt.powers)-1]
			if !g.Active(last) {
				t.Fatalf("%v not active after being applied", last)
			}

			g.Update(testDT, Input{})
			if got := read(g); !near(got.paddleW, tt.during.paddleW) || !near(got.speed, tt.during.speed) {
				t.Errorf("while active: %+v, want %+v", got, tt.during)
			}

			// Just before expiry it still holds; just after, all is normal.
			// The ball is pinned in mid-air so nothing else happens.
			hold := func(seconds float64) {
				for range int(seconds / testDT) {
					g.Balls[0].Pos = Vec{40, 30}
					g.Update(testDT, Input{})
				}
			}
			hold(tt.duration - 0.5)
			if !g.Active(last) {
				t.Errorf("%v expired early", last)
			}
			hold(1)
			for k, left := range g.Timers {
				if left != 0 {
					t.Errorf("timer %d still has %v left after %v s", k, left, tt.duration+0.5)
				}
			}
			if got := read(g); !near(got.paddleW, normal.paddleW) || !near(got.speed, normal.speed) {
				t.Errorf("after expiry: %+v, want %+v", got, normal)
			}
		})
	}
}

func TestPowerupTimerRestartsNotStacks(t *testing.T) {
	g := playing()
	g.applyPower(PowerFire)
	g.Timers[PowerFire] = 3
	g.applyPower(PowerFire)
	if got := g.Timers[PowerFire]; got != powerDuration[PowerFire] {
		t.Errorf("timer = %v, want a fresh %v", got, powerDuration[PowerFire])
	}
}

func TestTimersOnlyRunDuringPlay(t *testing.T) {
	g := playing()
	g.applyPower(PowerLong)
	g.Update(testDT, Input{Pause: true})
	run(g, 500, Input{})
	if got := g.Timers[PowerLong]; got != powerDuration[PowerLong] {
		t.Errorf("timer ran down to %v while paused", got)
	}
}

func TestExtraLife(t *testing.T) {
	tests := []struct{ lives, want int }{
		{1, 2},
		{3, 4},
		{MaxLives - 1, MaxLives},
		{MaxLives, MaxLives},
	}
	for _, tt := range tests {
		g := playing()
		g.Lives = tt.lives
		g.applyPower(PowerLife)
		if g.Lives != tt.want {
			t.Errorf("from %d lives: got %d, want %d", tt.lives, g.Lives, tt.want)
		}
	}
}

func TestFireball(t *testing.T) {
	// A ball rising through column 5 (x 20..24), rows 3 and 2.
	up := Vec{0, -BallSpeed}
	tests := []struct {
		name      string
		fire      bool
		bricks    map[[2]int]Brick
		steps     int
		wantVelY  float64
		wantGone  [][2]int
		wantLeft  [][2]int
		wantScore int
	}{
		{
			name:      "without fire the first brick turns the ball back",
			bricks:    map[[2]int]Brick{{5, 3}: {Kind: BrickNormal, Hits: 1}, {5, 2}: {Kind: BrickNormal, Hits: 1}},
			steps:     30,
			wantVelY:  BallSpeed,
			wantGone:  [][2]int{{5, 3}},
			wantLeft:  [][2]int{{5, 2}},
			wantScore: 10,
		},
		{
			name:      "fire passes through a stack",
			fire:      true,
			bricks:    map[[2]int]Brick{{5, 3}: {Kind: BrickNormal, Hits: 1}, {5, 2}: {Kind: BrickNormal, Hits: 1}},
			steps:     30,
			wantVelY:  -BallSpeed,
			wantGone:  [][2]int{{5, 3}, {5, 2}},
			wantScore: 20,
		},
		{
			name:      "fire destroys armour outright and scores every hit",
			fire:      true,
			bricks:    map[[2]int]Brick{{5, 3}: {Kind: BrickArmor, Hits: 3}, {5, 2}: {Kind: BrickArmor, Hits: 2}},
			steps:     30,
			wantVelY:  -BallSpeed,
			wantGone:  [][2]int{{5, 3}, {5, 2}},
			wantScore: 50,
		},
		{
			name:      "fire still bounces off solid bricks",
			fire:      true,
			bricks:    map[[2]int]Brick{{5, 3}: {Kind: BrickNormal, Hits: 1}, {5, 2}: {Kind: BrickSolid}},
			steps:     30,
			wantVelY:  BallSpeed,
			wantGone:  [][2]int{{5, 3}},
			wantLeft:  [][2]int{{5, 2}},
			wantScore: 10,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := playing()
			g.Bricks = bricksAt([2]int{19, 13})
			for cell, b := range tt.bricks {
				g.Bricks[cell[1]][cell[0]] = b
			}
			if tt.fire {
				g.applyPower(PowerFire)
			}
			g.Balls[0] = Ball{Pos: Vec{22, 13}, Vel: up}
			run(g, tt.steps, Input{})

			if got := g.Balls[0].Vel; !near(got.X, 0) || !near(got.Y, tt.wantVelY) {
				t.Errorf("velocity = %v, want {0 %v}", got, tt.wantVelY)
			}
			for _, c := range tt.wantGone {
				if g.Bricks.alive(c[0], c[1]) {
					t.Errorf("brick %v survived", c)
				}
			}
			for _, c := range tt.wantLeft {
				if !g.Bricks.alive(c[0], c[1]) {
					t.Errorf("brick %v was destroyed", c)
				}
			}
			if g.Score != tt.wantScore {
				t.Errorf("score = %d, want %d", g.Score, tt.wantScore)
			}
		})
	}
}

func TestStickyPaddle(t *testing.T) {
	land := func(g *Game, x float64) {
		g.Balls[0] = Ball{Pos: Vec{x, PaddleTop - ballHalf - 0.1}, Vel: Vec{0, BallSpeed}}
		g.Update(testDT, Input{})
	}

	t.Run("catches where the ball lands and carries it", func(t *testing.T) {
		g := playing()
		g.applyPower(PowerSticky)
		land(g, 43) // paddle spans 34..46, so halfway out to the right
		b := g.Balls[0]
		if !b.Stuck || !near(b.Offset, 0.5) || b.Vel != (Vec{}) {
			t.Fatalf("ball = %+v, want stuck at offset 0.5", b)
		}
		g.Update(testDT, Input{PaddleX: 20, HasMouse: true})
		if got, want := g.Balls[0].Pos, (Vec{23, PaddleTop - ballHalf}); !vecNear(got, want) {
			t.Errorf("after moving the paddle to 20: ball at %v, want %v", got, want)
		}
		run(g, 300, Input{})
		if !g.Balls[0].Stuck || g.State != StatePlaying {
			t.Errorf("ball should wait on the paddle indefinitely; got %+v in %v", g.Balls[0], g.State)
		}
	})

	t.Run("launch releases at the angle for that spot", func(t *testing.T) {
		g := playing()
		g.applyPower(PowerSticky)
		land(g, 43)
		g.Update(testDT, Input{Launch: true})
		b := g.Balls[0]
		want := paddleBounce(0.5, BallSpeed)
		if b.Stuck || !vecNear(b.Vel, want) {
			t.Errorf("ball = %+v, want released with velocity %v", b, want)
		}
	})

	t.Run("a stuck ball stays held after sticky runs out", func(t *testing.T) {
		g := playing()
		g.applyPower(PowerSticky)
		land(g, 40)
		g.Timers[PowerSticky] = 0
		run(g, 10, Input{})
		if !g.Balls[0].Stuck {
			t.Error("ball was released without a launch")
		}
		g.Update(testDT, Input{Launch: true})
		land(g, 40)
		if g.Balls[0].Stuck {
			t.Error("ball stuck again with sticky expired")
		}
	})

	t.Run("without sticky the ball bounces", func(t *testing.T) {
		g := playing()
		land(g, 43)
		if b := g.Balls[0]; b.Stuck || b.Vel.Y >= 0 {
			t.Errorf("ball = %+v, want bounced", b)
		}
	})

	t.Run("a stuck ball stays on a shrinking paddle", func(t *testing.T) {
		g := playing()
		g.applyPower(PowerSticky)
		land(g, 45.5)
		g.applyPower(PowerShort)
		g.Update(testDT, Input{})
		if x := g.Balls[0].Pos.X; x < g.Paddle.Left() || x > g.Paddle.Right() {
			t.Errorf("ball at x=%v is off the paddle %v..%v", x, g.Paddle.Left(), g.Paddle.Right())
		}
	})
}

func TestMultiBall(t *testing.T) {
	flying := func(n int) []Ball {
		balls := make([]Ball, n)
		for i := range balls {
			balls[i] = Ball{Pos: Vec{10 + float64(i)*5, 30}, Vel: paddleBounce(0.2, BallSpeed)}
		}
		return balls
	}
	tests := []struct {
		name  string
		balls []Ball
		want  int
	}{
		{"one becomes three", flying(1), 3},
		{"three become nine", flying(3), 9},
		{"capped at twelve", flying(5), MaxBalls},
		{"already at the cap", flying(MaxBalls), MaxBalls},
		{"a stuck ball launches two and stays", []Ball{{Stuck: true, Offset: 0.1, Pos: Vec{40, 39.5}}}, 3},
		{"steep and shallow headings", []Ball{
			{Pos: Vec{40, 20}, Vel: paddleBounce(1, BallSpeed)},
			{Pos: Vec{40, 20}, Vel: paddleBounce(-1, BallSpeed).Scale(-1)},
		}, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := playing()
			g.Balls = tt.balls
			orig := append([]Ball(nil), tt.balls...)
			g.applyPower(PowerMulti)

			if len(g.Balls) != tt.want {
				t.Fatalf("balls = %d, want %d", len(g.Balls), tt.want)
			}
			for i, b := range orig {
				if g.Balls[i] != b {
					t.Errorf("original ball %d changed: %+v, want %+v", i, g.Balls[i], b)
				}
			}
			seen := map[Vec]bool{}
			for i, b := range g.Balls[len(orig):] {
				if b.Stuck {
					t.Errorf("new ball %d is stuck", i)
				}
				if !near(b.Vel.Len(), BallSpeed) {
					t.Errorf("new ball %d speed = %v, want %v", i, b.Vel.Len(), BallSpeed)
				}
				if math.Abs(b.Vel.Y) < BallSpeed*minVertical-eps {
					t.Errorf("new ball %d is too flat: %v", i, b.Vel)
				}
				if len(orig) == 1 && seen[b.Vel] {
					t.Errorf("new ball %d duplicates another heading %v", i, b.Vel)
				}
				seen[b.Vel] = true
			}
		})
	}
}

// A life is lost only when the last ball goes.
func TestMultiBallLifeAccounting(t *testing.T) {
	out := Ball{Pos: Vec{40, Height + 1}, Vel: Vec{0, BallSpeed}}
	in := Ball{Pos: Vec{40, 30}, Vel: Vec{0, -BallSpeed}}

	tests := []struct {
		name      string
		balls     []Ball
		wantBalls int
		wantLives int
		wantState State
	}{
		{"one of three lost", []Ball{in, out, in}, 2, 3, StatePlaying},
		{"two of three lost", []Ball{out, in, out}, 1, 3, StatePlaying},
		{"all three lost at once costs one life", []Ball{out, out, out}, 1, 2, StateServe},
		{"the only ball lost", []Ball{out}, 1, 2, StateServe},
		{"a stuck ball counts as in play", []Ball{out, {Stuck: true}}, 1, 3, StatePlaying},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := playing()
			g.Balls = append([]Ball(nil), tt.balls...)
			g.Update(testDT, Input{})
			if len(g.Balls) != tt.wantBalls || g.Lives != tt.wantLives || g.State != tt.wantState {
				t.Errorf("balls=%d lives=%d state=%v, want balls=%d lives=%d state=%v",
					len(g.Balls), g.Lives, g.State, tt.wantBalls, tt.wantLives, tt.wantState)
			}
		})
	}
}

func TestCapsules(t *testing.T) {
	// The paddle is centred on 40: it spans 34..46 across and
	// PaddleTop..PaddleTop+PaddleHeight down. Capsules are 3x2.
	const step = PowerFallSpeed * testDT
	tests := []struct {
		name       string
		pos        Vec
		wantCaught bool
		wantGone   bool
	}{
		{"falling in open space", Vec{40, 20}, false, false},
		{"touches the paddle top", Vec{40, PaddleTop - 1 + 0.01}, true, true},
		{"just short of the paddle top", Vec{40, PaddleTop - 1 - 2*step}, false, false},
		{"clips the left end", Vec{32.6, PaddleTop}, true, true},
		{"just outside the left end", Vec{32.4, PaddleTop}, false, false},
		{"clips the right end", Vec{47.4, PaddleTop}, true, true},
		{"just outside the right end", Vec{47.6, PaddleTop}, false, false},
		{"beside the paddle, below its top", Vec{20, PaddleTop + 1}, false, false},
		{"off the bottom", Vec{20, Height + 1 - step/2}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := playing()
			g.Balls[0] = Ball{Pos: Vec{70, 20}, Vel: Vec{0, -BallSpeed}}
			g.Powerups = []Powerup{{PowerLife, tt.pos}}
			g.Update(testDT, Input{})

			if caught := g.Lives == StartLives+1; caught != tt.wantCaught {
				t.Errorf("caught = %v, want %v", caught, tt.wantCaught)
			}
			if gone := len(g.Powerups) == 0; gone != tt.wantGone {
				t.Errorf("removed = %v, want %v", gone, tt.wantGone)
			}
			if !tt.wantGone {
				if got := g.Powerups[0].Pos; !vecNear(got, Vec{tt.pos.X, tt.pos.Y + step}) {
					t.Errorf("capsule at %v, want it %v lower than %v", got, step, tt.pos)
				}
			}
		})
	}
}

func TestCapsulesKeepFallingWhileOthersAreCaught(t *testing.T) {
	g := playing()
	g.Balls[0] = Ball{Pos: Vec{70, 20}, Vel: Vec{0, -BallSpeed}}
	g.Powerups = []Powerup{
		{PowerLong, Vec{10, 10}},
		{PowerLife, Vec{40, PaddleTop}},
		{PowerFire, Vec{60, 12}},
		{PowerSlow, Vec{40, PaddleTop}},
	}
	g.Update(testDT, Input{})
	if len(g.Powerups) != 2 || g.Powerups[0].Kind != PowerLong || g.Powerups[1].Kind != PowerFire {
		t.Errorf("capsules left = %+v, want the long and fire ones", g.Powerups)
	}
	if g.Lives != StartLives+1 || !g.Active(PowerSlow) || g.Active(PowerLong) || g.Active(PowerFire) {
		t.Errorf("lives=%d timers=%v: want exactly life and slow applied", g.Lives, g.Timers)
	}
}

func TestDrops(t *testing.T) {
	newRNG := func() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) }
	strike := func(g *Game) {
		g.Balls[0] = Ball{Pos: Vec{22, 12.6}, Vel: Vec{0, -BallSpeed}}
		g.Update(testDT, Input{})
	}

	tests := []struct {
		name   string
		rng    *rand.Rand
		chance float64
		brick  Brick
		want   int
	}{
		{"certain drop from a destroyed brick", newRNG(), 1, Brick{Kind: BrickNormal, Hits: 1}, 1},
		{"zero chance never drops", newRNG(), 0, Brick{Kind: BrickNormal, Hits: 1}, 0},
		{"no rng never drops", nil, 1, Brick{Kind: BrickNormal, Hits: 1}, 0},
		{"damaged armour does not drop", newRNG(), 1, Brick{Kind: BrickArmor, Hits: 2}, 0},
		{"armour's last hit drops", newRNG(), 1, Brick{Kind: BrickArmor, Hits: 1}, 1},
		{"solid never drops", newRNG(), 1, Brick{Kind: BrickSolid}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New([]Grid{bricksAt([2]int{19, 13})}, tt.rng)
			g.DropChance = tt.chance
			g.Update(testDT, Input{Launch: true})
			g.Bricks[3][5] = tt.brick
			strike(g)

			if len(g.Powerups) != tt.want {
				t.Fatalf("capsules = %d, want %d", len(g.Powerups), tt.want)
			}
			if tt.want == 1 {
				// Brick (5,3) is x 20..24, y 10..12; the capsule has
				// had this one step to start falling from its centre.
				want := Vec{22, 11}
				if got := g.Powerups[0].Pos; math.Abs(got.X-want.X) > eps || got.Y < want.Y || got.Y > want.Y+1 {
					t.Errorf("capsule at %v, want it starting from %v", got, want)
				}
			}
		})
	}

	t.Run("fireball kills drop too", func(t *testing.T) {
		g := New([]Grid{bricksAt([2]int{19, 13})}, newRNG())
		g.DropChance = 1
		g.Update(testDT, Input{Launch: true})
		g.Bricks[3][5] = Brick{Kind: BrickArmor, Hits: 3}
		g.applyPower(PowerFire)
		strike(g)
		if len(g.Powerups) != 1 {
			t.Errorf("capsules = %d, want 1", len(g.Powerups))
		}
	})
}

func TestPickPowerFollowsWeights(t *testing.T) {
	g := New([]Grid{demoGrid()}, rand.New(rand.NewPCG(7, 11)))
	const draws = 16000
	var counts [NumPowers]int
	for range draws {
		counts[g.pickPower()]++
	}
	total := 0
	for _, w := range powerWeight {
		total += w
	}
	for k, w := range powerWeight {
		want := float64(draws) * float64(w) / float64(total)
		if got := float64(counts[k]); math.Abs(got-want) > want*0.1 {
			t.Errorf("kind %d drawn %v times, want about %.0f", k, got, want)
		}
	}
}

func TestRoundEndClearsPowerups(t *testing.T) {
	setup := func() *Game {
		g := playing()
		g.Balls = append(g.Balls, Ball{Pos: Vec{30, 30}, Vel: Vec{0, -BallSpeed}})
		g.Powerups = []Powerup{{PowerFire, Vec{10, 10}}}
		for _, k := range []PowerKind{PowerLong, PowerFire, PowerSticky, PowerSlow} {
			g.applyPower(k)
		}
		g.Update(testDT, Input{})
		if g.Paddle.W == PaddleWidth {
			t.Fatal("setup: long paddle not in effect")
		}
		return g
	}
	check := func(t *testing.T, g *Game, wantBalls int) {
		t.Helper()
		if len(g.Powerups) != 0 {
			t.Errorf("capsules left: %+v", g.Powerups)
		}
		if g.Timers != ([NumPowers]float64{}) {
			t.Errorf("timers left: %v", g.Timers)
		}
		if g.Paddle.W != PaddleWidth {
			t.Errorf("paddle width = %v, want %v", g.Paddle.W, PaddleWidth)
		}
		if len(g.Balls) != wantBalls {
			t.Errorf("balls = %d, want %d", len(g.Balls), wantBalls)
		}
	}

	t.Run("life lost", func(t *testing.T) {
		g := setup()
		lose(g)
		check(t, g, 1)
	})
	t.Run("level cleared", func(t *testing.T) {
		g := setup()
		clearLevel(g)
		check(t, g, 0)
		run(g, promptSteps, Input{})
		g.Update(testDT, Input{Launch: true})
		check(t, g, 1)
	})
	t.Run("game over", func(t *testing.T) {
		g := setup()
		g.Lives = 1
		lose(g)
		check(t, g, 0)
	})
}

func TestSteepen(t *testing.T) {
	const s = 10.0
	flatX := s * math.Sqrt(1-minVertical*minVertical)
	tests := []struct {
		name    string
		v, want Vec
	}{
		{"steep enough is untouched", Vec{6, -8}, Vec{6, -8}},
		{"straight up is untouched", Vec{0, -s}, Vec{0, -s}},
		{"exactly at the limit is untouched", Vec{flatX, -s * minVertical}, Vec{flatX, -s * minVertical}},
		{"too flat, rising right", Vec{s * 0.995, -s * 0.0998749}, Vec{flatX, -s * minVertical}},
		{"too flat, rising left", Vec{-s * 0.995, -s * 0.0998749}, Vec{-flatX, -s * minVertical}},
		{"too flat, falling right", Vec{s * 0.995, s * 0.0998749}, Vec{flatX, s * minVertical}},
		{"dead horizontal goes up", Vec{-s, 0}, Vec{-flatX, -s * minVertical}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := steepen(tt.v)
			if math.Abs(got.X-tt.want.X) > 1e-6 || math.Abs(got.Y-tt.want.Y) > 1e-6 {
				t.Errorf("steepen(%v) = %v, want %v", tt.v, got, tt.want)
			}
			if math.Abs(got.Len()-tt.v.Len()) > 1e-6 {
				t.Errorf("speed changed from %v to %v", tt.v.Len(), got.Len())
			}
		})
	}
}

func TestRotate(t *testing.T) {
	tests := []struct {
		v    Vec
		a    float64
		want Vec
	}{
		{Vec{0, -1}, math.Pi / 2, Vec{1, 0}},   // up turns to the right
		{Vec{0, -1}, -math.Pi / 2, Vec{-1, 0}}, // and to the left
		{Vec{3, 4}, math.Pi, Vec{-3, -4}},
		{Vec{3, 4}, 0, Vec{3, 4}},
	}
	for _, tt := range tests {
		if got := rotate(tt.v, tt.a); !vecNear(got, tt.want) {
			t.Errorf("rotate(%v, %v) = %v, want %v", tt.v, tt.a, got, tt.want)
		}
	}
}

// Ten simulated minutes of sloppy play with every brick dropping a
// power-up: nothing may panic and the game's invariants must hold.
func TestPowerupSoak(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 9))
	g := New([]Grid{demoGrid(), bricksAt([2]int{3, 3}, [2]int{4, 3}, [2]int{10, 6})}, rng)
	g.DropChance = 1
	caught := 0

	for i := range 120 * 600 {
		in := Input{Launch: i%40 == 0}
		if len(g.Balls) > 0 {
			// Follow a ball, badly, so that some are lost.
			in.PaddleX, in.HasMouse = g.Balls[0].Pos.X+8*math.Sin(float64(i)/300), true
		}
		before := g.Timers
		g.Update(testDT, in)
		if g.Timers != before && g.State == StatePlaying {
			for k := range g.Timers {
				if g.Timers[k] > before[k] {
					caught++
				}
			}
		}

		if n := len(g.Balls); n > MaxBalls {
			t.Fatalf("tick %d: %d balls", i, n)
		}
		if g.Lives < 0 || g.Lives > MaxLives {
			t.Fatalf("tick %d: %d lives", i, g.Lives)
		}
		if g.Active(PowerLong) && g.Active(PowerShort) {
			t.Fatalf("tick %d: long and short both active", i)
		}
		if g.Paddle.Left() < -eps || g.Paddle.Right() > Width+eps {
			t.Fatalf("tick %d: paddle %v..%v outside the field", i, g.Paddle.Left(), g.Paddle.Right())
		}
		for _, b := range g.Balls {
			if b.Pos.X < ballHalf-eps || b.Pos.X > Width-ballHalf+eps || b.Pos.Y < ballHalf-eps {
				t.Fatalf("tick %d: ball outside the walls at %v", i, b.Pos)
			}
			// A slow capsule caught this tick takes hold on the next one.
			slowJustCaught := g.Timers[PowerSlow] > before[PowerSlow]
			if !b.Stuck && !slowJustCaught && !near(b.Vel.Len(), g.speed()) {
				t.Fatalf("tick %d: ball speed %v, want %v", i, b.Vel.Len(), g.speed())
			}
		}
		if g.State == StatePlaying && len(g.Balls) == 0 {
			t.Fatalf("tick %d: playing with no balls", i)
		}
	}
	if caught < 10 || g.Level < 2 {
		t.Errorf("soak was too quiet to mean much: %d timed power-ups caught, reached level %d", caught, g.Level)
	}
}
