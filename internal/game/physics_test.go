package game

import (
	"math"
	"testing"
)

const eps = 1e-9

func near(a, b float64) bool { return math.Abs(a-b) < eps }

func vecNear(a, b Vec) bool { return near(a.X, b.X) && near(a.Y, b.Y) }

func TestPaddleBounce(t *testing.T) {
	const speed = 40.0
	sin60, cos60 := math.Sin(math.Pi/3), math.Cos(math.Pi/3)
	sin30, cos30 := math.Sin(math.Pi/6), math.Cos(math.Pi/6)

	tests := []struct {
		name   string
		offset float64
		want   Vec
	}{
		{"centre goes straight up", 0, Vec{0, -speed}},
		{"right edge is 60 degrees right", 1, Vec{speed * sin60, -speed * cos60}},
		{"left edge is 60 degrees left", -1, Vec{-speed * sin60, -speed * cos60}},
		{"halfway right is 30 degrees", 0.5, Vec{speed * sin30, -speed * cos30}},
		{"halfway left is 30 degrees", -0.5, Vec{-speed * sin30, -speed * cos30}},
		{"beyond right edge clamps", 1.4, Vec{speed * sin60, -speed * cos60}},
		{"beyond left edge clamps", -3, Vec{-speed * sin60, -speed * cos60}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := paddleBounce(tt.offset, speed)
			if !vecNear(got, tt.want) {
				t.Errorf("paddleBounce(%v) = %v, want %v", tt.offset, got, tt.want)
			}
			if !near(got.Len(), speed) {
				t.Errorf("speed = %v, want %v", got.Len(), speed)
			}
			if got.Y >= 0 {
				t.Errorf("Y velocity = %v, want upward", got.Y)
			}
		})
	}
}

func TestSweepAABB(t *testing.T) {
	// A 4x2 brick at (10,10)-(14,12) and a 1x1 ball: the ball's centre
	// touches it on the box (9.5,9.5)-(14.5,12.5).
	box := AABB{Vec{10, 10}, Vec{14, 12}}
	const half = 0.5

	tests := []struct {
		name   string
		pos, d Vec
		wantOK bool
		wantT  float64
		wantN  Vec
	}{
		// Each face, hit square on.
		{"bottom face from below", Vec{12, 14.5}, Vec{0, -4}, true, 0.5, Vec{0, 1}},
		{"top face from above", Vec{12, 7.5}, Vec{0, 4}, true, 0.5, Vec{0, -1}},
		{"left face from the left", Vec{7.5, 11}, Vec{4, 0}, true, 0.5, Vec{-1, 0}},
		{"right face from the right", Vec{16.5, 11}, Vec{-4, 0}, true, 0.5, Vec{1, 0}},

		// Diagonal approaches: the axis that enters last is the face hit.
		{"diagonal onto bottom face", Vec{11, 13.5}, Vec{1, -2}, true, 0.5, Vec{0, 1}},
		{"diagonal onto left face", Vec{8.5, 11.5}, Vec{2, -0.5}, true, 0.5, Vec{-1, 0}},
		{"diagonal onto top face", Vec{13, 8.5}, Vec{-1, 2}, true, 0.5, Vec{0, -1}},
		{"diagonal onto right face", Vec{15.5, 10.5}, Vec{-2, 0.5}, true, 0.5, Vec{1, 0}},
		{"side hit near the bottom is still a side hit", Vec{8.5, 12.4}, Vec{2, -0.2}, true, 0.5, Vec{-1, 0}},
		{"underside hit near the corner is still the underside", Vec{9.6, 13.5}, Vec{0.2, -2}, true, 0.5, Vec{0, 1}},

		// Exact corners report both axes.
		{"bottom-left corner", Vec{8.5, 13.5}, Vec{2, -2}, true, 0.5, Vec{-1, 1}},
		{"top-right corner", Vec{15.5, 8.5}, Vec{-2, 2}, true, 0.5, Vec{1, -1}},

		// Contact timing.
		{"contact at the very end of the move", Vec{12, 14.5}, Vec{0, -2}, true, 1, Vec{0, 1}},
		{"already touching and moving in", Vec{12, 12.5}, Vec{0, -1}, true, 0, Vec{0, 1}},
		{"stops short", Vec{12, 14.5}, Vec{0, -1.9}, false, 0, Vec{}},

		// Speed: a move far longer than the brick is thick cannot skip it.
		{"very fast straight through", Vec{12, 112.5}, Vec{0, -1000}, true, 0.1, Vec{0, 1}},
		{"very fast diagonal", Vec{-87.5, 111}, Vec{1000, -1000}, true, 0.0985, Vec{0, 1}},

		// Misses.
		{"passes beside", Vec{15, 14.5}, Vec{0, -4}, false, 0, Vec{}},
		{"diagonal clears the corner", Vec{8, 14}, Vec{1, -5}, false, 0, Vec{}},
		{"moving away", Vec{12, 14.5}, Vec{0, 4}, false, 0, Vec{}},
		{"touching and moving away", Vec{12, 12.5}, Vec{0, 1}, false, 0, Vec{}},
		{"sliding along the bottom face", Vec{8, 12.5}, Vec{10, 0}, false, 0, Vec{}},
		{"sliding along the left face", Vec{9.5, 8}, Vec{0, 10}, false, 0, Vec{}},
		{"grazing a corner diagonally", Vec{8.5, 11.5}, Vec{2, 2}, false, 0, Vec{}},
		{"stationary outside", Vec{12, 14.5}, Vec{}, false, 0, Vec{}},

		// Starting inside is not a collision: the ball is let out.
		{"overlapping and moving", Vec{12, 11}, Vec{0, -4}, false, 0, Vec{}},
		{"overlapping and stationary", Vec{12, 11}, Vec{}, false, 0, Vec{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotT, gotN, ok := sweepAABB(tt.pos, half, tt.d, box)
			if ok != tt.wantOK {
				t.Fatalf("hit = %v (t=%v n=%v), want %v", ok, gotT, gotN, tt.wantOK)
			}
			if !ok {
				return
			}
			if !near(gotT, tt.wantT) {
				t.Errorf("t = %v, want %v", gotT, tt.wantT)
			}
			if gotN != tt.wantN {
				t.Errorf("normal = %v, want %v", gotN, tt.wantN)
			}
		})
	}
}

