package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"github.com/gdamore/tcell/v2"
)

type screen struct {
	tcell.Screen
	titleBar  *bar
	editor    *editor
	statusBar *statusBar
}

func (s *screen) resize(width, height int) {
	s.titleBar.x2 = width - 1
	s.titleBar.draw()

	s.statusBar.y1 = height - 1
	s.statusBar.x2 = width - 1
	s.statusBar.y2 = height - 1
	s.statusBar.draw()

	s.editor.bx2 = width - 1
	s.editor.by2 = height - 2
	s.editor.draw()
}

// bar draws texts in lines
type bar struct {
	s            tcell.Screen
	x1, y1       int
	x2, y2       int
	style        tcell.Style
	texts        []string
	align        alignKind
	paddingRight int
}

type alignKind int

const (
	alignLeft = iota
	alignRight
)

func (b *bar) draw() {
	if b.y2 < b.y1 {
		b.y1, b.y2 = b.y2, b.y1
	}
	if b.x2 < b.x1 {
		b.x1, b.x2 = b.x2, b.x1
	}
	for y := b.y1; y <= b.y2; y++ {
		for x := b.x1; x <= b.x2; x++ {
			b.s.SetContent(x, y, ' ', nil, b.style)
		}
	}

	x, y := b.x1, b.y1
	for i, s := range b.texts {
		if y+i > b.y2 {
			break
		}
		for j, c := range s {
			if x+j > b.x2 {
				break
			}
			if b.align == alignLeft {
				b.s.SetContent(x+j, y+i, c, nil, b.style)
			} else {
				b.s.SetContent(b.x2-(len(s)-j)-b.paddingRight, y+i, c, nil, b.style) // align right
			}
		}
	}
}

var logger *log.Logger

// A multiplier to be used on the deltaX and deltaY of mouse wheel scroll events
const wheelSensitivity = 0.125

func main() {
	tmp, err := os.OpenFile("/tmp/jo.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Print(err)
		return
	}
	defer tmp.Close()
	logger = log.New(tmp, "", log.LstdFlags|log.Lshortfile)

	tc, err := tcell.NewScreen()
	if err != nil {
		log.Print(err)
		return
	}
	if err = tc.Init(); err != nil {
		log.Print(err)
		return
	}
	tc.SetStyle(tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset))
	tc.EnableMouse()
	tc.SetCursorStyle(tcell.CursorStyleDefault)
	tc.EnablePaste()
	tc.Clear()
	defer tc.Fini()

	s := &screen{Screen: tc}
	width, height := s.Size()

	var filename string
	if len(os.Args) > 1 {
		filename = os.Args[1]
	}

	title := "New Buffer"
	if filename != "" {
		title = filename
	}
	s.titleBar = &bar{
		s: s, x1: 0, y1: 0, x2: width - 1, y2: 0, texts: []string{title},
		style: tcell.StyleDefault.Background(tcell.ColorGray).Foreground(tcell.ColorWhite),
	}
	s.titleBar.draw()

	var f *os.File
	if filename != "" {
		f, err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			log.Print(err)
			return
		}
		defer f.Close()
	}
	editor, err := newEditor(s, 0, 1, width-1, height-2, tcell.StyleDefault.Background(tcell.ColorWhite).Foreground(tcell.ColorBlack), f)
	if err != nil {
		log.Println(err)
		return
	}
	s.editor = editor
	s.editor.draw()
	s.ShowCursor(s.editor.cx, s.editor.cy)

	s.statusBar = &statusBar{
		s: s, x1: 0, y1: height - 1, x2: width - 1, y2: height - 1,
		style: tcell.StyleDefault.Background(tcell.ColorGray).Foreground(tcell.ColorWhite),
	}
	s.statusBar.draw()

	for {
		s.statusBar.update(s.editor.row(), s.editor.col())
		s.Show()
		ev := s.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			w, h := s.Size()
			s.resize(w, h)
			s.Sync()
		case *tcell.EventMouse:
			x, y := ev.Position()
			switch ev.Buttons() {
			case tcell.Button1, tcell.Button2:
				s.editor.PutCursor(x, y)
			case tcell.WheelUp:
				deltaY := float32(y)
				delta := int(deltaY * wheelSensitivity)
				for i := 0; i < delta; i++ {
					s.editor.CursorUp()
				}
			case tcell.WheelDown:
				deltaY := float32(y)
				delta := int(deltaY * wheelSensitivity)
				for i := 0; i < delta; i++ {
					s.editor.CursorDown()
				}
			}
		case *tcell.EventKey:
			logger.Printf("key %s", ev.Name())
			switch ev.Key() {
			case tcell.KeyCtrlQ:
				// FIXME
				// if s.editor.Dirty() {
				// s.AskSave()
				// }
				return
			case tcell.KeyCtrlS:
				err = s.editor.Save()
				if err != nil {
					logger.Print(err)
					continue
				}
				// a new line may be added at the end of file
				s.editor.draw()
			case tcell.KeyCtrlA:
				s.editor.CursorLineStart()
			case tcell.KeyCtrlE:
				s.editor.CursorLineEnd()
			case tcell.KeyUp:
				s.editor.CursorUp()
			case tcell.KeyDown:
				s.editor.CursorDown()
			case tcell.KeyLeft:
				s.editor.CursorLeft()
			case tcell.KeyRight:
				s.editor.CursorRight()
			case tcell.KeyRune:
				s.editor.Insert(ev.Rune())
			case tcell.KeyEnter:
				s.editor.Enter()
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				s.editor.DeleteRune()
			case tcell.KeyCtrlU:
				s.editor.DeleteToLineStart()
			case tcell.KeyCtrlK:
				s.editor.DeleteToLineEnd()
				//case tcell.KeyCtrlF:
				//s.editor.Search()
				//case tcell.KeyCtrlP:
			}
		}
	}
}

