package main

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type editor struct {
	x1, y1 int
	x2, y2 int
	style  tcell.Style

	// editing buffer
	buf      [][]rune
	bx1, by1 int
	bx2, by2 int
	line     int // line number, starting at 1
	column   int // column number, starting at 1
	dirty    bool
	filename string

	lineBar   *lineBar
	startLine int // starting line number

	findKey   string
	findLine  int // line number when starting to find
	findMatch [][2]int
	findIndex int // index of the matching result
}

func (e *editor) ClearFind() {
	e.findKey = ""
	e.findMatch = nil
}

func (e *editor) SetPos(x, y, width, height int) {
	e.x1 = x
	e.y1 = y
	e.x2 = x + width - 1
	e.y2 = y + height - 1
}

func newEditor(j *Jo, filename string) *editor {
	style := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	e := &editor{
		style:     style,
		startLine: 1,
		line:      1,
		column:    1,
		filename:  filename,
		lineBar: &lineBar{
			style: style.Foreground(tcell.ColorGray),
		},
	}

	src, err := os.ReadFile(filename)
	if err != nil {
		logger.Println(err)
		e.buf = append(e.buf, []rune{})
		return e
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
	return e
}

// the number of lines visible in the editor view
func (e *editor) PageSize() int { return e.by2 - e.by1 + 1 }

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

func (e *editor) renderLine(line int) {
	text := e.buf[line-1]
	for x := e.bx1; x <= e.bx2; x++ {
		if x <= e.bx1+len(text)-1 {
			continue
		}
		screen.SetContent(x, e.by1+line-e.startLine, ' ', nil, e.style)
	}

	if len(text) == 0 {
		return
	}

	var mi int
	var inLineMatch [][2]int
	for _, m := range e.findMatch {
		if m[0] == line-1 {
			inLineMatch = append(inLineMatch, m)
		}
	}

	tokens := parseToken(text)
	i := 0
	tabs := leadingTabs(text)
	padding := 0
	style := e.style.Foreground(tokenColor(tokens[i].class))
	for j := range text {
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
				_, bg, _ := e.style.Decompose()
				style = style.Background(bg)
			}
		}

		screen.SetContent(e.bx1+padding+j, e.by1+line-e.startLine, text[j], nil, style)
		if j < tabs {
			// consider showing tab as '|' for debugging
			screen.SetContent(e.bx1+padding+j, e.by1+line-e.startLine, ' ', nil, e.style.Foreground(tcell.ColorGray))
			for k := 0; k < tabWidth-1; k++ {
				padding++
				screen.SetContent(e.bx1+padding+j, e.by1+line-e.startLine, ' ', nil, e.style.Foreground(tcell.ColorGray))
			}
		}
	}
}

func (e *editor) Render() {
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
	endLine := e.startLine + e.PageSize() - 1
	if endLine > len(e.buf) {
		endLine = len(e.buf)
	}
	e.lineBar.endLine = endLine
	e.lineBar.render()

	for y := e.by1; y <= e.by2; y++ {
		for x := e.bx1; x <= e.bx2; x++ {
			screen.SetContent(x, y, ' ', nil, e.style)
		}
	}

	for i := 0; i < e.PageSize(); i++ {
		if e.startLine-1+i >= len(e.buf) {
			break
		}
		e.renderLine(e.startLine + i)
	}
}

func (e *editor) cursorLineAdd(delta int) {
	if delta == 0 {
		return
	}

	line := e.line + delta
	if line < 1 {
		line = 1
	} else if line > len(e.buf) {
		line = len(e.buf)
	}
	e.line = line

	if e.column > len(e.buf[e.line-1])+1 {
		e.column = len(e.buf[e.line-1]) + 1
	}
}

func (e *editor) cursorColAdd(delta int) {
	col := e.column + delta
	if 1 <= col && col <= len(e.buf[e.line-1])+1 {
		e.column = col
		return
	}

	// line start
	if col < 1 {
		if e.line == 1 {
			e.column = 1
			return
		}
		// to the end of the previous line
		e.line--
		e.column = len(e.buf[e.line-1]) + 1
		return
	}

	// line end
	if e.line == len(e.buf) {
		e.column = len(e.buf[e.line-1]) + 1
		return
	}
	e.line++
	e.column = 1
}