func TestSweepWalls(t *testing.T) {
	const (
		half = 0.5
		w    = 80.0
	)
	tests := []struct {
		name   string
		pos, d Vec
		wantOK bool
		wantT  float64
		wantN  Vec
	}{
		{"open space", Vec{40, 20}, Vec{3, -4}, false, 0, Vec{}},
		{"stops exactly on the left wall", Vec{2.5, 20}, Vec{-2, 0}, false, 0, Vec{}},
		{"left wall", Vec{1.5, 20}, Vec{-2, 1}, true, 0.5, Vec{1, 0}},
		{"right wall", Vec{78.5, 20}, Vec{2, 1}, true, 0.5, Vec{-1, 0}},
		{"top wall", Vec{40, 1.5}, Vec{1, -2}, true, 0.5, Vec{0, 1}},
		{"top-left corner", Vec{1.5, 1.5}, Vec{-2, -2}, true, 0.5, Vec{1, 1}},
		{"top-right corner", Vec{78.5, 1.5}, Vec{2, -2}, true, 0.5, Vec{-1, 1}},
		{"side wall before top wall", Vec{1.5, 3.5}, Vec{-2, -4}, true, 0.5, Vec{1, 0}},
		{"top wall before side wall", Vec{3.5, 1.5}, Vec{-4, -2}, true, 0.5, Vec{0, 1}},
		{"bottom is open", Vec{40, 43}, Vec{0, 50}, false, 0, Vec{}},
		{"very fast into the right wall", Vec{40, 20}, Vec{790, 0}, true, 0.05, Vec{-1, 0}},
		{"on the wall, moving along it", Vec{0.5, 20}, Vec{0, -3}, false, 0, Vec{}},
		{"on the wall, moving away", Vec{0.5, 20}, Vec{3, 0}, false, 0, Vec{}},
		{"outside and heading further out is turned back at once", Vec{0.2, 20}, Vec{-1, 0}, true, 0, Vec{1, 0}},
		{"outside but heading back in is left alone", Vec{0.2, 20}, Vec{1, 0}, false, 0, Vec{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotT, gotN, ok := sweepWalls(tt.pos, half, tt.d, w)
			if ok != tt.wantOK {
				t.Fatalf("hit = %v (t=%v n=%v), want %v", ok, gotT, gotN, tt.wantOK)
			}
			if !ok {
				return
			}
			if !near(gotT, tt.wantT) {
				t.Errorf("t = %v, want %v", gotT, tt.wantT)
			}
			if gotN != tt.wantN {
				t.Errorf("normal = %v, want %v", gotN, tt.wantN)
			}
		})
	}
}

func TestReflect(t *testing.T) {
	tests := []struct {
		name       string
		v, n, want Vec
	}{
		{"off a floor-facing surface", Vec{3, -4}, Vec{0, 1}, Vec{3, 4}},
		{"off a ceiling-facing surface", Vec{3, 4}, Vec{0, -1}, Vec{3, -4}},
		{"off a left face", Vec{3, 4}, Vec{-1, 0}, Vec{-3, 4}},
		{"off a right face", Vec{-3, 4}, Vec{1, 0}, Vec{3, 4}},
		{"off a corner", Vec{3, -4}, Vec{-1, 1}, Vec{-3, 4}},
		{"already leaving is unchanged", Vec{3, 4}, Vec{0, 1}, Vec{3, 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reflect(tt.v, tt.n); got != tt.want {
				t.Errorf("reflect(%v, %v) = %v, want %v", tt.v, tt.n, got, tt.want)
			}
		})
	}
}
