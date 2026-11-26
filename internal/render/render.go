// Package render draws a game.Game onto a tcell screen.
//
// The playfield is rasterised into a pixel buffer and then emitted as
// upper-half blocks: each cell's foreground is its top pixel and its
// background is its bottom pixel.
package render

import (
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"github.com/Allen-Iype/txball/internal/game"
)

const (
	cols = game.Width
	rows = game.Height / 2

	// MinCols and MinRows are the smallest terminal that fits the HUD row,
	// the border and the playfield.
	MinCols = cols + 2
	MinRows = rows + 3
)

var (
	colBackground = tcell.NewRGBColor(12, 12, 20)
	colBorder     = tcell.NewRGBColor(90, 90, 120)
	colText       = tcell.NewRGBColor(200, 200, 215)
	colDim        = tcell.NewRGBColor(110, 110, 130)
	colPaddle     = tcell.NewRGBColor(120, 210, 255)
	colBall       = tcell.NewRGBColor(255, 255, 255)
)

// Stats are loop measurements shown when debugging.
type Stats struct {
	Show     bool
	FPS, UPS int
}

// Renderer owns the pixel buffer so it is not reallocated every frame.
type Renderer struct {
	pix [game.Width * game.Height]tcell.Color
}

// New returns a ready Renderer.
func New() *Renderer {
	return &Renderer{}
}

// Draw paints one frame. It does not call Show, and it only writes the cells
// it owns: the caller clears the screen when the layout changes (resize).
func (r *Renderer) Draw(s tcell.Screen, g *game.Game, st Stats) {
	ox, oy := Origin(s.Size())

	for i := range r.pix {
		r.pix[i] = colBackground
	}
	r.rect(g.Paddle.Left(), game.PaddleTop, g.Paddle.W, game.PaddleHeight, colPaddle)
	r.rect(g.Ball.Pos.X-game.BallSize/2, g.Ball.Pos.Y-game.BallSize/2, game.BallSize, game.BallSize, colBall)

	r.drawBorder(s, ox, oy)
	r.flush(s, ox, oy)

	// The screen is not cleared per frame (tcell would repaint every cell),
	// so the HUD row is composed in full and each cell written exactly once.
	hud := newLine(cols)
	hud.put(0, tcell.StyleDefault.Foreground(colText).Bold(true), "TX-Ball")
	hint := "mouse or ←/→ move · q quit"
	hud.put(cols-utf8.RuneCountInString(hint), tcell.StyleDefault.Foreground(colDim), hint)
	if st.Show {
		dbg := fmt.Sprintf("fps %d  ups %d  t %.1fs", st.FPS, st.UPS, g.Time)
		hud.put((cols-len(dbg))/2, tcell.StyleDefault.Foreground(colDim), dbg)
	}
	hud.draw(s, ox, oy-2)
}

// Origin returns the screen cell of the playfield's top-left corner for a
// terminal of the given size. The HUD sits two rows above it.
func Origin(screenW, screenH int) (ox, oy int) {
	return max((screenW-MinCols)/2, 0) + 1, max((screenH-MinRows)/2, 0) + 2
}

// WorldX converts a screen column to the world X of that cell's centre.
func WorldX(screenW, screenH, col int) float64 {
	ox, _ := Origin(screenW, screenH)
	return float64(col-ox) + 0.5
}

// rect fills a world-space rectangle, snapping its corner to the nearest
// pixel. This is the only place world coordinates are rounded.
func (r *Renderer) rect(x, y, w, h float64, c tcell.Color) {
	x0, y0 := int(math.Round(x)), int(math.Round(y))
	for py := y0; py < y0+int(math.Round(h)); py++ {
		for px := x0; px < x0+int(math.Round(w)); px++ {
			r.set(px, py, c)
		}
	}
}

// set colours one world pixel, ignoring anything outside the playfield.
func (r *Renderer) set(x, y int, c tcell.Color) {
	if x < 0 || x >= game.Width || y < 0 || y >= game.Height {
		return
	}
	r.pix[y*game.Width+x] = c
}

func (r *Renderer) flush(s tcell.Screen, ox, oy int) {
	for cy := range rows {
		top := r.pix[2*cy*game.Width:]
		bot := r.pix[(2*cy+1)*game.Width:]
		for cx := range cols {
			st := tcell.StyleDefault.Foreground(top[cx]).Background(bot[cx])
			s.SetContent(ox+cx, oy+cy, '▀', nil, st)
		}
	}
}

func (r *Renderer) drawBorder(s tcell.Screen, ox, oy int) {
	st := tcell.StyleDefault.Foreground(colBorder)
	l, rt, t, b := ox-1, ox+cols, oy-1, oy+rows
	for x := ox; x < rt; x++ {
		s.SetContent(x, t, '─', nil, st)
		s.SetContent(x, b, '─', nil, st)
	}
	for y := oy; y < b; y++ {
		s.SetContent(l, y, '│', nil, st)
		s.SetContent(rt, y, '│', nil, st)
	}
	s.SetContent(l, t, '┌', nil, st)
	s.SetContent(rt, t, '┐', nil, st)
	s.SetContent(l, b, '└', nil, st)
	s.SetContent(rt, b, '┘', nil, st)
}

// line is one row of styled text, composed off-screen.
type line struct {
	runes  []rune
	styles []tcell.Style
}

func newLine(n int) *line {
	l := &line{runes: make([]rune, n), styles: make([]tcell.Style, n)}
	for i := range l.runes {
		l.runes[i] = ' '
	}
	return l
}

// put writes str starting at column x, clipping to the line.
func (l *line) put(x int, st tcell.Style, str string) {
	for _, ch := range str {
		if x >= 0 && x < len(l.runes) {
			l.runes[x], l.styles[x] = ch, st
		}
		x++
	}
}

func (l *line) draw(s tcell.Screen, x, y int) {
	for i, ch := range l.runes {
		s.SetContent(x+i, y, ch, nil, l.styles[i])
	}
}
