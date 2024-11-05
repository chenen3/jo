package main

import (
	"bytes"
	"fmt"
	"go/token"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

type EditorGroup struct {
	BaseView
	screen   tcell.Screen
	editor   *Editor
	titleBar *titleBar
	status   *bindStr
	all      []*Editor
}

func NewEditorGroup(screen tcell.Screen, status *bindStr) *EditorGroup {
	g := &EditorGroup{
		screen: screen,
		status: status,
	}
	// blank editor
	g.editor = newEditor(screen, "", status)
	g.titleBar = newTitleBar("")
	return g
}

func (g *EditorGroup) Click(x, y int) {
	if inView(g.titleBar, x, y) {
		i := g.titleBar.i
		g.titleBar.Click(x, y)
		if g.titleBar.i != i {
			g.Open(g.titleBar.names[g.titleBar.i])
			g.Draw(g.screen)
		}
		return
	}

	g.editor.Click(x, y)
	g.cursorX = g.editor.cursorX
	g.cursorY = g.editor.cursorY
}

func (g *EditorGroup) ScrollUp(delta int) (ok bool) {
	return g.editor.ScrollUp(delta)
}

func (g *EditorGroup) ScrollDown(delta int) (ok bool) {
	return g.editor.ScrollDown(delta)
}

func (g *EditorGroup) HandleEventKey(ev *tcell.EventKey, screen tcell.Screen) {
	g.editor.HandleEventKey(ev, screen)
}

func (g *EditorGroup) Focus() (int, int) {
	recentE = g
	g.BaseView.Focus()
	return g.editor.Focus()
}

func (g *EditorGroup) SetPos(x, y, width, height int) {
	g.x = x
	g.y = y
	g.width = width
	g.height = height
	g.titleBar.SetPos(x, y, width, 1)
	g.editor.SetPos(x, y+g.titleBar.height, width, height-g.titleBar.height)
}

func (g *EditorGroup) Draw(screen tcell.Screen) {
	g.titleBar.Draw(screen)
	g.editor.Draw(screen)
}

func (g *EditorGroup) Open(name string) {
	if g.editor.filename == name {
		return
	}

	for _, e := range g.all {
		if e.filename == name {
			g.editor = e
			return
		}
	}

	e := newEditor(g.screen, name, g.status)
	e.SetPos(g.editor.x, g.editor.y, g.editor.width, g.editor.height)
	g.editor = e
	g.all = append(g.all, e)
	g.titleBar.Add(name)
}

func (g *EditorGroup) CloseOne() {
	t := g.titleBar
	if len(t.names) == 0 {
		return
	}

	oldname := t.names[t.i]
	t.Del()
	if len(t.names) == 0 {
		// reset
		g.editor = newEditor(g.screen, "", g.status)
		g.all = nil
		return
	}

	var old int
	for i, e := range g.all {
		if e.filename == t.names[t.i] {
			g.editor = e
		}
		if e.filename == oldname {
			old = i
		}
	}
	g.all = slices.Delete(g.all, old, old+1)
}

type Editor struct {
	BaseView
	screen tcell.Screen
	style  tcell.Style

	// editing buffer
	buf      [][]rune
	bx1, by1 int
	bx2, by2 int
	top      int // top line number, starting at 1
	cursor   pos // write at buf[cursor.row][cursor.col]
	dirty    bool
	filename string

	lineBar *lineBar
	status  *bindStr

	suggest *suggestion

	find find

	clickCount *struct {
		x, y  int
		n     int
		since time.Time
	}
	selection *struct {
		start pos
		stop  pos
	}

	history     []Action // stack of changes, for undo
	historyUndo []Action // for redo
}

type find struct {
	key   string
	line  int
	match [][2]int
	index int // index of the matching result
}

func (e *Editor) Click(x, y int) {
	e.BaseView.Click(x, y)
	if y < e.by1 {
		y = e.by1
	}
	line := y - e.by1 + e.top
	if line > len(e.buf) {
		line = len(e.buf)
	}
	col := x - e.bx1 + 1
	tabs := leadingTabs(e.buf[line-1])
	if col <= tabs*tabSize {
		i, j := col/tabSize, col%tabSize
		// When cursor is over half of the tabWidth,
		// will be moved to the next tab.
		if j > tabSize/2 {
			col = i + 2
		} else {
			col = i + 1
		}
	} else {
		col -= tabs * (tabSize - 1)
	}

	if col > len(e.buf[line-1])+1 {
		col = len(e.buf[line-1]) + 1
	}
	e.cursor = pos{line - 1, col - 1}
	defer e.syncCursor()

	if e.clickCount == nil || e.clickCount.x != x || e.clickCount.y != y ||
		time.Since(e.clickCount.since) > time.Second*2/3 {
		e.clickCount = &struct {
			x, y  int
			n     int
			since time.Time
		}{
			x: x, y: y, since: time.Now(), n: 1,
		}
		// restore the previous selected characters
		if e.selection != nil {
			line := e.selection.start.row + 1
			e.selection = nil
			e.drawLine(e.screen, line)
		}
		return
	}

	e.clickCount.n++
	switch e.clickCount.n {
	case 2:
		// double-click expands selection to a word
		tokens := parseToken(e.buf[e.cursor.row])
		for _, t := range tokens {
			if t.off <= e.cursor.col && e.cursor.col < (t.off+t.len) {
				e.selection = &struct {
					start, stop pos
				}{
					start: pos{e.cursor.row, t.off},
					stop:  pos{e.cursor.row, t.off + t.len},
				}
				e.cursor.col = e.selection.stop.col
				break
			}
		}
	case 3:
		// triple-click expands selection to a line
		e.selection = &struct {
			start, stop pos
		}{
			start: pos{e.cursor.row, 0},
			stop:  pos{e.cursor.row, len(e.buf[e.cursor.row])},
		}
		e.cursor.col = e.selection.stop.col
	default:
		// cancel selection
		e.clickCount.n = 1
		e.selection = nil
	}
	e.drawLine(e.screen, e.cursor.row+1)
}

var tokenTree = new(node)

func newEditor(screen tcell.Screen, filename string, status *bindStr) *Editor {
	style := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	e := &Editor{
		screen:   screen,
		style:    style,
		top:      1,
		filename: filename,
		lineBar:  new(lineBar),
		status:   status,
	}

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

// the number of spaces a tab is equal to
const tabSize = 4

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

// calculate the index of column after tab conversion.
// Note that it is intended for the statusBar.
func padCol(line []rune, col int) int {
	var i int
	nTab := leadingTabs(line)
	if col < nTab {
		i = (col) * tabSize
	} else {
		padding := nTab * (tabSize - 1)
		i = col + padding
	}
	return i
}

// calculate the index of column after removing the tab padding.
func unpadCol(line []rune, padCol int) int {
	if len(line) == 0 {
		return 0
	}
	var col int
	n := leadingTabs(line)
	if padCol < n*tabSize {
		col = padCol / tabSize
	} else {
		padding := n * (tabSize - 1)
		col = padCol - padding
		if col > len(line)-1 {
			col = len(line) - 1
		}
	}
	return col
}

// draw a row of the buffer, parameter line is the line number starting from 1
func (e *Editor) drawLine(screen tcell.Screen, line int) {
	text := e.buf[line-1]
	for x := e.bx1; x <= e.bx2; x++ {
		if x <= e.bx1+len(text)-1 {
			continue
		}
		screen.SetContent(x, e.by1+line-e.top, ' ', nil, e.style)
	}

	if len(text) == 0 {
		return
	}

	var mi int
	var matches [][2]int
	for _, m := range e.find.match {
		if m[0] == line-1 {
			matches = append(matches, m)
		}
	}

	var style = e.style
	var i int
	var tokenInfo []tokenInfo
	if filepath.Ext(e.filename) == ".go" {
		tokenInfo = parseToken(text)
	}

	tabs := leadingTabs(text)
	padding := 0
	_, bg, _ := e.style.Decompose()
	for j := range text {
		if e.bx1+padding+j > e.bx2 {
			break
		}
		if len(tokenInfo) > 0 {
			if j >= tokenInfo[i].off+tokenInfo[i].len && i < len(tokenInfo)-1 {
				i++
			}
			style = tokenInfo[i].Style().Background(bg)
		}

		// highlight search results
		if len(matches) > 0 && mi < len(matches) {
			if matches[mi][1] <= j && j < matches[mi][1]+len(e.find.key) {
				if matches[mi] == e.find.match[e.find.index] {
					style = style.Background(tcell.ColorYellow)
				} else {
					style = style.Background(tcell.ColorLightGray)
				}
			} else if j >= matches[mi][1]+len(e.find.key) {
				mi++
				// restore
				style = style.Background(bg)
			}
		}

		// highlight selection
		if e.selection != nil && e.selection.start.row+1 == line && e.selection.start.col <= j && j <= e.selection.stop.col {
			style = style.Background(tcell.ColorLightGray)
		}

		screen.SetContent(e.bx1+padding+j, e.by1+line-e.top, text[j], nil, style)
		if j < tabs {
			// consider showing tab as '|' for debugging
			screen.SetContent(e.bx1+padding+j, e.by1+line-e.top, ' ', nil, e.style.Foreground(tcell.ColorGray))
			for k := 0; k < tabSize-1; k++ {
				padding++
				screen.SetContent(e.bx1+padding+j, e.by1+line-e.top, ' ', nil, e.style.Foreground(tcell.ColorGray))
			}
		}
	}
}

func (e *Editor) Draw(screen tcell.Screen) {
	lineBarWidth := 2
	for i := len(e.buf); i > 0; i = i / 10 {
		lineBarWidth++
	}
	e.lineBar.SetPos(e.x, e.y, lineBarWidth, e.height)

	e.bx1 = e.x + lineBarWidth
	e.by1 = e.y
	e.bx2 = e.x + e.width - 1
	e.by2 = e.y + e.height - 1

	e.syncCursor()
	if e.focused {
		screen.ShowCursor(e.cursorX, e.cursorY)
	}

	e.lineBar.top = e.top
	bottom := e.top + e.PageSize() - 1
	if bottom > len(e.buf) {
		bottom = len(e.buf)
	}
	e.lineBar.bottom = bottom
	e.lineBar.Draw(screen)

	for y := e.by1; y <= e.by2; y++ {
		for x := e.bx1; x <= e.bx2; x++ {
			screen.SetContent(x, y, ' ', nil, e.style)
		}
	}

	for i := 0; i < e.PageSize(); i++ {
		if e.top-1+i >= len(e.buf) {
			break
		}
		e.drawLine(screen, e.top+i)
	}
}

// use buffer cursor to update screen cursor and status bar
func (e *Editor) syncCursor() {
	padCol := padCol(e.buf[e.cursor.row], e.cursor.col)
	e.status.Set(fmt.Sprintf("line %d, column %d", e.cursor.row+1, padCol+1))
	e.cursorX = e.bx1 + padCol
	e.cursorY = e.by1 + e.cursor.row + 1 - e.top
}

func (e *Editor) moveUp() (redraw bool) {
	if e.cursor.row == 0 {
		return false
	}

	if e.cursor.row+1 == e.top {
		e.top--
		redraw = true
	}
	padcol := padCol(e.buf[e.cursor.row], e.cursor.col)
	e.cursor.col = unpadCol(e.buf[e.cursor.row-1], padcol)
	e.cursor.row--
	e.syncCursor()
	return redraw
}

func (e *Editor) moveDown() (redraw bool) {
	if e.cursor.row == len(e.buf)-1 {
		return false
	}

	if e.cursor.row == e.top+e.PageSize()-2 {
		e.top++
		redraw = true
	}
	padcol := padCol(e.buf[e.cursor.row], e.cursor.col)
	e.cursor.col = unpadCol(e.buf[e.cursor.row+1], padcol)
	e.cursor.row++
	e.syncCursor()
	return redraw
}

func (e *Editor) moveLeft() {
	if e.cursor.col > 0 {
		e.cursor.col--
		e.syncCursor()
		return
	}

	// head of file
	if e.cursor.row == 0 {
		return
	}
	// head of line
	e.cursor.row--
	e.cursor.col = len(e.buf[e.cursor.row])
	e.syncCursor()
}

func (e *Editor) moveRight() {
	if e.cursor.col < len(e.buf[e.cursor.row]) {
		e.cursor.col++
		e.syncCursor()
		return
	}

	// end of file
	if e.cursor.row == len(e.buf)-1 {
		return
	}
	// end of line
	e.cursor.row++
	e.cursor.col = 0
	e.syncCursor()
}

func (e *Editor) ScrollUp(delta int) (ok bool) {
	if e.top == 1 {
		return false
	}
	if e.top-delta < 1 {
		e.top = 1
	} else {
		e.top -= delta
	}
	return true
}

func (e *Editor) ScrollDown(delta int) (ok bool) {
	if e.top >= len(e.buf)-e.PageSize()+1 {
		return false
	}
	e.top += delta
	if e.top >= len(e.buf)-e.PageSize()+1 {
		e.top = len(e.buf) - e.PageSize() + 1
	}
	return true
}

func (e *Editor) writeRune(r rune) {
	e.do(
		Insert(e, e.cursor, string([]rune{r})),
		Move(e, pos{e.cursor.row, e.cursor.col + 1}),
	)
}

func (e *Editor) writeString(s string) {
	e.do(
		Insert(e, e.cursor, s),
		Move(e, pos{e.cursor.row, e.cursor.col + len(s)}),
	)
}

// deletes a character to the left of the cursor,
// or delete the selected characters.
// If redraw is true, caller should redraw editor,
// otherwise render the current line.
func (e *Editor) deleteLeft() (redraw bool) {
	e.dirty = true
	// cursor at the head of line, so concatenate previous line
	if e.cursor.col == 0 {
		if e.cursor.row == 0 {
			return
		}
		prevLine := e.buf[e.cursor.row-1]
		e.buf[e.cursor.row-1] = append(prevLine, e.buf[e.cursor.row]...)
		e.buf = append(e.buf[:e.cursor.row], e.buf[e.cursor.row+1:]...)
		Move(e, pos{e.cursor.row - 1, len(prevLine)}).Do()
		return true
	}

	if e.selection != nil {
		e.do(Delete(e, e.selection.start, e.selection.stop), Move(e, e.selection.start))
		e.selection = nil
		return false
	}

	e.do(
		Delete(e, pos{e.cursor.row, e.cursor.col - 1}, e.cursor),
		Move(e, pos{e.cursor.row, e.cursor.col - 1}),
	)
	return false
}

func (e *Editor) delete(start, stop pos) {
	e.do(Delete(e, start, stop), Move(e, start))
}

func (e *Editor) cursorEnter() {
	line := e.buf[e.cursor.row]
	n := leadingTabs(line)
	if e.cursor.col > 0 {
		switch line[e.cursor.col-1] {
		case '(', '{', '[':
			n++
		}
	}
	indent := make([]rune, n)
	for i := 0; i < n; i++ {
		indent[i] = '\t'
	}
	// auto indent
	text := append(indent, line[e.cursor.col:]...)

	e.dirty = true
	e.buf[e.cursor.row] = line[:e.cursor.col]
	e.buf = slices.Insert(e.buf, e.cursor.row+1, text)
	Move(e, pos{e.cursor.row + 1, n}).Do()
}

// A newline is appended if the last character of buffer is not
// already a newline
func (e *Editor) WriteTo(w io.Writer) (int64, error) {
	var b bytes.Buffer
	for i := range e.buf {
		b.WriteString(string(e.buf[i]))
		if i != len(e.buf)-1 || len(e.buf[i]) != 0 {
			b.WriteString("\n")
		}
	}
	n, err := b.WriteTo(w)
	if err != nil {
		return n, err
	}

	e.dirty = false
	buildTokenTree(tokenTree, e.buf)
	return n, nil
}

func (e *Editor) Find(s string) {
	if len(s) == 0 {
		return
	}
	e.find.key = s

	var match [][2]int
	for i := range e.buf {
		var index, start int
		for {
			index = strings.Index(string(e.buf[i][start:]), s)
			if index < 0 {
				break
			}
			match = append(match, [2]int{i, start + index})
			start += index + len(s)
		}
	}
	e.find.match = match
	if len(match) == 0 {
		return
	}

	// jump to the nearest match
	var minGap = len(e.buf)
	var near int
	for i, m := range match {
		gap := m[0] - e.find.line
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
	e.find.index = near

	e.cursor.row = match[near][0]
	// place the cursor at the end of the matching word for easy editing
	e.cursor.col = match[near][1] + len(e.find.key)
	if e.top > e.cursor.row+1 {
		e.top = e.cursor.row + 1
	} else if e.top+e.PageSize() < e.cursor.row+1 {
		e.top = e.cursor.row - e.PageSize()/2
	}
}

func (e *Editor) FindNext() {
	if len(e.find.match) == 0 {
		return
	}

	if e.find.index == len(e.find.match)-1 {
		e.find.index = 0
	} else {
		e.find.index++
	}

	i, j := e.find.match[e.find.index][0], e.find.match[e.find.index][1]
	e.cursor.row = i
	if e.top > e.cursor.row+1 {
		e.top = e.cursor.row + 1
	} else if e.top+e.PageSize() < e.cursor.row+1 {
		e.top = e.cursor.row - e.PageSize()/2
	}
	// place the cursor at the end of the matching word for easy editing
	e.cursor.col = j + len(e.find.key)
}

func (e *Editor) FindPrev() {
	if len(e.find.match) == 0 {
		return
	}

	if e.find.index == 0 {
		e.find.index = len(e.find.match) - 1
	} else {
		e.find.index--
	}

	i, j := e.find.match[e.find.index][0], e.find.match[e.find.index][1]
	e.cursor.row = i
	if e.top > e.cursor.row+1 {
		e.top = e.cursor.row + 1
	} else if e.top+e.PageSize() < e.cursor.row+1 {
		e.top = e.cursor.row - e.PageSize()/2
	}
	// place the cursor at the end of the matching word for easy editing
	e.cursor.col = j + len(e.find.key)
}

func (e *Editor) HandleEventKey(ev *tcell.EventKey, screen tcell.Screen) {
	defer func() {
		if e.focused {
			screen.ShowCursor(e.cursorX, e.cursorY)
		}
	}()
	switch ev.Key() {
	case tcell.KeyPgUp:
		if !e.ScrollUp(e.PageSize() - 1) {
			return
		}
		row := e.cursor.row - e.PageSize()
		if row < 0 {
			row = 0
		}
		Move(e, pos{row, 0}).Do()
		e.Draw(screen)
	case tcell.KeyPgDn:
		if e.ScrollDown(e.PageSize() - 1) {
			return
		}
		row := e.cursor.row + e.PageSize()
		if row > len(e.buf)-1 {
			row = len(e.buf) - 1
		}
		Move(e, pos{row, 0}).Do()
		e.Draw(screen)
	case tcell.KeyHome:
		n := leadingTabs(e.buf[e.cursor.row])
		var col int
		if n > 0 {
			col = n * tabSize
		}
		// to the first non-whitespace character
		Move(e, pos{e.cursor.row, col}).Do()
	case tcell.KeyEnd:
		if e.cursor.col == len(e.buf[e.cursor.row])-1 {
			return
		}
		Move(e, pos{e.cursor.row, len(e.buf[e.cursor.row]) - 1}).Do()
	case tcell.KeyUp:
		if e.suggest == nil {
			if e.moveUp() {
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
			if e.moveDown() {
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
		e.moveLeft()
	case tcell.KeyRight:
		e.moveRight()
	case tcell.KeyRune:
		if e.selection != nil {
			e.delete(e.selection.start, e.selection.stop)
		}
		e.writeRune(ev.Rune())
		e.drawLine(screen, e.cursor.row+1)
		if e.suggest != nil {
			e.Draw(screen) // clear previous suggestions
			if e.loadSuggestion() {
				e.showSuggestion(screen)
			}
		}
	case tcell.KeyTab:
		// insert '\t' at the head of line
		if e.cursor.col == 0 || e.buf[e.cursor.row][e.cursor.col-1] == '\t' {
			e.writeRune('\t')
			e.drawLine(screen, e.cursor.row+1)
			return
		}

		// on second <tab>, accept suggestion
		if e.suggest != nil {
			e.accecptSuggestion()
			e.Draw(screen)
			return
		}

		// on first <tab>, show suggestions
		if e.loadSuggestion() {
			if len(e.suggest.options) == 1 {
				e.accecptSuggestion()
				e.drawLine(screen, e.cursor.row+1)
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
		e.drawLine(screen, e.cursor.row+1)

		if e.suggest != nil {
			e.Draw(screen) // clear previous suggestions
			if e.loadSuggestion() {
				e.showSuggestion(screen)
			}
		}
	case tcell.KeyCtrlU:
		e.delete(pos{e.cursor.row, 0}, e.cursor)
		e.drawLine(screen, e.cursor.row+1)
	case tcell.KeyCtrlK:
		e.delete(e.cursor, pos{e.cursor.row, len(e.buf[e.cursor.row])})
		e.drawLine(screen, e.cursor.row+1)
	case tcell.KeyESC:
		if e.suggest != nil {
			e.suggest = nil
			e.Draw(screen)
			return
		}
		if e.find.match != nil {
			e.ClearFind()
			e.Draw(screen)
			return
		}
	case tcell.KeyCtrlZ:
		e.undo()
		e.Draw(screen)
	case tcell.KeyCtrlR:
		e.redo()
		e.Draw(screen)
	}
}

type suggestion struct {
	x, y    int
	options []string
	i       int
	up      bool // layout direction
}

func (e *Editor) loadSuggestion() bool {
	prevWord := string(getToken(e.buf[e.cursor.row], e.cursor.col-1))
	if len(prevWord) == 0 {
		e.suggest = nil
		return false
	}

	tokens := tokenTree.get(prevWord)
	if len(tokens) == 0 {
		e.suggest = nil
		return false
	}

	// are ten options enough for most cases ?
	max := 10
	if len(tokens) > max {
		tokens = tokens[:max+1]
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
	word := string(getToken(e.buf[e.cursor.row], e.cursor.col))
	// TODO: this is a replacement, use e.do()
	e.buf[e.cursor.row] = e.buf[e.cursor.row][:e.cursor.col-len(word)]
	e.cursor.col = -len(word)
	e.syncCursor()
	e.writeString(e.suggest.options[e.suggest.i])
	e.suggest = nil
}

func (e *Editor) ClearFind() {
	e.find = find{}
}

type lineBar struct {
	BaseView
	top    int
	bottom int
}

func (b *lineBar) Draw(screen tcell.Screen) {
	style := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorGray)
	for i := 0; i < b.height; i++ {
		for j := 0; j < b.width; j++ {
			screen.SetContent(b.x+j, b.y+i, ' ', nil, style)
		}
	}

	paddingRight := 1
	for i := b.top; i <= b.bottom; i++ {
		s := strconv.Itoa(i)
		for j, c := range s {
			if j > b.width {
				break
			}
			// align right
			screen.SetContent(b.x+b.width-1-(len(s)-j)-paddingRight, b.y+i-b.top, c, nil, style)
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

func (e *Editor) do(a ...Action) {
	if len(a) == 0 {
		return
	}
	action := group(a)
	action.Do()
	e.history = append(e.history, action)
	e.historyUndo = nil
	e.dirty = true
}

func (e *Editor) undo() {
	if len(e.history) == 0 {
		return
	}
	act := e.history[len(e.history)-1]
	act.Undo()
	e.history = e.history[:len(e.history)-1]
	e.historyUndo = append(e.historyUndo, act)
	e.dirty = true
}

func (e *Editor) redo() {
	if len(e.historyUndo) == 0 {
		return
	}
	act := e.historyUndo[len(e.historyUndo)-1]
	act.Do()
	e.historyUndo = e.historyUndo[:len(e.historyUndo)-1]
	e.history = append(e.history, act)
	e.dirty = true
}

// Action represents a buffer change or cursor movement, or both.
type Action interface {
	Do()
	Undo()
}

// cursor position in buffer, starting from 0.
type pos struct {
	row int
	col int
}

type insertion struct {
	e   *Editor
	pos pos
	str string
}

func Insert(e *Editor, p pos, str string) insertion {
	return insertion{e: e, pos: p, str: str}
}

func (i insertion) Do() {
	i.e.buf[i.pos.row] = slices.Insert(i.e.buf[i.pos.row], i.pos.col, []rune(i.str)...)
}

func (i insertion) Undo() {
	i.e.buf[i.pos.row] = slices.Delete(i.e.buf[i.pos.row], i.pos.col, i.pos.col+len(i.str))
}

type deletion struct {
	e     *Editor
	start pos
	stop  pos
	str   string
}

func Delete(e *Editor, start, stop pos) deletion {
	return deletion{
		e:     e,
		start: start,
		stop:  stop,
		str:   string(e.buf[start.row][start.col:stop.col]),
	}
}

func (d deletion) Do() {
	d.e.buf[d.start.row] = slices.Delete(d.e.buf[d.start.row], d.start.col, d.stop.col)
}

func (d deletion) Undo() {
	d.e.buf[d.start.row] = slices.Insert(d.e.buf[d.start.row], d.start.col, []rune(d.str)...)
}

// cursor movement
type movement struct {
	e   *Editor
	old pos
	new pos
}

func Move(e *Editor, to pos) movement {
	return movement{
		e:   e,
		old: e.cursor,
		new: to,
	}
}

func (m movement) Do() {
	m.e.cursor = m.new
	m.e.syncCursor()
}

func (m movement) Undo() {
	m.e.cursor = m.old
	m.e.syncCursor()
}

type group []Action

func (g group) Do() {
	for _, act := range g {
		act.Do()
	}
}

func (g group) Undo() {
	for _, act := range g {
		act.Undo()
	}
}
