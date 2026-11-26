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

func TestBounceWalls(t *testing.T) {
	const (
		half = 0.5
		w    = 80.0
	)
	tests := []struct {
		name             string
		pos, vel         Vec
		wantPos, wantVel Vec
	}{
		{"inside is untouched", Vec{40, 20}, Vec{3, -4}, Vec{40, 20}, Vec{3, -4}},
		{"touching left is untouched", Vec{0.5, 20}, Vec{-3, 4}, Vec{0.5, 20}, Vec{-3, 4}},
		{"left overshoot mirrors", Vec{0.2, 20}, Vec{-3, 4}, Vec{0.8, 20}, Vec{3, 4}},
		{"right overshoot mirrors", Vec{79.9, 20}, Vec{3, 4}, Vec{79.1, 20}, Vec{-3, 4}},
		{"top overshoot mirrors", Vec{40, 0.1}, Vec{3, -4}, Vec{40, 0.9}, Vec{3, 4}},
		{"top-left corner flips both", Vec{0.1, 0.3}, Vec{-3, -4}, Vec{0.9, 0.7}, Vec{3, 4}},
		{"top-right corner flips both", Vec{79.8, 0.3}, Vec{3, -4}, Vec{79.2, 0.7}, Vec{-3, 4}},
		{"bottom is open", Vec{40, 60}, Vec{3, 4}, Vec{40, 60}, Vec{3, 4}},
		{"outside left but already heading in", Vec{0.2, 20}, Vec{3, 4}, Vec{0.8, 20}, Vec{3, 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos, vel := bounceWalls(tt.pos, tt.vel, half, w)
			if !vecNear(pos, tt.wantPos) {
				t.Errorf("pos = %v, want %v", pos, tt.wantPos)
			}
			if !vecNear(vel, tt.wantVel) {
				t.Errorf("vel = %v, want %v", vel, tt.wantVel)
			}
		})
	}
}
