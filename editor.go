package main

import (
	"bytes"
	"fmt"
	"go/token"
	"io"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

// TODO: undo/redo changes, aka editing history
type Editor struct {
	BaseView
	screen tcell.Screen
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

	lastPos map[string][3]int // filename: [startline, line, column]

	titleBar *titleBar
	status   *bindStr
	suggest  *suggestion

	clickCount *struct {
		x, y  int
		n     int
		since time.Time
	}
	// select e.buf[e.selection.line-1][e.selection.startCol-1:e.selection.endCol-1]
	selection *struct {
		line     int
		startCol int
		endCol   int
	}
}

func (e *Editor) Click(x, y int) {
	if inView(e.titleBar, x, y) {
		lastNameIdx := e.titleBar.index
		e.titleBar.Click(x, y)
		if e.titleBar.index != lastNameIdx {
			e.Load(e.titleBar.names[e.titleBar.index])
			e.Draw(e.screen)
		}
		return
	}

	e.BaseView.Click(x, y)
	e.setCursor(x, y)

	if e.clickCount == nil || e.clickCount.x != x || e.clickCount.y != y ||
		time.Since(e.clickCount.since) > time.Second/2 {
		e.clickCount = &struct {
			x, y  int
			n     int
			since time.Time
		}{
			x: x, y: y, since: time.Now(), n: 1,
		}
		// restore the previous selected characters
		if e.selection != nil {
			line := e.selection.line
			e.selection = nil
			e.drawLine(e.screen, line)
		}
		return
	}

	e.clickCount.n++
	switch e.clickCount.n {
	case 2:
		// double-click expands selection to a word
		tokens := parseToken(e.buf[e.line-1])
		for _, t := range tokens {
			if t.off <= e.column-1 && e.column-1 < (t.off+t.len) {
				e.selection = &struct {
					line     int
					startCol int
					endCol   int
				}{
					line:     e.line,
					startCol: t.off + 1,
					endCol:   t.off + t.len + 1,
				}
				e.column = e.selection.endCol
				e.updateCursorPos()
				break
			}
		}
	case 3:
		// triple-click expands selection to a line
		e.selection = &struct {
			line     int
			startCol int
			endCol   int
		}{
			line:     e.line,
			startCol: 1,
			endCol:   len(e.buf[e.line-1]) + 1,
		}
		e.column = e.selection.endCol
		e.updateCursorPos()
	default:
		// cancel selection
		e.clickCount.n = 1
		e.selection = nil
	}
	e.drawLine(e.screen, e.line)
}

func (e *Editor) Focus() (int, int) {
	recentE = e
	return e.BaseView.Focus()
}

var tokenTree = new(node)

func newEditor(screen tcell.Screen, filename string, status *bindStr) *Editor {
	style := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	e := &Editor{
		screen:    screen,
		style:     style,
		startLine: 1,
		line:      1,
		column:    1,
		filename:  filename,
		lineBar: &lineBar{
			style: style.Foreground(tcell.ColorGray),
		},
		lastPos: make(map[string][3]int),
		status:  status,
	}

	e.titleBar = newTitleBar(e, filename)

	if filename == "" {
		e.buf = append(e.buf, []rune{})
		return e
	}

	var a [][]byte
	src, err := os.ReadFile(filename)
	if err != nil {
		log.Println(err)
		e.buf = append(e.buf, []rune{})
		return e
	}
	a = bytes.Split(src, []byte{'\n'})
	e.buf = make([][]rune, len(a))
	for i := range a {
		e.buf[i] = []rune(string(a[i]))
	}

	if len(e.buf) > 0 {
		buildTokenTree(tokenTree, e.buf)
	}
	// file ends with a new line
	if len(e.buf) == 0 || len(e.buf[len(e.buf)-1]) != 0 {
		e.buf = append(e.buf, []rune{})
	}
	return e
}

// the number of lines visible in the editor view
func (e *Editor) PageSize() int { return e.by2 - e.by1 + 1 }

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

func (e *Editor) drawLine(screen tcell.Screen, line int) {
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
	var matches [][2]int
	for _, m := range e.findMatch {
		if m[0] == line-1 {
			matches = append(matches, m)
		}
	}

	tokenInfo := parseToken(text)
	i := 0
	tabs := leadingTabs(text)
	padding := 0
	_, bg, _ := e.style.Decompose()
	style := tokenInfo[i].Style().Background(bg)
	for j := range text {
		if e.bx1+padding+j > e.bx2 {
			break
		}
		if j >= tokenInfo[i].off+tokenInfo[i].len && i < len(tokenInfo)-1 {
			i++
			style = tokenInfo[i].Style().Background(bg)
		}

		// highlight search results
		if len(matches) > 0 && mi < len(matches) {
			if matches[mi][1] <= j && j < matches[mi][1]+len(e.findKey) {
				if matches[mi] == e.findMatch[e.findIndex] {
					style = style.Background(tcell.ColorYellow)
				} else {
					style = style.Background(tcell.ColorLightGray)
				}
			} else if j >= matches[mi][1]+len(e.findKey) {
				mi++
				// restore
				style = style.Background(bg)
			}
		}

		// highlight selection
		if e.selection != nil && e.selection.line == line && e.selection.startCol-1 <= j && j <= e.selection.endCol-1 {
			style = style.Background(tcell.ColorLightGray)
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

func (e *Editor) Draw(screen tcell.Screen) {
	e.titleBar.Draw(screen)
	lineBarWidth := 2
	for i := len(e.buf); i > 0; i = i / 10 {
		lineBarWidth++
	}
	e.lineBar.x1 = e.x
	e.lineBar.y1 = e.y + e.titleBar.height
	e.lineBar.x2 = e.x + lineBarWidth
	e.lineBar.y2 = e.y + e.height - 1

	e.bx1 = e.x + lineBarWidth
	e.by1 = e.y + e.titleBar.height
	e.bx2 = e.x + e.width - 1
	e.by2 = e.y + e.height - 1

	e.updateCursorPos()
	if e.focused {
		screen.ShowCursor(e.cursorX, e.cursorY)
	}

	e.lineBar.startLine = e.startLine
	endLine := e.startLine + e.PageSize() - 1
	if endLine > len(e.buf) {
		endLine = len(e.buf)
	}
	e.lineBar.endLine = endLine
	e.lineBar.render(screen)

	for y := e.by1; y <= e.by2; y++ {
		for x := e.bx1; x <= e.bx2; x++ {
			screen.SetContent(x, y, ' ', nil, e.style)
		}
	}

	for i := 0; i < e.PageSize(); i++ {
		if e.startLine-1+i >= len(e.buf) {
			break
		}
		e.drawLine(screen, e.startLine+i)
	}
}

func (e *Editor) cursorLineAdd(delta int) {
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
	e.updateCursorPos()
}

func (e *Editor) updateCursorPos() {
	e.status.Set(fmt.Sprintf("line %d, column %d", e.line, e.Column()))
	e.cursorX = e.bx1 + e.Column() - 1
	e.cursorY = e.by1 + e.line - e.startLine
}

func (e *Editor) cursorColAdd(delta int) {
	defer e.updateCursorPos()

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

func (e *Editor) setCursor(x, y int) {
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
	e.updateCursorPos()
}

func (e *Editor) cursorUp() (redraw bool) {
	if e.line == 1 {
		return false
	}

	defer e.updateCursorPos()
	if e.line == e.startLine {
		e.startLine--
		e.cursorLineAdd(-1)
		return true
	}
	e.cursorLineAdd(-1)
	return false
}

func (e *Editor) cursorDown() (redraw bool) {
	if e.line == len(e.buf) {
		// end of file
		return false
	}

	defer e.updateCursorPos()
	if e.line < e.startLine+e.PageSize()-1 {
		e.cursorLineAdd(1)
		return false
	}

	e.startLine++
	e.cursorLineAdd(1)
	return true
}

// go to the first non-whitespace character in line
func (e *Editor) cursorLineStart() {
	defer e.updateCursorPos()
	for i, c := range e.buf[e.line-1] {
		if c != ' ' && c != '\t' {
			e.column = i + 1
			return
		}
	}
	e.column = 1
}

func (e *Editor) cursorLineEnd() {
	if e.column == len(e.buf[e.line-1])+1 {
		return
	}
	e.cursorColAdd(len(e.buf[e.line-1]) - e.column + 1)
}

func (e *Editor) ScrollUp(delta int) (ok bool) {
	if e.startLine == 1 {
		return false
	}
	if e.startLine-delta < 1 {
		e.startLine = 1
	} else {
		e.startLine -= delta
	}
	return true
}

func (e *Editor) ScrollDown(delta int) (ok bool) {
	if e.startLine >= len(e.buf)-e.PageSize()+1 {
		return false
	}
	e.startLine += delta
	if e.startLine >= len(e.buf)-e.PageSize()+1 {
		e.startLine = len(e.buf) - e.PageSize() + 1
	}
	return true
}

func (e *Editor) writeRune(r rune) {
	text := e.buf[e.line-1]
	rs := make([]rune, len(text[e.column-1:]))
	copy(rs, text[e.column-1:])
	text = append(append(text[:e.column-1], r), rs...)
	e.buf[e.line-1] = text
	// e.renderLine(e.line)
	e.cursorColAdd(1)
	e.dirty = true
}

func (e *Editor) writeString(s string) {
	text := e.buf[e.line-1]
	rs := make([]rune, len(text[e.column-1:]))
	copy(rs, text[e.column-1:])
	text = append(append(text[:e.column-1], []rune(s)...), rs...)
	e.buf[e.line-1] = text
	e.cursorColAdd(len(s))
	e.dirty = true
}

// Line return current line number in editor
func (e *Editor) Line() int {
	return e.line
}

// Column return current column number in editor.
// Note that it is intended for the statusBar,
// instead of editor buffer.
func (e *Editor) Column() int {
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

// deletes a character to the left of the cursor,
// or delete the selected characters.
// If redraw is true, caller should redraw editor,
// otherwise render the current line.
func (e *Editor) deleteLeft() (redraw bool) {
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
		return true
	}

	i, j := e.column-2, e.column-1
	if e.selection != nil {
		i = e.selection.startCol - 1
		j = e.selection.endCol - 1
		e.selection = nil
	}
	e.buf[e.line-1] = slices.Delete(e.buf[e.line-1], i, j)
	e.cursorColAdd(-(j - i))
	return false
}

func (e *Editor) deleteToLineStart() {
	e.dirty = true
	e.buf[e.line-1] = e.buf[e.line-1][e.column-1:]
	e.cursorLineStart()
}

func (e *Editor) deleteToLineEnd() {
	e.dirty = true
	e.buf[e.line-1] = e.buf[e.line-1][:e.column-1]
}

func (e *Editor) cursorEnter() {
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
}

// A newline is appended if the last character of buffer is not
// already a newline
func (e *Editor) WriteTo(w io.Writer) (int64, error) {
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
	buildTokenTree(tokenTree, e.buf)
	return int64(n), nil
}

func (e *Editor) Find(s string) {
	if len(s) == 0 {
		return
	}
	e.findKey = s

	var match [][2]int
	for i := range e.buf {
		var index, start int
		for {
			index = strings.Index(string(e.buf[i][start:]), e.findKey)
			if index < 0 {
				break
			}
			match = append(match, [2]int{i, start + index})
			start += index + len(e.findKey)
		}
	}
	e.findMatch = match
	if len(match) == 0 {
		return
	}

	// jump to the nearest match
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

func (e *Editor) FindNext() {
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
}

func (e *Editor) FindPrev() {
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
}

func (e *Editor) HandleEventKey(ev *tcell.EventKey, screen tcell.Screen) {
	defer func() {
		if e.focused {
			screen.ShowCursor(e.cursorX, e.cursorY)
		}
	}()
	switch ev.Key() {
	case tcell.KeyPgUp:
		if e.ScrollUp(e.PageSize() - 1) {
			e.Draw(screen)
		}
		e.cursorLineAdd(-(e.PageSize() - 1))
	case tcell.KeyPgDn:
		if e.ScrollDown(e.PageSize() - 1) {
			e.Draw(screen)
		}
		e.cursorLineAdd(e.PageSize() - 1)
	case tcell.KeyHome:
		e.cursorLineStart()
	case tcell.KeyEnd:
		e.cursorLineEnd()
	case tcell.KeyUp:
		if e.suggest == nil {
			if e.cursorUp() {
				e.Draw(screen)
			}
			return
		}

		if e.suggest.up {
			e.suggest.i++
		} else {
			e.suggest.i--
		}

		if e.suggest.i == -1 {
			e.suggest.i = len(e.suggest.options) - 1
		} else if e.suggest.i == len(e.suggest.options) {
			e.suggest.i = 0
		}
		e.showSuggestion(screen)
	case tcell.KeyDown:
		if e.suggest == nil {
			if e.cursorDown() {
				e.Draw(screen)
			}
			return
		}

		if !e.suggest.up {
			e.suggest.i++
		} else {
			e.suggest.i--
		}
		if e.suggest.i == -1 {
			e.suggest.i = len(e.suggest.options) - 1
		} else if e.suggest.i == len(e.suggest.options) {
			e.suggest.i = 0
		}
		e.showSuggestion(screen)
	case tcell.KeyLeft:
		e.cursorColAdd(-1)
	case tcell.KeyRight:
		e.cursorColAdd(1)
	case tcell.KeyRune:
		if e.selection != nil {
			e.deleteLeft()
		}
		e.writeRune(ev.Rune())
		e.drawLine(screen, e.line)
		if e.suggest != nil {
			e.Draw(screen) // clear previous suggestions
			if e.loadSuggestion() {
				e.showSuggestion(screen)
			}
		}
	case tcell.KeyTab:
		// insert '\t' at the head of line
		if e.column == 1 || e.buf[e.line-1][e.column-2] == '\t' {
			e.writeRune('\t')
			e.drawLine(screen, e.line)
			return
		}

		// on second <tab>, accept the first suggestion
		if e.suggest != nil {
			e.accecptSuggestion()
			e.Draw(screen)
			return
		}

		// on first <tab>, show suggestions
		if e.loadSuggestion() {
			if len(e.suggest.options) == 1 {
				e.accecptSuggestion()
				e.drawLine(screen, e.line)
			} else {
				e.showSuggestion(screen)
			}
		}
	case tcell.KeyEnter:
		if e.suggest != nil {
			e.accecptSuggestion()
			e.Draw(screen)
			return
		}
		e.cursorEnter()
		e.Draw(screen)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if e.deleteLeft() {
			e.Draw(screen)
			return
		}
		e.drawLine(screen, e.line)

		if e.suggest != nil {
			e.Draw(screen) // clear previous suggestions
			if e.loadSuggestion() {
				e.showSuggestion(screen)
			}
		}
	case tcell.KeyCtrlU:
		e.deleteToLineStart()
		e.drawLine(screen, e.line)
	case tcell.KeyCtrlK:
		e.deleteToLineEnd()
		e.drawLine(screen, e.line)
	case tcell.KeyESC:
		if e.suggest != nil {
			e.suggest = nil
			e.Draw(screen)
			return
		}
		if e.findMatch != nil {
			e.ClearFind()
			e.Draw(screen)
			return
		}
	}
}

type suggestion struct {
	x, y    int
	options []string
	i       int
	up      bool
}

func (e *Editor) loadSuggestion() bool {
	prevWord := string(getToken(e.buf[e.line-1], e.column-2))
	if len(prevWord) == 0 {
		e.suggest = nil
		return false
	}

	tokens := tokenTree.get(prevWord)
	if len(tokens) == 0 {
		e.suggest = nil
		return false
	}

	if len(tokens) > 10 {
		// TODO: adjust the number
		tokens = tokens[:10]
	}
	e.suggest = &suggestion{
		x:       e.cursorX - len(prevWord),
		y:       e.cursorY,
		options: tokens,
	}
	return true
}

func (e *Editor) showSuggestion(screen tcell.Screen) {
	if len(e.suggest.options) == 0 {
		return
	}
	optionY := func(i int) int {
		var yy int
		if e.by2-e.suggest.y >= len(e.suggest.options) {
			yy = e.suggest.y + 1 + i // list down
		} else {
			e.suggest.up = true
			yy = e.suggest.y - 1 - i // list up
		}
		return yy
	}
	for i := range e.suggest.options {
		style := tcell.StyleDefault.Background(tcell.ColorLightGray).Foreground(tcell.ColorBlack)
		if i == e.suggest.i {
			style = style.Background(tcell.ColorLightBlue)
		}
		oy := optionY(i)
		for j, c := range e.suggest.options[i] {
			screen.SetContent(e.suggest.x+j, oy, c, nil, style)
		}
		for padding := optionWidth - len(e.suggest.options[i]); padding > 0; padding-- {
			screen.SetContent(e.suggest.x+optionWidth-padding, oy, ' ', nil, style)
		}
	}
}

func (e *Editor) accecptSuggestion() {
	word := string(getToken(e.buf[e.line-1], e.column-2))
	e.buf[e.line-1] = e.buf[e.line-1][:e.column-1-len(word)]
	e.cursorColAdd(-len(word))
	e.writeString(e.suggest.options[e.suggest.i])
	e.suggest = nil
	// e.Draw() // TODO: no need to refresh the whole screen
}

func (e *Editor) ClearFind() {
	e.findLine = 0
	e.findKey = ""
	e.findMatch = nil
	e.findIndex = 0
}

func (e *Editor) SetPos(x, y, width, height int) {
	e.titleBar.SetPos(x, y, width, 1)
	e.x = x
	e.y = y
	e.width = width
	e.height = height
}

func (e *Editor) Load(filename string) {
	if filename == "" {
		return
	}
	if filename == e.filename {
		return
	}

	src, err := os.ReadFile(filename)
	if err != nil {
		log.Println(err)
		return
	}
	a := bytes.Split(src, []byte{'\n'})

	e.buf = e.buf[:0]
	for i := range a {
		e.buf = append(e.buf, []rune(string(a[i])))
	}
	if len(e.buf) > 0 {
		buildTokenTree(tokenTree, e.buf)
	}
	// file ends with a new line
	if len(e.buf) == 0 || len(e.buf[len(e.buf)-1]) != 0 {
		e.buf = append(e.buf, []rune{})
	}

	// restore cursor position
	e.lastPos[e.filename] = [3]int{e.startLine, e.line, e.column}
	e.filename = filename
	if pos, ok := e.lastPos[filename]; ok {
		e.startLine = pos[0]
		e.line = pos[1]
		e.column = pos[2]
	} else {
		e.startLine = 1
		e.line = 1
		e.column = 1
	}

	e.dirty = false
	e.suggest = nil
	e.titleBar.Add(filename)
	e.status.Set(fmt.Sprintf("line %d, column %d", e.line, e.Column()))
}

type lineBar struct {
	x1, y1    int
	x2, y2    int
	style     tcell.Style
	startLine int
	endLine   int
}

func (b *lineBar) render(screen tcell.Screen) {
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

func buildTokenTree(tree *node, buf [][]rune) {
	if tree == nil || len(buf) == 0 {
		return
	}
	for i := range buf {
		infos := parseToken(buf[i])
		for _, info := range infos {
			t := string(buf[i][info.off : info.off+info.len])
			if token.IsKeyword(t) || token.IsIdentifier(t) {
				tree.set(t)
			}
		}
	}
}
