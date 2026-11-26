package game

import "math"

// PowerKind identifies a power-up.
type PowerKind uint8

const (
	// PowerMulti splits every ball in play into three.
	PowerMulti PowerKind = iota
	// PowerLong widens the paddle; it cancels PowerShort.
	PowerLong
	// PowerShort narrows the paddle; it cancels PowerLong.
	PowerShort
	// PowerFire lets balls plough through breakable bricks.
	PowerFire
	// PowerSticky makes the paddle catch balls until they are launched.
	PowerSticky
	// PowerSlow slows every ball.
	PowerSlow
	// PowerLife grants an extra life.
	PowerLife

	// NumPowers is the number of power-up kinds.
	NumPowers = iota
)

// Power-up tuning.
const (
	PowerW         = 3.0
	PowerH         = 2.0
	PowerFallSpeed = 14.0
	// PowerDropChance is the default chance that a destroyed brick drops
	// a power-up.
	PowerDropChance = 0.15

	LongFactor  = 1.5
	ShortFactor = 0.6
	SlowFactor  = 0.6

	MaxBalls = 12
	MaxLives = 9

	// splitAngle is how far, in radians, multi-ball fans the new balls
	// out either side of the original.
	splitAngle = 0.4
	// minVertical is the least share of a split ball's speed that must be
	// vertical, so it can't end up crawling sideways across the field.
	minVertical = 0.26
)

// powerDuration is how long each timed effect lasts, in seconds. Instant
// power-ups have no entry.
var powerDuration = [NumPowers]float64{
	PowerLong:   20,
	PowerShort:  20,
	PowerFire:   10,
	PowerSticky: 20,
	PowerSlow:   15,
}

// powerWeight is each kind's relative chance of being the one dropped.
var powerWeight = [NumPowers]int{
	PowerMulti:  3,
	PowerLong:   3,
	PowerShort:  2,
	PowerFire:   2,
	PowerSticky: 2,
	PowerSlow:   3,
	PowerLife:   1,
}

// Powerup is a capsule falling towards the paddle. Pos is its centre.
type Powerup struct {
	Kind PowerKind
	Pos  Vec
}

// Active reports whether a timed effect is running.
func (g *Game) Active(k PowerKind) bool { return g.Timers[k] > 0 }

// applyPower gives the player power-up k.
func (g *Game) applyPower(k PowerKind) {
	switch k {
	case PowerMulti:
		g.splitBalls()
	case PowerLife:
		g.Lives = min(g.Lives+1, MaxLives)
	case PowerLong:
		g.Timers[PowerShort] = 0
		g.Timers[k] = powerDuration[k]
	case PowerShort:
		g.Timers[PowerLong] = 0
		g.Timers[k] = powerDuration[k]
	default:
		g.Timers[k] = powerDuration[k]
	}
}

// tickTimers runs down the timed effects.
func (g *Game) tickTimers(dt float64) {
	for k := range g.Timers {
		g.Timers[k] = max(g.Timers[k]-dt, 0)
	}
}

// paddleWidth returns the paddle's width under the current effects.
func (g *Game) paddleWidth() float64 {
	switch {
	case g.Active(PowerLong):
		return PaddleWidth * LongFactor
	case g.Active(PowerShort):
		return PaddleWidth * ShortFactor
	}
	return PaddleWidth
}

// speed returns the speed balls travel at right now.
func (g *Game) speed() float64 {
	if g.Active(PowerSlow) {
		return g.BallSpeed() * SlowFactor
	}
	return g.BallSpeed()
}

// splitBalls turns each ball into three, up to MaxBalls. A ball in flight
// gains two copies fanned out either side of its heading; a ball stuck to
// the paddle stays put and launches its two copies.
func (g *Game) splitBalls() {
	for _, b := range g.Balls { // ranges over the balls present at the start
		for _, side := range [2]float64{-1, 1} {
			if len(g.Balls) >= MaxBalls {
				return
			}
			nb := Ball{Pos: b.Pos}
			if b.Stuck {
				nb.Vel = paddleBounce(b.Offset+side*0.5, g.speed())
			} else {
				nb.Vel = steepen(rotate(b.Vel, side*splitAngle))
			}
			g.Balls = append(g.Balls, nb)
		}
	}
}

// maybeDrop rolls for a power-up from the brick just destroyed at (c, r).
func (g *Game) maybeDrop(c, r int) {
	if g.rng == nil || g.rng.Float64() >= g.DropChance {
		return
	}
	box := BrickRect(c, r)
	g.Powerups = append(g.Powerups, Powerup{
		Kind: g.pickPower(),
		Pos:  Vec{(box.Min.X + box.Max.X) / 2, (box.Min.Y + box.Max.Y) / 2},
	})
}

// pickPower chooses a kind according to powerWeight.
func (g *Game) pickPower() PowerKind {
	total := 0
	for _, w := range powerWeight {
		total += w
	}
	n := g.rng.IntN(total)
	for k, w := range powerWeight {
		if n < w {
			return PowerKind(k)
		}
		n -= w
	}
	panic("unreachable")
}

// stepPowerups lets the capsules fall, applies the ones the paddle catches
// and discards the ones that reach the bottom.
func (g *Game) stepPowerups(dt float64) {
	p := g.Paddle
	kept := g.Powerups[:0]
	for _, pu := range g.Powerups {
		pu.Pos.Y += PowerFallSpeed * dt
		caught := pu.Pos.X+PowerW/2 > p.Left() && pu.Pos.X-PowerW/2 < p.Right() &&
			pu.Pos.Y+PowerH/2 > PaddleTop && pu.Pos.Y-PowerH/2 < PaddleTop+PaddleHeight
		switch {
		case caught:
			g.Events |= EventPowerup
			g.applyPower(pu.Kind)
		case pu.Pos.Y-PowerH/2 < Height:
			kept = append(kept, pu)
		}
	}
	g.Powerups = kept
}

// rotate turns v clockwise on screen by a radians.
func rotate(v Vec, a float64) Vec {
	sin, cos := math.Sincos(a)
	return Vec{v.X*cos - v.Y*sin, v.X*sin + v.Y*cos}
}

// steepen returns v, tilted if necessary so that at least minVertical of
// its speed is vertical. Speed and the direction of travel on each axis
// are kept; a perfectly horizontal v is sent upward.
func steepen(v Vec) Vec {
	speed := v.Len()
	if math.Abs(v.Y) >= speed*minVertical {
		return v
	}
	up := -1.0
	if v.Y > 0 {
		up = 1
	}
	return Vec{
		X: math.Copysign(speed*math.Sqrt(1-minVertical*minVertical), v.X),
		Y: up * speed * minVertical,
	}
}
