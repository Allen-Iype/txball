// Package render draws a game.Game onto a tcell screen.
//
// The playfield is rasterised into a pixel buffer and then emitted as
// upper-half blocks: each cell's foreground is its top pixel and its
// background is its bottom pixel.
package render

import (
	"fmt"
	"math"
	"strings"
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
	// armorShades is indexed by hits remaining, 1 to game.MaxArmor: the
	// brick dulls as it weakens.
	armorShades = [game.MaxArmor + 1]tcell.Color{
		1: tcell.NewRGBColor(120, 122, 142),
		2: tcell.NewRGBColor(178, 180, 200),
		3: tcell.NewRGBColor(238, 238, 248),
	}
	colSolidLight = tcell.NewRGBColor(104, 96, 88)
	colSolidDark  = tcell.NewRGBColor(58, 54, 50)

	colLife  = tcell.NewRGBColor(235, 80, 90)
	colTitle = tcell.NewRGBColor(255, 225, 120)

	// overlayRow is the playfield cell row for messages: the gap between
	// the lowest bricks and the paddle.
	overlayRow = 17

	// brickPalette is indexed by game.Brick.Color, wrapping around.
	brickPalette = []tcell.Color{
		tcell.NewRGBColor(235, 80, 90),
		tcell.NewRGBColor(245, 150, 60),
		tcell.NewRGBColor(240, 210, 70),
		tcell.NewRGBColor(110, 205, 95),
		tcell.NewRGBColor(70, 190, 200),
		tcell.NewRGBColor(90, 130, 240),
		tcell.NewRGBColor(175, 110, 230),
	}
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
	r.bricks(&g.Bricks)
	r.rect(g.Paddle.Left(), game.PaddleTop, g.Paddle.W, game.PaddleHeight, colPaddle)
	r.rect(g.Ball.Pos.X-game.BallSize/2, g.Ball.Pos.Y-game.BallSize/2, game.BallSize, game.BallSize, colBall)

	// The screen is not cleared per frame (tcell would repaint every cell),
	// so every cell is written exactly once: text rows are composed off
	// screen, and the playfield skips the cells its overlay will cover.
	msg := overlay(g)
	r.flush(s, ox, oy, msg)
	msg.draw(s, ox, oy+overlayRow)
	r.drawBorder(s, ox, oy, st, g)

	hud := newLine(cols, ' ', tcell.StyleDefault)
	hud.put(0, tcell.StyleDefault.Foreground(colText).Bold(true), "TX-Ball")
	level := fmt.Sprintf("LEVEL %d", g.Level)
	hud.put((cols-len(level))/2, tcell.StyleDefault.Foreground(colText), level)
	score := fmt.Sprintf("%06d", g.Score)
	hud.put(cols-len(score), tcell.StyleDefault.Foreground(colText).Bold(true), score)
	lives := strings.Repeat("♥", max(g.Lives, 0))
	hud.put(cols-len(score)-2-utf8.RuneCountInString(lives), tcell.StyleDefault.Foreground(colLife), lives)
	hud.draw(s, ox, oy-2)
}