func (e *editor) setCursor(x, y int) {
	if y < e.by1 {
		y = e.by1
	}
	line := y - e.by1 + e.startLine
	if line > len(e.buf) {
		line = len(e.buf)
	}
	col := x - e.bx1 + 1
	tabs := leadingTabs(e.buf[line-1])
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

	if col > len(e.buf[line-1])+1 {
		col = len(e.buf[line-1]) + 1
	}
	e.line, e.column = line, col
}

func (e *editor) ShowCursor() {
	tabs := leadingTabs(e.buf[e.line-1])
	var x int
	if e.column <= tabs {
		x = e.bx1 + (e.column-1)*tabWidth
	} else {
		padding := tabs * (tabWidth - 1)
		x = e.bx1 + e.column + padding - 1
	}

	screen.ShowCursor(x, e.by1+e.line-e.startLine)
}

func (e *editor) cursorUp() {
	if e.line == 1 {
		return
	}
	if e.line == e.startLine {
		e.startLine--
		e.cursorLineAdd(-1)
		e.Render()
		return
	}
	e.cursorLineAdd(-1)
}

func (e *editor) cursorDown() {
	if e.line == len(e.buf) {
		// end of file
		return
	}

	if e.line < e.startLine+e.PageSize()-1 {
		e.cursorLineAdd(1)
		return
	}

	e.startLine++
	e.cursorLineAdd(1)
	e.Render()
}

// go to the first non-whitespace character in line
func (e *editor) cursorLineStart() {
	for i, c := range e.buf[e.line-1] {
		if c != ' ' && c != '\t' {
			e.column = i + 1
			return
		}
	}
	e.column = 1
}

func (e *editor) cursorLineEnd() {
	if e.column == len(e.buf[e.line-1])+1 {
		return
	}
	e.cursorColAdd(len(e.buf[e.line-1]) - e.column + 1)
}

func (e *editor) scrollUp(delta int) {
	if e.startLine == 1 {
		return
	}
	if e.startLine-delta < 1 {
		e.startLine = 1
	} else {
		e.startLine -= delta
	}
	e.Render()
}

func (e *editor) scrollDown(delta int) {
	if e.startLine == len(e.buf)-e.PageSize()+1 {
		return
	}
	if e.startLine > len(e.buf)-e.PageSize()+1 {
		e.startLine = len(e.buf) - e.PageSize() + 1
	} else {
		e.startLine += delta
	}
	e.Render()
}

func (e *editor) write(r rune) {
	text := e.buf[e.line-1]
	rs := make([]rune, len(text[e.column-1:]))
	copy(rs, text[e.column-1:])
	text = append(append(text[:e.column-1], r), rs...)
	e.buf[e.line-1] = text
	e.renderLine(e.line)
	e.cursorColAdd(1)
	e.dirty = true
}

// Line return current line number in editor
func (e *editor) Line() int {
	return e.line
}

// Column return current column number in editor.
// Note that it is intended for the statusBar,
// instead of editor buffer.
func (e *editor) Column() int {
	var col int
	tabs := leadingTabs(e.buf[e.line-1])
	if e.column <= tabs {
		col = (e.column-1)*tabWidth + 1
	} else {
		padding := tabs * (tabWidth - 1)
		col = e.column + padding
	}
	return col
}

func (e *editor) deleteLeft() {
	e.dirty = true
	// cursor at line start, merge the line to previous one
	if e.column == 1 {
		if e.line == 1 {
			return
		}
		prevLine := e.buf[e.line-2]
		e.buf[e.line-2] = append(prevLine, e.buf[e.line-1]...)
		e.buf = append(e.buf[:e.line-1], e.buf[e.line:]...)
		e.cursorLineAdd(-1)
		e.cursorColAdd(1 + len(prevLine) - e.column)
		e.Render()
		return
	}

	text := e.buf[e.line-1]
	if e.column-1 == len(text) {
		// line end
		text = text[:e.column-2]
	} else {
		// TODO: maybe slices.Delete ?
		text = append(text[:e.column-2], text[e.column-1:]...)
	}
	e.buf[e.line-1] = text
	e.renderLine(e.line)
	e.cursorColAdd(-1)
}

func (e *editor) deleteToLineStart() {
	e.dirty = true
	e.buf[e.line-1] = e.buf[e.line-1][e.column-1:]
	e.renderLine(e.line)
	e.cursorLineStart()
}

func (e *editor) deleteToLineEnd() {
	e.dirty = true
	e.buf[e.line-1] = e.buf[e.line-1][:e.column-1]
	e.renderLine(e.line)
}