type statusBar struct {
	s        tcell.Screen
	x1, y1   int
	x2, y2   int
	style    tcell.Style
	row, col int
}

func (sb *statusBar) draw() {
	for y := sb.y1; y <= sb.y2; y++ {
		for x := sb.x1; x <= sb.x2; x++ {
			sb.s.SetContent(x, y, ' ', nil, sb.style)
		}
	}

	s := fmt.Sprintf("line %d, column %d", sb.row, sb.col)
	for i, c := range s {
		sb.s.SetContent(sb.x1+i, sb.y1, c, nil, sb.style)
	}

	text := "ctrl-s save | ctrl-q quit"
	for i, c := range text {
		if sb.x1+i > sb.x2 {
			break
		}
		// align right
		sb.s.SetContent(sb.x2-len(text)+i, sb.y1, c, nil, sb.style)
	}
}

func (sb *statusBar) update(row, col int) {
	sb.row = row
	sb.col = col
	sb.draw()
}

type editor struct {
	x1, y1  int
	x2, y2  int
	screen  tcell.Screen
	style   tcell.Style
	scanner *bufio.Scanner
	rw      io.ReadWriter

	// editable buffer
	bx1, by1 int
	bx2, by2 int
	buf      [][]rune
	dirty    bool

	lineBar  *bar // FIXME: discard bar, use new type
	cx, cy   int  // cursor
	rowStart int  // the number of the first line in buffer
	eof      bool
}

// the maximize number of lines that buffer can have
func (e *editor) maxLines() int { return e.by2 - e.by1 + 1 }

func newEditor(s tcell.Screen, x1, y1, x2, y2 int, style tcell.Style, rw io.ReadWriter) (*editor, error) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	maxLines := y2 - y1 + 1
	buf := make([][]rune, 0, maxLines)
	scanner := bufio.NewScanner(rw)
	for len(buf) < maxLines {
		if scanner.Scan() {
			buf = append(buf, []rune(scanner.Text()))
		} else if scanner.Err() != nil {
			return nil, scanner.Err()
		} else {
			// file ends with a new line
			buf = append(buf, []rune{})
			break
		}
	}

	lineBarWidth := 2
	for i := len(buf); i > 0; i = i / 10 {
		lineBarWidth++
	}

	e := &editor{
		x1:       x1,
		y1:       y1,
		x2:       x2,
		y2:       y2,
		screen:   s,
		rw:       rw,
		scanner:  scanner,
		bx1:      x1 + lineBarWidth,
		by1:      y1,
		bx2:      x2,
		by2:      y2,
		style:    style,
		buf:      buf,
		cx:       x1 + lineBarWidth,
		cy:       y1,
		rowStart: 1,
		lineBar: &bar{
			s:     s,
			x1:    x1,
			y1:    y1,
			x2:    x1 + lineBarWidth,
			y2:    y2,
			style: style.Foreground(tcell.ColorGray),
			// texts:        nil,
			align:        alignRight,
			paddingRight: 1,
		},
	}
	return e, nil
}

