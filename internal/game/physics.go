package game

import "math"

// Vec is a point or direction in world space.
type Vec struct{ X, Y float64 }

func (v Vec) Add(o Vec) Vec           { return Vec{v.X + o.X, v.Y + o.Y} }
func (v Vec) Scale(k float64) Vec     { return Vec{v.X * k, v.Y * k} }
func (v Vec) Len() float64            { return math.Hypot(v.X, v.Y) }
func clamp(v, lo, hi float64) float64 { return max(lo, min(v, hi)) }

// paddleBounce returns the velocity of a ball leaving the paddle. offset is
// where it struck, from -1 (left edge) to +1 (right edge); it maps linearly
// to an angle of up to MaxBounceAngle either side of straight up. Speed is
// preserved and the result always points upward.
func paddleBounce(offset, speed float64) Vec {
	a := clamp(offset, -1, 1) * MaxBounceAngle
	return Vec{X: speed * math.Sin(a), Y: -speed * math.Cos(a)}
}

// bounceWalls reflects a box of the given half-size off the left, right and
// top walls of a w-wide field, mirroring any overshoot back inside. The
// bottom is open.
func bounceWalls(pos, vel Vec, half, w float64) (Vec, Vec) {
	if pos.X < half {
		pos.X = 2*half - pos.X
		vel.X = math.Abs(vel.X)
	} else if pos.X > w-half {
		pos.X = 2*(w-half) - pos.X
		vel.X = -math.Abs(vel.X)
	}
	if pos.Y < half {
		pos.Y = 2*half - pos.Y
		vel.Y = math.Abs(vel.Y)
	}
	return pos, vel
}
