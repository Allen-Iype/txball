package game

import "math"

// Vec is a point or direction in world space.
type Vec struct{ X, Y float64 }

func (v Vec) Add(o Vec) Vec       { return Vec{v.X + o.X, v.Y + o.Y} }
func (v Vec) Scale(k float64) Vec { return Vec{v.X * k, v.Y * k} }
func (v Vec) Len() float64        { return math.Hypot(v.X, v.Y) }

// AABB is an axis-aligned box.
type AABB struct{ Min, Max Vec }

func clamp(v, lo, hi float64) float64 { return max(lo, min(v, hi)) }

// paddleBounce returns the velocity of a ball leaving the paddle. offset is
// where it struck, from -1 (left edge) to +1 (right edge); it maps linearly
// to an angle of up to MaxBounceAngle either side of straight up. Speed is
// preserved and the result always points upward.
func paddleBounce(offset, speed float64) Vec {
	a := clamp(offset, -1, 1) * MaxBounceAngle
	return Vec{X: speed * math.Sin(a), Y: -speed * math.Cos(a)}
}

// reflect bounces v off a surface with axis-aligned normal n. A corner
// normal (both components set) turns the velocity back on both axes. The
// result always points along n, so reflecting twice can't re-enter.
func reflect(v, n Vec) Vec {
	if n.X != 0 {
		v.X = math.Copysign(v.X, n.X)
	}
	if n.Y != 0 {
		v.Y = math.Copysign(v.Y, n.Y)
	}
	return v
}

// sweepAABB moves a square of half-extent half, centred on pos, along the
// displacement d and reports its first contact with box b.
//
// t is the fraction of d travelled at contact, in [0, 1], and n is the
// normal of the face that was struck: one axis for a face, both for an
// exact corner. Boxes that merely touch or slide along each other do not
// collide, and neither does a square that starts out overlapping b: it is
// left free to move out.
//
// The square is reduced to a point by growing b by half on every side
// (the Minkowski sum), which turns the test into a ray/slab intersection
// and makes it exact however long d is.
func sweepAABB(pos Vec, half float64, d Vec, b AABB) (t float64, n Vec, ok bool) {
	enterX, exitX, okX := slab(pos.X, d.X, b.Min.X-half, b.Max.X+half)
	enterY, exitY, okY := slab(pos.Y, d.Y, b.Min.Y-half, b.Max.Y+half)
	if !okX || !okY {
		return 0, Vec{}, false
	}
	enter, exit := max(enterX, enterY), min(exitX, exitY)
	if enter >= exit || enter < 0 || enter > 1 {
		return 0, Vec{}, false
	}
	if enterX >= enterY {
		n.X = -math.Copysign(1, d.X)
	}
	if enterY >= enterX {
		n.Y = -math.Copysign(1, d.Y)
	}
	return enter, n, true
}

// slab returns the parametric interval over which p+t*d lies strictly
// between lo and hi. ok is false when a stationary p is not inside.
func slab(p, d, lo, hi float64) (enter, exit float64, ok bool) {
	if d == 0 {
		return math.Inf(-1), math.Inf(1), p > lo && p < hi
	}
	t1, t2 := (lo-p)/d, (hi-p)/d
	return min(t1, t2), max(t1, t2), true
}

// sweepWalls is sweepAABB against the inside of the field: the left, right
// and top walls of a w-wide playfield. The bottom is open. A square that
// has drifted outside a wall and is still heading out is turned back at
// t=0.
func sweepWalls(pos Vec, half float64, d Vec, w float64) (t float64, n Vec, ok bool) {
	t = math.Inf(1)
	// cross records a wall at limit that the move p -> p+dp ends beyond.
	cross := func(p, dp, limit float64, normal Vec) {
		switch tt := clamp((limit-p)/dp, 0, 1); {
		case tt < t:
			t, n = tt, normal
		case tt == t:
			n = n.Add(normal) // corner
		}
		ok = true
	}
	if d.X < 0 && pos.X+d.X < half {
		cross(pos.X, d.X, half, Vec{X: 1})
	}
	if d.X > 0 && pos.X+d.X > w-half {
		cross(pos.X, d.X, w-half, Vec{X: -1})
	}
	if d.Y < 0 && pos.Y+d.Y < half {
		cross(pos.Y, d.Y, half, Vec{Y: 1})
	}
	if !ok {
		return 0, Vec{}, false
	}
	return t, n, true
}
