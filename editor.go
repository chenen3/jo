package main

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type editor struct {
	x1, y1 int
	x2, y2 int
	screen tcell.Screen
	style  tcell.Style

	// editable buffer
	bx1, by1 int
	bx2, by2 int
	buf      [][]rune
	row      int // current number of line in buffer
	col      int // current number of column in buffer
	dirty    bool

	lineBar   *lineBar
	startLine int

	findKey   string
	findRow   int // cursor position when starting to find
	findMatch [][2]int
	findIndex int
}

func (e *editor) ClearFind() {
	e.findKey = ""
	e.findMatch = nil
}

// It is not efficient to accept src in bytes, but it is acceptable.
func newEditor(s tcell.Screen, src []byte) (*editor, error) {
	style := tcell.StyleDefault.Background(tcell.ColorWhite).Foreground(tcell.ColorBlack)
	e := &editor{
		screen:    s,
		style:     style,
		startLine: 1,
		row:       1,
		col:       1,
	}

	a := bytes.Split(src, []byte{'\n'})
	e.buf = make([][]rune, len(a))
	for i := range a {
		e.buf[i] = []rune(string(a[i]))
	}
	if len(e.buf) == 0 || len(e.buf[len(e.buf)-1]) != 0 {
		// file ends with a new line
		e.buf = append(e.buf, []rune{})
	}

	e.lineBar = &lineBar{
		s:     s,
		style: style.Foreground(tcell.ColorGray),
	}
	return e, nil
}

// maximize number of lines that can be displayed by the editor at one time
func (e *editor) height() int { return e.by2 - e.by1 + 1 }

const tabWidth = 4

// return the number of leading tabs
func leadingTabs(line []rune) int {
	var n int
	for i := range line {
		if line[i] != '\t' {
			break
		}
		n++
	}
	return n
}

func (e *editor) drawLine(row int) {
	line := e.buf[row-1]
	for x := e.bx1; x <= e.bx2; x++ {
		if x <= e.bx1+len(line)-1 {
			continue
		}
		e.screen.SetContent(x, e.by1+row-e.startLine, ' ', nil, e.style)
	}

	if len(line) == 0 {
		return
	}

	var mi int
	var inLineMatch [][2]int
	for _, m := range e.findMatch {
		if m[0] == row-1 {
			inLineMatch = append(inLineMatch, m)
		}
	}

	tokens := parseToken(line)
	i := 0
	tabs := leadingTabs(line)
	padding := 0
	style := e.style.Foreground(tokenColor(tokens[i].class))
	for j := range line {
		if j >= tokens[i].off+tokens[i].len && i < len(tokens)-1 {
			i++
			style = style.Foreground(tokenColor(tokens[i].class))
		}

		if len(inLineMatch) > 0 && mi < len(inLineMatch) {
			if inLineMatch[mi][1] <= j && j < inLineMatch[mi][1]+len(e.findKey) {
				if inLineMatch[mi] == e.findMatch[e.findIndex] {
					style = style.Background(tcell.ColorYellow)
				} else {
					style = style.Background(tcell.ColorLightYellow)
				}
			} else if j >= inLineMatch[mi][1]+len(e.findKey) {
				mi++
				// restore
				style = style.Background(tcell.ColorWhite)
			}
		}

		e.screen.SetContent(e.bx1+padding+j, e.by1+row-e.startLine, line[j], nil, style)
		if j < tabs {
			// consider showing tab as '|' for debugging
			e.screen.SetContent(e.bx1+padding+j, e.by1+row-e.startLine, ' ', nil, e.style.Foreground(tcell.ColorGray))
			for k := 0; k < tabWidth-1; k++ {
				padding++
				e.screen.SetContent(e.bx1+padding+j, e.by1+row-e.startLine, ' ', nil, e.style.Foreground(tcell.ColorGray))
			}
		}
	}
}

