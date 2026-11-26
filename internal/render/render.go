// Package render draws a game.Game onto a tcell screen.
//
// The playfield is rasterised into a pixel buffer and then emitted as
// upper-half blocks: each cell's foreground is its top pixel and its
// background is its bottom pixel.
package render

import (
	"fmt"

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
	sw, sh := s.Size()
	// Origin of the playfield's top-left cell; row oy-2 is the HUD.
	ox := max((sw-MinCols)/2, 0) + 1
	oy := max((sh-MinRows)/2, 0) + 2

	for i := range r.pix {
		r.pix[i] = colBackground
	}

	r.drawBorder(s, ox, oy)
	r.flush(s, ox, oy)

	// The screen is not cleared per frame (tcell would repaint every cell),
	// so the HUD row is composed in full and each cell written exactly once.
	hud := newLine(cols)
	hud.put(0, tcell.StyleDefault.Foreground(colText).Bold(true), "TX-Ball")
	hint := "q quit"
	hud.put(cols-len(hint), tcell.StyleDefault.Foreground(colDim), hint)
	if st.Show {
		dbg := fmt.Sprintf("fps %d  ups %d  t %.1fs", st.FPS, st.UPS, g.Time)
		hud.put((cols-len(dbg))/2, tcell.StyleDefault.Foreground(colDim), dbg)
	}
	hud.draw(s, ox, oy-2)
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
