package main

import (
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
	promptBox *promptBox
	filename  string
	done      chan struct{}
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

	if s.promptBox != nil {
		s.promptBox.Draw()
	}
}

func (s *screen) promptSaveBeforeExit() {
	prompt := "Save changes before exit?"
	if s.filename == "" {
		prompt = "Buffer changed, save as: "
	}
	cfg := &promptBoxConfig{
		Screen: s.Screen,
		Style:  tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack),
		Prompt: prompt,
		Input:  s.filename,
		OnYes: func() {
			if s.promptBox.String() == "" {
				logger.Print("empty filename")
				return
			}
			ff, e := os.Create(s.promptBox.String())
			if e != nil {
				logger.Print(e)
				return
			}
			_, e = s.editor.WriteTo(ff)
			ff.Close()
			if e != nil {
				logger.Print(e)
				return
			}
			close(s.done)
		},
		OnNo: func() {
			close(s.done)
		},
		OnCancel: func() {
			s.promptBox = nil
			s.editor.draw()
		},
	}
	s.promptBox = newPromptBox(cfg)
	s.promptBox.Draw()
}

func (s *screen) promptSaveAs() {
	cfg := &promptBoxConfig{
		Screen: s.Screen,
		Style:  tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack),
		Prompt: "Save as: ",
		OnYes: func() {
			if s.promptBox.String() == "" {
				logger.Print("empty filename")
				return
			}
			ff, e := os.Create(s.promptBox.String())
			if e != nil {
				logger.Print(e)
				return
			}
			_, e = s.editor.WriteTo(ff)
			ff.Close()
			if e != nil {
				logger.Print(e)
				return
			}
			s.promptBox = nil
			s.editor.draw()
		},
		OnNo: func() {
			close(s.done)
		},
		OnCancel: func() {
			s.promptBox = nil
			s.editor.draw()
		},
	}
	s.promptBox = newPromptBox(cfg)
	s.promptBox.Draw()
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
		logger.Print(err)
		return
	}
	defer tmp.Close()
	logger = log.New(tmp, "", log.LstdFlags|log.Lshortfile)

	tc, err := tcell.NewScreen()
	if err != nil {
		logger.Print(err)
		return
	}
	if err = tc.Init(); err != nil {
		logger.Print(err)
		return
	}
	tc.SetStyle(tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset))
	tc.EnableMouse()
	tc.SetCursorStyle(tcell.CursorStyleBlinkingBlock)
	tc.EnablePaste()
	tc.Clear()
	defer tc.Fini()

	s := &screen{
		Screen: tc,
		done:   make(chan struct{}),
	}
	screenWidth, screenHeight := s.Size()

	if len(os.Args) > 1 {
		s.filename = os.Args[1]
	}

	title := "New Buffer"
	if s.filename != "" {
		title = s.filename
	}
	s.titleBar = &bar{
		s: s, x1: 0, y1: 0, x2: screenWidth - 1, y2: 0, texts: []string{title},
		style: tcell.StyleDefault.Background(tcell.ColorGray).Foreground(tcell.ColorWhite),
	}
	s.titleBar.draw()

	var src []byte
	if s.filename != "" {
		src, err = os.ReadFile(s.filename)
		if err != nil {
			logger.Print(err)
			return
		}
	}

	editorStyle := tcell.StyleDefault.Background(tcell.ColorWhite).Foreground(tcell.ColorBlack)
	editor, err := newEditor(s, 0, 1, screenWidth-1, screenHeight-2, editorStyle, src)
	if err != nil {
		logger.Println(err)
		return
	}
	s.editor = editor
	s.editor.draw()
	s.ShowCursor(s.editor.cx, s.editor.cy)

	s.statusBar = &statusBar{
		s: s, x1: 0, y1: screenHeight - 1, x2: screenWidth - 1, y2: screenHeight - 1,
		style: tcell.StyleDefault.Background(tcell.ColorGray).Foreground(tcell.ColorWhite),
	}
	s.statusBar.draw()

	for {
		select {
		case <-s.done:
			return
		default:
		}

		s.statusBar.update(s.editor.row(), s.editor.col())
		s.Show()
		ev := s.PollEvent()

		switch ev := ev.(type) {
		case *tcell.EventResize:
			w, h := s.Size()
			s.resize(w, h)
			s.Sync()
		case *tcell.EventMouse:
			// TODO: editor lost focus
			if s.promptBox != nil {
				continue
			}
			x, y := ev.Position()
			switch ev.Buttons() {
			case tcell.Button1, tcell.Button2:
				s.editor.SetCursor(x, y)
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
			if s.promptBox != nil {
				s.promptBox.Handle(ev)
				continue
			}
			switch ev.Key() {
			case tcell.KeyCtrlQ:
				if s.editor.dirty {
					s.promptSaveBeforeExit()
					continue
				}
				return
			case tcell.KeyCtrlS:
				if s.filename == "" {
					s.promptSaveAs()
					continue
				}
				ff, err := os.Create(s.filename)
				if err != nil {
					logger.Println(err)
					continue
				}
				_, err = s.editor.WriteTo(ff)
				ff.Close()
				if err != nil {
					logger.Println(err)
					continue
				}
				// a new line may be added at the end of file
				s.editor.draw()
				s.promptBox = nil
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
				s.editor.DeleteLeft()
			case tcell.KeyCtrlU:
				s.editor.DeleteToLineStart()
			case tcell.KeyCtrlK:
				s.editor.DeleteToLineEnd()
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

	text := "[ctrl+s] save | [ctrl+q] quit"
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
	x1, y1 int
	x2, y2 int
	screen tcell.Screen
	style  tcell.Style

	// editable buffer
	bx1, by1 int
	bx2, by2 int
	buf      [][]rune
	dirty    bool

	lineBar   *bar
	cx, cy    int // cursor position
	startLine int
}

// TODO: Accepting src of bytes is not an efficient way, and can lead to lags when loading large files;
// A Reader or Scanner should be used instead, but this brings up another problem,
// which is how to incrementally read the file and then rewrite it correctly.
func newEditor(s tcell.Screen, x1, y1, x2, y2 int, style tcell.Style, src []byte) (*editor, error) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	e := &editor{
		x1:        x1,
		y1:        y1,
		x2:        x2,
		y2:        y2,
		screen:    s,
		by1:       y1,
		bx2:       x2,
		by2:       y2,
		style:     style,
		cy:        y1,
		startLine: 1,
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

	lineBarWidth := 2
	for i := len(e.buf); i > 0; i = i / 10 {
		lineBarWidth++
	}
	e.bx1 = x1 + lineBarWidth
	e.cx = x1 + lineBarWidth
	e.lineBar = &bar{
		s:            s,
		x1:           x1,
		y1:           y1,
		x2:           x1 + lineBarWidth,
		y2:           y2,
		style:        style.Foreground(tcell.ColorGray),
		align:        alignRight,
		paddingRight: 1,
	}
	return e, nil
}

// maximize number of lines that can be displayed by the editor at one time
func (e *editor) height() int { return e.by2 - e.by1 + 1 }

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

	tokens := parseToken(line)
	i := 0
	color := tokenColor(tokens[i].class)
	for j := range line {
		if j >= tokens[i].off+tokens[i].len && i < len(tokens)-1 {
			i++
			color = tokenColor(tokens[i].class)
		}
		e.screen.SetContent(e.bx1+j, e.by1+row-e.startLine, line[j], nil, e.style.Foreground(color))
	}
}

func (e *editor) draw() {
	lineBarWidth := 2
	for i := len(e.buf); i > 0; i = i / 10 {
		lineBarWidth++
	}
	e.bx1 = e.x1 + lineBarWidth
	e.lineBar.x2 = e.x1 + lineBarWidth
	lineNums := make([]string, 0, e.height())
	for i := 0; i < e.height(); i++ {
		if i+e.startLine-1 >= len(e.buf) {
			break
		}
		lineNums = append(lineNums, strconv.Itoa(i+e.startLine))
	}
	e.lineBar.texts = lineNums
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
	e.screen.ShowCursor(e.cx, e.cy)
}

func (e *editor) SetCursor(x, y int) {
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
	if x >= e.bx1+len(e.buf[y-e.by1+e.startLine-1]) {
		x = e.bx1 + len(e.buf[y-e.by1+e.startLine-1])
	}

	e.cx, e.cy = x, y
	e.screen.ShowCursor(x, y)
}

func (e *editor) CursorUp() {
	if e.row() > 1 && e.row() == e.startLine {
		e.startLine--
		e.draw()
	}
	e.SetCursor(e.cx, e.cy-1)
}

func (e *editor) CursorDown() {
	if e.row() == len(e.buf) {
		// end of file
		return
	}

	if e.row() < e.startLine+e.height()-1 {
		e.SetCursor(e.cx, e.cy+1)
		return
	}

	e.startLine++
	e.draw()
	e.SetCursor(e.cx, e.cy+1)
}

func (e *editor) CursorLeft() {
	if e.col() == 1 {
		// go to previous line
		e.SetCursor(e.bx2, e.cy-1)
		return
	}
	e.SetCursor(e.cx-1, e.cy)
}

func (e *editor) CursorRight() {
	if e.col() == len(e.buf[e.row()-1])+1 {
		// go to next line
		e.SetCursor(e.bx1, e.cy+1)
		return
	}
	e.SetCursor(e.cx+1, e.cy)
}

func (e *editor) CursorLineStart() {
	e.SetCursor(e.bx1, e.cy)
}

func (e *editor) CursorLineEnd() {
	e.SetCursor(e.bx2, e.cy)
}

func (e *editor) Insert(r rune) {
	line := e.buf[e.row()-1]
	rs := make([]rune, len(line[e.col()-1:]))
	copy(rs, line[e.col()-1:])
	line = append(append(line[:e.col()-1], r), rs...)
	// logger.Printf("row:%d col:%d, line: %q", e.row(), e.col(), string(line))

	// for i, c := range line {
	// 	e.screen.SetContent(e.bx1+i, e.cy, c, nil, e.style)
	// }
	e.buf[e.row()-1] = line
	e.drawLine(e.row())
	e.CursorRight()
	e.dirty = true
}

// current line number
func (e *editor) row() int {
	return e.cy - e.by1 + e.startLine
}

// current column number
func (e *editor) col() int {
	return e.cx - e.bx1 + 1
}

func (e *editor) DeleteLeft() {
	e.dirty = true
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
		e.SetCursor(e.bx1+prevLen, e.cy-1)
		return
	}

	line := e.buf[row-1]
	if col-1 == len(line) {
		// line end
		line = line[:col-2]
	} else {
		// TODO: consider the new function slices.Delete ?
		line = append(line[:col-2], line[col-1:]...)
	}
	// logger.Printf("row:%d col:%d, line: %q", row, col, string(line))
	// for x := e.bx1; x <= e.bx2; x++ {
	// 	e.screen.SetContent(x, e.cy, ' ', nil, e.style)
	// }
	// for i, c := range line {
	// 	e.screen.SetContent(e.bx1+i, e.cy, c, nil, e.style)
	// }
	e.buf[row-1] = line
	e.drawLine(row)
	e.CursorLeft()
}

func (e *editor) DeleteToLineStart() {
	e.dirty = true
	row, col := e.row(), e.col()
	e.buf[row-1] = e.buf[row-1][col-1:]
	// for x := e.bx1; x <= e.bx2; x++ {
	// 	e.screen.SetContent(x, e.cy, ' ', nil, e.style)
	// }
	// for i, c := range e.buf[row-1] {
	// 	e.screen.SetContent(e.bx1+i, e.cy, c, nil, e.style)
	// }
	e.drawLine(row)
	e.CursorLineStart()
}

func (e *editor) DeleteToLineEnd() {
	e.dirty = true
	row, col := e.row(), e.col()
	e.buf[row-1] = e.buf[row-1][:col-1]
	// for x := e.bx1; x <= e.bx2; x++ {
	// 	e.screen.SetContent(x, e.cy, ' ', nil, e.style)
	// }
	// for i, c := range e.buf[row-1] {
	// 	e.screen.SetContent(e.bx1+i, e.cy, c, nil, e.style)
	// }
	e.drawLine(row)
}

func (e *editor) Enter() {
	e.dirty = true
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
	e.draw()
	e.SetCursor(e.bx1, e.cy+1)
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

type promptBox struct {
	cfg    *promptBoxConfig
	x1, y1 int
	x2, y2 int

	buf    []rune
	noEdit bool
	cx, cy int
}

type promptBoxConfig struct {
	Screen   tcell.Screen
	Style    tcell.Style
	Prompt   string
	Input    string
	OnYes    func()
	OnNo     func()
	OnCancel func()
}

func newPromptBox(cfg *promptBoxConfig) *promptBox {
	width, height := cfg.Screen.Size()
	p := promptBox{
		cfg: cfg,
		x1:  width / 3,
		y1:  height / 3,
		x2:  width * 2 / 3,
		y2:  (height / 3) + 4,
	}
	if cfg.Input != "" {
		p.buf = []rune(cfg.Input)
		p.noEdit = true
	}
	return &p
}

func (p *promptBox) Draw() {
	width, height := p.cfg.Screen.Size()
	p.x1 = width / 3
	p.y1 = height / 3
	p.x2 = width * 2 / 3
	p.y2 = (height / 3) + 4

	for y := p.y1; y <= p.y2; y++ {
		for x := p.x1; x <= p.x2; x++ {
			p.cfg.Screen.SetContent(x, y, ' ', nil, p.cfg.Style)
		}
	}

	// Draw borders
	for x := p.x1; x <= p.x2; x++ {
		p.cfg.Screen.SetContent(x, p.y1, tcell.RuneHLine, nil, p.cfg.Style)
		p.cfg.Screen.SetContent(x, p.y2, tcell.RuneHLine, nil, p.cfg.Style)
	}
	for y := p.y1 + 1; y < p.y2; y++ {
		p.cfg.Screen.SetContent(p.x1, y, tcell.RuneVLine, nil, p.cfg.Style)
		p.cfg.Screen.SetContent(p.x2, y, tcell.RuneVLine, nil, p.cfg.Style)
	}
	if p.y1 != p.y2 && p.x1 != p.x2 {
		p.cfg.Screen.SetContent(p.x1, p.y1, tcell.RuneULCorner, nil, p.cfg.Style)
		p.cfg.Screen.SetContent(p.x2, p.y1, tcell.RuneURCorner, nil, p.cfg.Style)
		p.cfg.Screen.SetContent(p.x1, p.y2, tcell.RuneLLCorner, nil, p.cfg.Style)
		p.cfg.Screen.SetContent(p.x2, p.y2, tcell.RuneLRCorner, nil, p.cfg.Style)
	}

	s := []rune(p.cfg.Prompt)
	if !p.noEdit {
		s = append(s, p.buf...)
	}
	x := p.x1 + 1
	y := p.y1 + 1
	for _, r := range s {
		if x >= p.x2 {
			x = p.x1 + 1
			y++
		}
		p.cfg.Screen.SetContent(x, y, r, nil, p.cfg.Style)
		x++
	}
	p.cx = x
	p.cy = y
	p.cfg.Screen.ShowCursor(p.cx, p.cy)

	keymap := "[Enter] yes | [Esc] cancel | [ctrl+q] exit"
	offset := (p.x2 - p.x1 - len(keymap)) / 2
	for i, r := range keymap {
		p.cfg.Screen.SetContent(p.x1+1+offset+i, p.y2-1, r, nil, p.cfg.Style)
	}
}

func (p *promptBox) Insert(r rune) {
	if p.noEdit {
		return
	}
	p.buf = append(p.buf, r)
	if p.cx >= p.x2 {
		p.cx = p.x1 + 1
		p.cy++
	}
	p.cfg.Screen.SetContent(p.cx, p.cy, r, nil, p.cfg.Style)
	p.cx++
	p.cfg.Screen.ShowCursor(p.cx, p.cy)
}

func (p *promptBox) DeleteLeft() {
	if p.noEdit {
		return
	}
	if len(p.buf) == 0 {
		return
	}
	p.buf = p.buf[:len(p.buf)-1]
	p.cx--
	p.cfg.Screen.SetContent(p.cx, p.cy, ' ', nil, p.cfg.Style)
	p.cfg.Screen.ShowCursor(p.cx, p.cy)
}

func (p *promptBox) String() string {
	return string(p.buf)
}

func (p *promptBox) Handle(ev tcell.Event) {
	k, ok := ev.(*tcell.EventKey)
	if !ok {
		return
	}

	switch k.Key() {
	case tcell.KeyCtrlQ:
		p.cfg.OnNo()
	case tcell.KeyRune:
		p.Insert(k.Rune())
	case tcell.KeyEnter:
		p.cfg.OnYes()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		p.DeleteLeft()
	case tcell.KeyESC:
		p.cfg.OnCancel()
	}
}