func (e *editor) draw() {
	lineBarWidth := 2
	for i := len(e.buf); i > 0; i = i / 10 {
		lineBarWidth++
	}
	e.bx1 = e.x1 + lineBarWidth
	e.cx = e.x1 + lineBarWidth
	e.lineBar.x2 = e.x1 + lineBarWidth
	lineNums := make([]string, 0, e.maxLines())
	for i := 0; i < e.maxLines(); i++ {
		if i+e.rowStart-1 >= len(e.buf) {
			break
		}
		lineNums = append(lineNums, strconv.Itoa(i+e.rowStart))
	}
	e.lineBar.texts = lineNums
	e.lineBar.draw()

	for y := e.by1; y <= e.by2; y++ {
		for x := e.bx1; x <= e.bx2; x++ {
			e.screen.SetContent(x, y, ' ', nil, e.style)
		}
	}

	for i := 0; i < e.maxLines(); i++ {
		if e.rowStart-1+i >= len(e.buf) {
			break
		}
		s := e.buf[e.rowStart-1+i]
		for j, c := range s {
			if e.bx1+j > e.bx2 {
				break
			}
			e.screen.SetContent(e.bx1+j, e.by1+i, c, nil, e.style)
		}
	}
}

// update cursor and return the new position
func (e *editor) PutCursor(x, y int) {
	if x < e.bx1 || x > e.bx2 {
		return
	}
	if y < e.by1 || y > e.by2 {
		return
	}
	if x == e.cx && y == e.cy {
		return
	}
	// in editable area
	if y >= e.by1+len(e.buf)-1 {
		y = e.by1 + len(e.buf) - 1
	}
	if x >= e.bx1+len(e.buf[y-e.by1+e.rowStart-1]) {
		x = e.bx1 + len(e.buf[y-e.by1+e.rowStart-1])
	}

	e.cx, e.cy = x, y
	e.screen.ShowCursor(x, y)
}

func (e *editor) CursorUp() {
	if e.row() > 1 && e.row() == e.rowStart {
		e.rowStart--
		e.draw()
	}
	e.PutCursor(e.cx, e.cy-1)
}

func (e *editor) CursorDown() {
	if e.row() < e.rowStart+e.maxLines()-1 {
		e.PutCursor(e.cx, e.cy+1)
		return
	}

	// may not have reach the end of the buffer
	if e.row() < len(e.buf) {
		e.rowStart++
		e.draw()
		e.PutCursor(e.cx, e.cy+1)
		return
	}

	// may not have reach the end of the file
	if e.scanner.Scan() {
		e.buf = append(e.buf, []rune(e.scanner.Text()))
		e.rowStart++
		e.draw()
		e.PutCursor(e.cx, e.cy+1)
		return
	}
	if e.scanner.Err() != nil {
		logger.Print(e.scanner.Err())
		return
	}
}

func (e *editor) CursorLeft() {
	if e.col() == 1 {
		e.PutCursor(e.bx2, e.cy-1)
		return
	}
	e.PutCursor(e.cx-1, e.cy)
}

func (e *editor) CursorRight() {
	if e.col() == len(e.buf[e.row()-1])+1 {
		e.PutCursor(e.bx1, e.cy+1)
		return
	}
	e.PutCursor(e.cx+1, e.cy)
}

func (e *editor) CursorLineStart() {
	e.PutCursor(e.bx1, e.cy)
}

func (e *editor) CursorLineEnd() {
	e.PutCursor(e.bx2, e.cy)
}