func (e *editor) cursorEnter() {
	e.dirty = true
	// cut current line
	text := e.buf[e.line-1]
	newText := make([]rune, len(text[e.column-1:]))
	copy(newText, text[e.column-1:])
	e.buf[e.line-1] = text[:e.column-1]
	// TODO: not efficient
	buf := make([][]rune, len(e.buf[e.line:]))
	for i, rs := range e.buf[e.line:] {
		buf[i] = make([]rune, len(rs))
		copy(buf[i], rs)
	}
	e.buf = append(append(e.buf[:e.line], newText), buf...)
	e.cursorLineAdd(1)
	e.cursorLineStart()
	e.Render()
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
	e.findLine = e.line

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
	defer e.Render()

	// jump to the nearest matching line
	var minGap = len(e.buf)
	var near int
	for i, m := range match {
		gap := m[0] + 1 - e.findLine
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

	e.line = match[near][0] + 1
	// place the cursor at the end of the matching word for easy editing
	e.column = match[near][1] + len(e.findKey) + 1
	if e.line < e.startLine {
		e.startLine = e.line
	} else if e.line > (e.startLine + e.PageSize() - 1) {
		e.startLine = e.line - e.PageSize()/2
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
	e.line = i + 1
	if e.line < e.startLine {
		e.startLine = e.line
	} else if e.line > (e.startLine + e.PageSize() - 1) {
		e.startLine = e.line - e.PageSize()/2
	}
	// place the cursor at the end of the matching word for easy editing
	e.column = j + len(e.findKey) + 1
	e.Render()
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
	e.line = i + 1
	if e.line < e.startLine {
		e.startLine = e.line
	} else if e.line > (e.startLine + e.PageSize() - 1) {
		e.startLine = e.line - e.PageSize()/2
	}
	// place the cursor at the end of the matching word for easy editing
	e.column = j + len(e.findKey) + 1
	e.Render()
}

func (e *editor) HandleEvent(ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventMouse:
		x, y := ev.Position()
		switch ev.Buttons() {
		case tcell.Button1, tcell.Button2:
			e.setCursor(x, y)
		case tcell.WheelUp:
			delta := int(float32(y) * wheelScrollSensitivity)
			e.scrollUp(delta)
		case tcell.WheelDown:
			delta := int(float32(y) * wheelScrollSensitivity)
			e.scrollDown(delta)
		}
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyPgUp:
			e.scrollUp(e.PageSize() - 1)
			e.cursorLineAdd(-(e.PageSize() - 1))
		case tcell.KeyPgDn:
			e.scrollDown(e.PageSize() - 1)
			e.cursorLineAdd(e.PageSize() - 1)
		case tcell.KeyHome:
			e.cursorLineStart()
		case tcell.KeyEnd:
			e.cursorLineEnd()
		case tcell.KeyUp:
			e.cursorUp()
		case tcell.KeyDown:
			e.cursorDown()
		case tcell.KeyLeft:
			e.cursorColAdd(-1)
		case tcell.KeyRight:
			e.cursorColAdd(1)
		case tcell.KeyRune:
			e.write(ev.Rune())
		case tcell.KeyTab:
			e.write('\t')
		case tcell.KeyEnter:
			e.cursorEnter()
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			e.deleteLeft()
		case tcell.KeyCtrlU:
			e.deleteToLineStart()
		case tcell.KeyCtrlK:
			e.deleteToLineEnd()
		case tcell.KeyESC:
			// if _, ok := e.jo.statusBar.(*findBar); ok {
			// 	e.ClearFind()
			// 	e.Render()
			// }
			// if _, ok := e.jo.statusBar.(*statusBar); !ok {
			// 	e.jo.statusBar = newStatusBar(e.jo)
			// }
		}
	}
}

func (e *editor) Pos() (x1, y1, x2, y2 int) { return e.x1, e.y1, e.x2, e.y2 }
func (e *editor) LostFocus() {
	// TODO: format
}

type lineBar struct {
	x1, y1    int
	x2, y2    int
	style     tcell.Style
	startLine int
	endLine   int
}

func (b *lineBar) render() {
	for y := b.y1; y <= b.y2; y++ {
		for x := b.x1; x <= b.x2; x++ {
			screen.SetContent(x, y, ' ', nil, b.style)
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
			screen.SetContent(b.x2-(len(s)-j)-paddingRight, y+i-b.startLine, c, nil, b.style)
		}
	}
}
