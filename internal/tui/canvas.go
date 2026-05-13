package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

type cell struct {
	ch    rune
	style *lipgloss.Style
	skip  bool
}

type Canvas struct {
	width  int
	height int
	cells  [][]cell
}

type BorderRunes struct {
	TopLeft     rune
	TopRight    rune
	BottomLeft  rune
	BottomRight rune
	Horizontal  rune
	Vertical    rune
}

var (
	RoundedBorderRunes = BorderRunes{
		TopLeft:     '╭',
		TopRight:    '╮',
		BottomLeft:  '╰',
		BottomRight: '╯',
		Horizontal:  '─',
		Vertical:    '│',
	}
	DoubleBorderRunes = BorderRunes{
		TopLeft:     '╔',
		TopRight:    '╗',
		BottomLeft:  '╚',
		BottomRight: '╝',
		Horizontal:  '═',
		Vertical:    '║',
	}
	HeavyBorderRunes = BorderRunes{
		TopLeft:     '┏',
		TopRight:    '┓',
		BottomLeft:  '┗',
		BottomRight: '┛',
		Horizontal:  '━',
		Vertical:    '┃',
	}
)

func NewCanvas(width, height int) Canvas {
	cells := make([][]cell, height)
	for y := range cells {
		cells[y] = make([]cell, width)
		for x := range cells[y] {
			cells[y][x].ch = ' '
		}
	}
	return Canvas{width: width, height: height, cells: cells}
}

func (c *Canvas) Put(x, y int, ch rune, style lipgloss.Style) {
	if x < 0 || y < 0 || x >= c.width || y >= c.height {
		return
	}
	st := style
	c.cells[y][x] = cell{ch: ch, style: &st}
}

func (c *Canvas) Text(x, y int, text string, style lipgloss.Style) {
	if y < 0 || y >= c.height {
		return
	}
	col := x
	for _, ch := range []rune(text) {
		width := runewidth.RuneWidth(ch)
		if width <= 0 {
			width = 1
		}
		if col < 0 {
			col += width
			continue
		}
		if col+width > c.width {
			return
		}
		c.Put(col, y, ch, style)
		for offset := 1; offset < width; offset++ {
			c.cells[y][col+offset] = cell{skip: true}
		}
		col += width
	}
}

func (c *Canvas) HLine(x1, x2, y int, ch rune, style lipgloss.Style) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	for x := x1; x <= x2; x++ {
		c.Put(x, y, ch, style)
	}
}

func (c *Canvas) VLine(x, y1, y2 int, ch rune, style lipgloss.Style) {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	for y := y1; y <= y2; y++ {
		c.Put(x, y, ch, style)
	}
}

func (c *Canvas) Box(x, y, width, height int, style lipgloss.Style, selected bool) {
	border := RoundedBorderRunes
	if selected {
		border = DoubleBorderRunes
	}
	c.BoxWithBorder(x, y, width, height, style, border)
}

func (c *Canvas) BoxWithBorder(x, y, width, height int, style lipgloss.Style, border BorderRunes) {
	if width < 2 || height < 2 {
		return
	}
	c.Put(x, y, border.TopLeft, style)
	c.Put(x+width-1, y, border.TopRight, style)
	c.Put(x, y+height-1, border.BottomLeft, style)
	c.Put(x+width-1, y+height-1, border.BottomRight, style)
	c.HLine(x+1, x+width-2, y, border.Horizontal, style)
	c.HLine(x+1, x+width-2, y+height-1, border.Horizontal, style)
	c.VLine(x, y+1, y+height-2, border.Vertical, style)
	c.VLine(x+width-1, y+1, y+height-2, border.Vertical, style)
}

func (c Canvas) Render() string {
	lines := make([]string, c.height)
	for y := 0; y < c.height; y++ {
		var b strings.Builder
		for x := 0; x < c.width; x++ {
			if c.cells[y][x].skip {
				continue
			}
			ch := c.cells[y][x].ch
			if ch == 0 {
				ch = ' '
			}
			if c.cells[y][x].style != nil {
				b.WriteString(c.cells[y][x].style.Render(string(ch)))
			} else {
				b.WriteRune(ch)
			}
		}
		lines[y] = b.String()
	}
	return strings.Join(lines, "\n")
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	limit := width - 1
	var out strings.Builder
	current := 0
	for _, ch := range []rune(value) {
		chWidth := runewidth.RuneWidth(ch)
		if chWidth <= 0 {
			chWidth = 1
		}
		if current+chWidth > limit {
			break
		}
		out.WriteRune(ch)
		current += chWidth
	}
	return out.String() + "…"
}
