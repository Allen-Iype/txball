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

	colLife = tcell.NewRGBColor(235, 80, 90)
	colFire = tcell.NewRGBColor(255, 150, 40)
	// colCapsuleText is the letter on a power-up capsule.
	colCapsuleText = tcell.NewRGBColor(20, 20, 30)
	colTitle       = tcell.NewRGBColor(255, 225, 120)

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

// glyph is a character cell drawn in place of a half block.
type glyph struct {
	r  rune // zero means none
	st tcell.Style
}

// Renderer owns the frame buffers so they are not reallocated every frame:
// a pixel layer, and a layer of text cells that sit on top of it.
type Renderer struct {
	pix  [game.Width * game.Height]tcell.Color
	text [rows][cols]glyph
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
	r.text = [rows][cols]glyph{}

	r.bricks(&g.Bricks)
	r.rect(g.Paddle.Left(), game.PaddleTop, g.Paddle.W, game.PaddleHeight, paddleColor(g))
	ballCol := colBall
	if g.Active(game.PowerFire) {
		ballCol = colFire
	}
	for _, b := range g.Balls {
		r.rect(b.Pos.X-game.BallSize/2, b.Pos.Y-game.BallSize/2, game.BallSize, game.BallSize, ballCol)
	}
	for _, p := range g.Powerups {
		r.capsule(p)
	}
	r.overlay(g)

	// The screen is not cleared per frame (tcell would repaint every cell),
	// so every cell is written exactly once: flush emits either the text
	// cell or the half block for each position, never both.
	r.flush(s, ox, oy)
	r.drawBorder(s, ox, oy, st, g)

	hud := newLine(cols, ' ', tcell.StyleDefault)
	hud.put(0, tcell.StyleDefault.Foreground(colText).Bold(true), "TX-Ball")
	x := 9
	for k, left := range g.Timers {
		if left <= 0 {
			continue
		}
		look := powerLooks[k]
		hud.put(x, look.style(), string(look.letter))
		x += 1 + hud.put(x+1, tcell.StyleDefault.Foreground(colDim), fmt.Sprintf("%-3d", int(math.Ceil(left))))
	}
	level := fmt.Sprintf("LEVEL %d", g.Level)
	hud.put((cols-len(level))/2, tcell.StyleDefault.Foreground(colText), level)
	score := fmt.Sprintf("%06d", g.Score)
	hud.put(cols-len(score), tcell.StyleDefault.Foreground(colText).Bold(true), score)
	lives := strings.Repeat("♥", max(g.Lives, 0))
	hud.put(cols-len(score)-2-utf8.RuneCountInString(lives), tcell.StyleDefault.Foreground(colLife), lives)
	hud.draw(s, ox, oy-2)
}

// paddleColor tints the paddle while it is sticky.
func paddleColor(g *game.Game) tcell.Color {
	if g.Active(game.PowerSticky) {
		return powerLooks[game.PowerSticky].color
	}
	return colPaddle
}

// powerLook is how a power-up appears, as a capsule and in the HUD.
type powerLook struct {
	letter rune
	color  tcell.Color
}

func (l powerLook) style() tcell.Style {
	return tcell.StyleDefault.Background(l.color).Foreground(colCapsuleText).Bold(true)
}

var powerLooks = [game.NumPowers]powerLook{
	game.PowerMulti:  {'M', tcell.NewRGBColor(90, 200, 250)},
	game.PowerLong:   {'L', tcell.NewRGBColor(110, 220, 120)},
	game.PowerShort:  {'S', tcell.NewRGBColor(240, 90, 90)},
	game.PowerFire:   {'F', colFire},
	game.PowerSticky: {'G', tcell.NewRGBColor(190, 230, 90)},
	game.PowerSlow:   {'W', tcell.NewRGBColor(170, 150, 250)},
	game.PowerLife:   {'+', tcell.NewRGBColor(250, 130, 190)},
}

// capsule draws a falling power-up as three text cells: its letter on a
// coloured pill. Letters can't be drawn in half-block pixels, so capsules
// move a whole cell row at a time.
func (r *Renderer) capsule(p game.Powerup) {
	look := powerLooks[p.Kind]
	cy := int(math.Floor(p.Pos.Y / 2))
	cx := int(math.Round(p.Pos.X - game.PowerW/2))
	if cy < 0 || cy >= rows {
		return
	}
	for i, ch := range [3]rune{' ', look.letter, ' '} {
		if x := cx + i; x >= 0 && x < cols {
			r.text[cy][x] = glyph{ch, look.style()}
		}
	}
}

// overlay writes the message for the current state, if it has one, into
// the text layer.
func (r *Renderer) overlay(g *game.Game) {
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
		return
	}
	base := tcell.StyleDefault.Background(colBackground)
	put := func(x int, st tcell.Style, str string) int {
		for _, ch := range str {
			if x >= 0 && x < cols {
				r.text[overlayRow][x] = glyph{ch, st}
			}
			x++
		}
		return x
	}
	n := utf8.RuneCountInString(hint)
	if title != "" {
		n += utf8.RuneCountInString(title) + 2
	}
	x := (cols - n) / 2
	if title != "" {
		x = put(x, base.Foreground(colTitle).Bold(true), title+"  ")
	}
	put(x, base.Foreground(colText), hint)
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

// flush emits the frame: a text cell where there is one, otherwise the
// two pixels beneath it as a half block.
func (r *Renderer) flush(s tcell.Screen, ox, oy int) {
	for cy := range rows {
		top := r.pix[2*cy*game.Width:]
		bot := r.pix[(2*cy+1)*game.Width:]
		for cx := range cols {
			if g := r.text[cy][cx]; g.r != 0 {
				s.SetContent(ox+cx, oy+cy, g.r, nil, g.st)
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

// line is one row of styled text, composed off-screen so that each of its
// cells is written to the screen exactly once.
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

// put writes str starting at column x, clipping to the line, and returns
// the number of columns str occupies.
func (l *line) put(x int, st tcell.Style, str string) int {
	n := 0
	for _, ch := range str {
		if x >= 0 && x < len(l.runes) {
			l.runes[x], l.styles[x] = ch, st
		}
		x++
		n++
	}
	return n
}

func (l *line) draw(s tcell.Screen, x, y int) {
	for i, ch := range l.runes {
		s.SetContent(x+i, y, ch, nil, l.styles[i])
	}
}