func (e *editor) Dirty() bool {
	return e.dirty
}

func (e *editor) AskSave() {

}

func (e *editor) Insert(r rune) {
	line := e.buf[e.row()-1]
	rs := make([]rune, len(line[e.col()-1:]))
	copy(rs, line[e.col()-1:])
	line = append(append(line[:e.col()-1], r), rs...)
	// logger.Printf("row:%d col:%d, line: %q", e.row(), e.col(), string(line))

	for i, c := range line {
		e.screen.SetContent(e.bx1+i, e.cy, c, nil, e.style)
	}
	e.buf[e.row()-1] = line
	e.CursorRight()
	e.dirty = true
}

// the line number of cursor in editor
func (e *editor) row() int {
	return e.cy - e.by1 + e.rowStart
}

// the column number of cursor in editor
func (e *editor) col() int {
	return e.cx - e.bx1 + 1
}

func (e *editor) DeleteRune() {
	row, col := e.row(), e.col()
	// cursor at the head of line, merge the line to previous one
	if col == 1 {
		if row == 1 {
			return
		}
		prevLen := len(e.buf[row-2])
		e.buf[row-2] = append(e.buf[row-2], e.buf[row-1]...)
		e.buf = append(e.buf[:row-1], e.buf[row:]...)
		e.draw()
		// can not update cursor before redrawed,
		// because the width of lineBar may change, so does bx1
		e.PutCursor(e.bx1+prevLen, e.cy-1)
		return
	}

	line := e.buf[row-1]
	if col-1 == len(line) {
		// cursor at the end of line
		line = line[:col-2]
	} else {
		line = append(line[:col-2], line[col-1:]...)
	}
	// logger.Printf("row:%d col:%d, line: %q", row, col, string(line))
	for x := e.bx1; x <= e.bx2; x++ {
		e.screen.SetContent(x, e.cy, ' ', nil, e.style)
	}
	for i, c := range line {
		e.screen.SetContent(e.bx1+i, e.cy, c, nil, e.style)
	}
	e.buf[row-1] = line
	e.CursorLeft()
	e.dirty = true
}

func (e *editor) DeleteToLineStart() {
	row, col := e.row(), e.col()
	e.buf[row-1] = e.buf[row-1][col-1:]
	for x := e.bx1; x <= e.bx2; x++ {
		e.screen.SetContent(x, e.cy, ' ', nil, e.style)
	}
	for i, c := range e.buf[row-1] {
		e.screen.SetContent(e.bx1+i, e.cy, c, nil, e.style)
	}
	e.CursorLineStart()
	e.dirty = true
}

func (e *editor) DeleteToLineEnd() {
	row, col := e.row(), e.col()
	e.buf[row-1] = e.buf[row-1][:col-1]
	for x := e.bx1; x <= e.bx2; x++ {
		e.screen.SetContent(x, e.cy, ' ', nil, e.style)
	}
	for i, c := range e.buf[row-1] {
		e.screen.SetContent(e.bx1+i, e.cy, c, nil, e.style)
	}
	e.dirty = true
}

func (e *editor) Enter() {
	row, col := e.row(), e.col()
	// cut current line
	line := e.buf[row-1]
	e.buf[row-1] = line[:col-1]
	newline := make([]rune, len(line[col-1:]))
	copy(newline, line[col-1:])
	// TODO: not efficient
	buf := make([][]rune, len(e.buf[row:]))
	for i, rs := range e.buf[row:] {
		buf[i] = make([]rune, len(rs))
		copy(buf[i], rs)
	}
	e.buf = append(append(e.buf[:row], newline), buf...)
	e.PutCursor(e.bx1, e.cy+1)
	e.draw()
}

func (e *editor) Bytes() []byte {
	buf := make([][]byte, len(e.buf))
	for i, bs := range e.buf {
		buf[i] = []byte(string(bs))
	}
	return bytes.Join(buf, []byte{'\n'})
}

func (e *editor) Save() error {
	if len(e.buf[len(e.buf)-1]) != 0 {
		// file ends with a new line
		e.buf = append(e.buf, []rune{})
	}
	_, err := e.rw.Write(e.Bytes())
	return err
}