// overlay returns the message for the current state, centred on a
// playfield-wide line whose untouched cells are transparent, or nil.
func overlay(g *game.Game) *line {
	var title, hint string
	switch g.State {
	case game.StateServe:
		hint = "click or space to launch"
	case game.StatePaused:
		title, hint = "PAUSED", "p to resume"
	case game.StateLevelClear:
		title, hint = fmt.Sprintf("LEVEL %d CLEARED", g.Level), "click or space for the next level"
	case game.StateGameOver:
		title, hint = "GAME OVER", fmt.Sprintf("score %d · click or space to play again", g.Score)
	default:
		return nil
	}
	l := newLine(cols, 0, tcell.StyleDefault)
	text := hint
	if title != "" {
		text = title + "  " + hint
	}
	x := (cols - utf8.RuneCountInString(text)) / 2
	base := tcell.StyleDefault.Background(colBackground)
	if title != "" {
		l.put(x, base.Foreground(colTitle).Bold(true), title+"  ")
		x += utf8.RuneCountInString(title) + 2
	}
	l.put(x, base.Foreground(colText), hint)
	return l
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

// bricks draws the grid. Each breakable brick's last pixel column is
// shaded so that neighbours of the same colour read as separate tiles;
// indestructible bricks are a two-tone checker instead.
func (r *Renderer) bricks(grid *game.Grid) {
	for row := range grid {
		for col, b := range grid[row] {
			box := game.BrickRect(col, row)
			var c tcell.Color
			switch b.Kind {
			case game.BrickNone:
				continue
			case game.BrickSolid:
				x0, y0 := int(box.Min.X), int(box.Min.Y)
				for dy := range int(game.BrickH) {
					for dx := range int(game.BrickW) {
						c := colSolidLight
						if (dx+dy)%2 == 1 {
							c = colSolidDark
						}
						r.set(x0+dx, y0+dy, c)
					}
				}
				continue
			case game.BrickArmor:
				c = armorShades[min(int(b.Hits), game.MaxArmor)]
			default:
				c = brickPalette[int(b.Color)%len(brickPalette)]
			}
			r.rect(box.Min.X, box.Min.Y, game.BrickW-1, game.BrickH, c)
			r.rect(box.Max.X-1, box.Min.Y, 1, game.BrickH, shade(c, 0.6))
		}
	}
}

// shade scales a colour's brightness by k.
func shade(c tcell.Color, k float64) tcell.Color {
	r, g, b := c.RGB()
	return tcell.NewRGBColor(int32(float64(r)*k), int32(float64(g)*k), int32(float64(b)*k))
}

// set colours one world pixel, ignoring anything outside the playfield.
func (r *Renderer) set(x, y int, c tcell.Color) {
	if x < 0 || x >= game.Width || y < 0 || y >= game.Height {
		return
	}
	r.pix[y*game.Width+x] = c
}

// flush emits the pixel buffer as half blocks, leaving alone the cells of
// the overlay row that skip (which may be nil) has text in.
func (r *Renderer) flush(s tcell.Screen, ox, oy int, skip *line) {
	for cy := range rows {
		top := r.pix[2*cy*game.Width:]
		bot := r.pix[(2*cy+1)*game.Width:]
		for cx := range cols {
			if cy == overlayRow && skip.covers(cx) {
				continue
			}
			st := tcell.StyleDefault.Foreground(top[cx]).Background(bot[cx])
			s.SetContent(ox+cx, oy+cy, '▀', nil, st)
		}
	}
}

// drawBorder frames the playfield. The bottom edge carries the key hints
// and, when enabled, the loop stats.
func (r *Renderer) drawBorder(s tcell.Screen, ox, oy int, stats Stats, g *game.Game) {
	st := tcell.StyleDefault.Foreground(colBorder)
	l, rt, t, b := ox-1, ox+cols, oy-1, oy+rows
	for x := ox; x < rt; x++ {
		s.SetContent(x, t, '─', nil, st)
	}
	for y := oy; y < b; y++ {
		s.SetContent(l, y, '│', nil, st)
		s.SetContent(rt, y, '│', nil, st)
	}
	s.SetContent(l, t, '┌', nil, st)
	s.SetContent(rt, t, '┐', nil, st)
	s.SetContent(l, b, '└', nil, st)
	s.SetContent(rt, b, '┘', nil, st)

	bottom := newLine(cols, '─', st)
	dim := tcell.StyleDefault.Foreground(colDim)
	hint := " mouse or ←/→ move · p pause · q quit "
	bottom.put(cols-2-utf8.RuneCountInString(hint), dim, hint)
	if stats.Show {
		bottom.put(2, dim, fmt.Sprintf(" fps %d · ups %d · t %.1fs ", stats.FPS, stats.UPS, g.Time))
	}
	bottom.draw(s, ox, b)
}

// line is one row of styled text, composed off-screen. Cells holding the
// zero rune are transparent: draw leaves the screen alone there.
type line struct {
	runes  []rune
	styles []tcell.Style
}

func newLine(n int, fill rune, st tcell.Style) *line {
	l := &line{runes: make([]rune, n), styles: make([]tcell.Style, n)}
	for i := range l.runes {
		l.runes[i], l.styles[i] = fill, st
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

// covers reports whether column x holds text. A nil line covers nothing.
func (l *line) covers(x int) bool {
	return l != nil && l.runes[x] != 0
}

func (l *line) draw(s tcell.Screen, x, y int) {
	if l == nil {
		return
	}
	for i, ch := range l.runes {
		if ch != 0 {
			s.SetContent(x+i, y, ch, nil, l.styles[i])
		}
	}
}
