// Package game holds TX-Ball's state and update logic. It knows nothing about
// the terminal: coordinates are float64 world units, where one unit is one
// half-block "pixel" on screen.
package game

// World dimensions in pixels. The playfield is fixed-size; the renderer
// centres it in whatever terminal it gets.
const (
	Width  = 80
	Height = 44
)

// Input is the player's intent for a single update step.
type Input struct{}

// Game is the whole simulation state.
type Game struct {
	// Ticks counts fixed updates since the game started.
	Ticks uint64
	// Time is the simulated time in seconds.
	Time float64
}

// New returns a game ready for its first Update.
func New() *Game {
	return &Game{}
}

// Update advances the simulation by one fixed step of dt seconds.
func (g *Game) Update(dt float64, _ Input) {
	g.Ticks++
	g.Time += dt
}