func (e *editor) Draw() {
	e.x1 = 0
	e.y1 = 1
	width, height := e.screen.Size()
	e.x2 = width - 1
	e.y2 = height - 2

	lineBarWidth := 2
	for i := len(e.buf); i > 0; i = i / 10 {
		lineBarWidth++
	}
	e.lineBar.x1 = e.x1
	e.lineBar.y1 = e.y1
	e.lineBar.x2 = e.x1 + lineBarWidth
	e.lineBar.y2 = e.y2

	e.bx1 = e.x1 + lineBarWidth
	e.by1 = e.y1
	e.bx2 = e.x2
	e.by2 = e.y2

	e.lineBar.startLine = e.startLine
	endLine := e.startLine + e.height() - 1
	if endLine > len(e.buf) {
		endLine = len(e.buf)
	}
	e.lineBar.endLine = endLine
	e.lineBar.draw()

	for y := e.by1; y <= e.by2; y++ {
		for x := e.bx1; x <= e.bx2; x++ {
			e.screen.SetContent(x, y, ' ', nil, e.style)
		}
	}

	for i := 0; i < e.height(); i++ {
		if e.startLine-1+i >= len(e.buf) {
			break
		}
		e.drawLine(e.startLine + i)
	}
	e.ShowCursor()
}

func (e *editor) cursorRowAdd(delta int) {
	if delta == 0 {
		return
	}

	row := e.row + delta
	if row < 1 {
		row = 1
	} else if row > len(e.buf) {
		row = len(e.buf)
	}
	e.row = row

	if e.col > len(e.buf[e.row-1])+1 {
		e.col = len(e.buf[e.row-1]) + 1
	}
}

func (e *editor) cursorColAdd(delta int) {
	col := e.col + delta
	if 1 <= col && col <= len(e.buf[e.row-1])+1 {
		e.col = col
		return
	}

	// line start
	if col < 1 {
		if e.row == 1 {
			e.col = 1
			return
		}
		// to the end of the previous line
		e.row--
		e.col = len(e.buf[e.row-1]) + 1
		return
	}

	// line end
	if e.row == len(e.buf) {
		e.col = len(e.buf[e.row-1]) + 1
		return
	}
	e.row++
	e.col = 1
}

func (e *editor) SetCursor(x, y int) {
	if y < e.by1 {
		y = e.by1
	}
	row := y - e.by1 + e.startLine
	if row > len(e.buf) {
		row = len(e.buf)
	}
	col := x - e.bx1 + 1
	tabs := leadingTabs(e.buf[row-1])
	if col <= tabs*tabWidth {
		i, j := col/tabWidth, col%tabWidth
		// When the position of the cursor is more than half of the tabWidth,
		// it is considered to move to the next tab.
		if j > tabWidth/2 {
			col = i + 2
		} else {
			col = i + 1
		}
	} else {
		col -= tabs * (tabWidth - 1)
	}

	if col > len(e.buf[row-1])+1 {
		col = len(e.buf[row-1]) + 1
	}
	e.row, e.col = row, col
	e.ShowCursor()
}

func (e *editor) ShowCursor() {
	tabs := leadingTabs(e.buf[e.row-1])
	var x int
	if e.col <= tabs {
		x = e.bx1 + (e.col-1)*tabWidth
	} else {
		padding := tabs * (tabWidth - 1)
		x = e.bx1 + e.col + padding - 1
	}

	e.screen.ShowCursor(x, e.by1+e.row-e.startLine)
}

func (e *editor) CursorUp() {
	if e.row == 1 {
		return
	}
	if e.row == e.startLine {
		e.startLine--
		e.cursorRowAdd(-1)
		e.Draw()
		return
	}
	e.cursorRowAdd(-1)
	e.ShowCursor()
}

func (e *editor) CursorDown() {
	if e.row == len(e.buf) {
		// end of file
		return
	}

	if e.row < e.startLine+e.height()-1 {
		e.cursorRowAdd(1)
		e.ShowCursor()
		return
	}

	e.startLine++
	e.cursorRowAdd(1)
	e.Draw()
}

func (e *editor) CursorLeft() {
	e.cursorColAdd(-1)
	e.ShowCursor()
}

func (e *editor) CursorRight() {
	e.cursorColAdd(1)
	e.ShowCursor()
}

func (e *editor) CursorLineStart() {
	e.col = 1
	e.ShowCursor()
}

func (e *editor) CursorLineEnd() {
	e.cursorColAdd(len(e.buf[e.row-1]) - e.col + 1)
	e.ShowCursor()
}

func (e *editor) WheelUp(delta int) {
	e.startLine -= delta
	if e.startLine < 1 {
		e.startLine = 1
	} else if e.startLine > len(e.buf) {
		e.startLine = len(e.buf)
	}
	e.Draw()
}

func (e *editor) WheelDown(delta int) {
	e.startLine += delta
	if e.startLine < 1 {
		e.startLine = 1
	} else if e.startLine > len(e.buf) {
		e.startLine = len(e.buf)
	}
	e.Draw()
}

func (e *editor) Insert(r rune) {
	line := e.buf[e.row-1]
	rs := make([]rune, len(line[e.col-1:]))
	copy(rs, line[e.col-1:])
	line = append(append(line[:e.col-1], r), rs...)
	e.buf[e.row-1] = line
	e.drawLine(e.row)
	e.CursorRight()
	e.dirty = true
}

// Row return current number of line in editor
func (e *editor) Row() int {
	return e.row
}

// Col return current number of column in editor,
// it is intended for the line number of status line.
// Note that it is different from e.col.
func (e *editor) Col() int {
	var col int
	tabs := leadingTabs(e.buf[e.row-1])
	if e.col <= tabs {
		col = (e.col-1)*tabWidth + 1
	} else {
		padding := tabs * (tabWidth - 1)
		col = e.col + padding
	}
	return col
}

func (e *editor) DeleteLeft() {
	e.dirty = true
	// cursor at the head of line, merge the line to previous one
	if e.col == 1 {
		if e.row == 1 {
			return
		}
		// prevLen := len(e.buf[e.row-2])
		prevLine := e.buf[e.row-2]
		e.buf[e.row-2] = append(prevLine, e.buf[e.row-1]...)
		e.buf = append(e.buf[:e.row-1], e.buf[e.row:]...)
		e.cursorRowAdd(-1)
		e.cursorColAdd(1 + len(prevLine) - e.col)
		e.Draw()
		return
	}

	line := e.buf[e.row-1]
	if e.col-1 == len(line) {
		// line end
		line = line[:e.col-2]
	} else {
		// TODO: consider the new function slices.Delete ?
		line = append(line[:e.col-2], line[e.col-1:]...)
	}
	e.buf[e.row-1] = line
	e.drawLine(e.row)
	e.CursorLeft()
}

func (e *editor) DeleteToLineStart() {
	e.dirty = true
	e.buf[e.row-1] = e.buf[e.row-1][e.col-1:]
	e.drawLine(e.row)
	e.CursorLineStart()
}

func (e *editor) DeleteToLineEnd() {
	e.dirty = true
	e.buf[e.row-1] = e.buf[e.row-1][:e.col-1]
	e.drawLine(e.row)
}

func (e *editor) Enter() {
	e.dirty = true
	// cut current line
	line := e.buf[e.row-1]
	newline := make([]rune, len(line[e.col-1:]))
	copy(newline, line[e.col-1:])
	e.buf[e.row-1] = line[:e.col-1]
	// TODO: not efficient
	buf := make([][]rune, len(e.buf[e.row:]))
	for i, rs := range e.buf[e.row:] {
		buf[i] = make([]rune, len(rs))
		copy(buf[i], rs)
	}
	e.buf = append(append(e.buf[:e.row], newline), buf...)
	e.cursorRowAdd(1)
	e.CursorLineStart()
	e.Draw()
}

// A newline is appended if the last character of buffer is not
// already a newline
func (e *editor) WriteTo(w io.Writer) (int64, error) {
	if len(e.buf[len(e.buf)-1]) != 0 {
		// file ends with a new line
		e.buf = append(e.buf, []rune{})
	}
	// TODO: not efficient
	buf := make([][]byte, len(e.buf))
	for i, rs := range e.buf {
		buf[i] = []byte(string(rs))
	}
	bs := bytes.Join(buf, []byte{'\n'})

	n, err := w.Write(bs)
	if err != nil {
		return int64(n), err
	}
	e.dirty = false
	return int64(n), nil
}

func (e *editor) Find(s string) {
	if len(s) == 0 {
		return
	}
	e.findKey = s
	e.findRow = e.row

	var match [][2]int
	for i := range e.buf {
		j := strings.Index(string(e.buf[i]), e.findKey)
		if j >= 0 {
			match = append(match, [2]int{i, j})
		}
	}
	e.findMatch = match
	if len(match) == 0 {
		return
	}
	defer e.Draw()

	// jump to the nearest matching line
	var minGap = len(e.buf)
	var near int
	for i, m := range match {
		gap := m[0] + 1 - e.findRow
		if gap < 0 {
			gap = 0 - gap
		}
		if gap == 0 {
			near = i
			break
		} else if gap < minGap {
			minGap = gap
			near = i
		} else {
			// the matching results are naturally ordered
			break
		}
	}
	e.findIndex = near

	e.row = match[near][0] + 1
	// place the cursor at the end of the matching word for easy editing
	e.col = match[near][1] + len(e.findKey) + 1
	if e.row < e.startLine {
		e.startLine = e.row
	} else if e.row > (e.startLine + e.height() - 1) {
		e.startLine = e.row - e.height()/2
	}
}

func (e *editor) FindNext() {
	if len(e.findMatch) == 0 {
		return
	}

	if e.findIndex == len(e.findMatch)-1 {
		e.findIndex = 0
	} else {
		e.findIndex++
	}

	i, j := e.findMatch[e.findIndex][0], e.findMatch[e.findIndex][1]
	e.row = i + 1
	if e.row < e.startLine {
		e.startLine = e.row
	} else if e.row > (e.startLine + e.height() - 1) {
		e.startLine = e.row - e.height()/2
	}
	// place the cursor at the end of the matching word for easy editing
	e.col = j + len(e.findKey) + 1
	e.Draw()
}

func (e *editor) FindPrev() {
	if len(e.findMatch) == 0 {
		return
	}

	if e.findIndex == 0 {
		e.findIndex = len(e.findMatch) - 1
	} else {
		e.findIndex--
	}

	i, j := e.findMatch[e.findIndex][0], e.findMatch[e.findIndex][1]
	e.row = i + 1
	if e.row < e.startLine {
		e.startLine = e.row
	} else if e.row > (e.startLine + e.height() - 1) {
		e.startLine = e.row - e.height()/2
	}
	// place the cursor at the end of the matching word for easy editing
	e.col = j + len(e.findKey) + 1
	e.Draw()
}

func (e *editor) HandleEvent(ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventMouse:
		x, y := ev.Position()
		switch ev.Buttons() {
		case tcell.Button1, tcell.Button2:
			e.SetCursor(x, y)
		case tcell.WheelUp:
			delta := int(float32(y) * wheelSensitivity)
			e.WheelUp(delta)
		case tcell.WheelDown:
			delta := int(float32(y) * wheelSensitivity)
			e.WheelDown(delta)
		}
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyCtrlA:
			e.CursorLineStart()
		case tcell.KeyCtrlE:
			e.CursorLineEnd()
		case tcell.KeyUp:
			e.CursorUp()
		case tcell.KeyDown:
			e.CursorDown()
		case tcell.KeyLeft:
			e.CursorLeft()
		case tcell.KeyRight:
			e.CursorRight()
		case tcell.KeyRune:
			e.Insert(ev.Rune())
		case tcell.KeyTab:
			e.Insert('\t')
		case tcell.KeyEnter:
			e.Enter()
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			e.DeleteLeft()
		case tcell.KeyCtrlU:
			e.DeleteToLineStart()
		case tcell.KeyCtrlK:
			e.DeleteToLineEnd()
		}
	}
}

func (e *editor) Update(_ string)                {}
func (e *editor) Position() (x1, y1, x2, y2 int) { return e.x1, e.y1, e.x2, e.y2 }

type lineBar struct {
	s         tcell.Screen
	x1, y1    int
	x2, y2    int
	style     tcell.Style
	startLine int
	endLine   int
}

func (b *lineBar) draw() {
	for y := b.y1; y <= b.y2; y++ {
		for x := b.x1; x <= b.x2; x++ {
			b.s.SetContent(x, y, ' ', nil, b.style)
		}
	}

	paddingRight := 1
	x, y := b.x1, b.y1
	for i := b.startLine; i <= b.endLine; i++ {
		s := strconv.Itoa(i)
		for j, c := range s {
			if x+j > b.x2 {
				break
			}
			// align right
			b.s.SetContent(b.x2-(len(s)-j)-paddingRight, y+i-b.startLine, c, nil, b.style)
		}
	}
}
