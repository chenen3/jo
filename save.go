package main

import (
	"os"

	"github.com/gdamore/tcell/v2"
)

type saveBar struct {
	jo               *Jo
	filename         []rune
	x, y             int
	width            int
	height           int
	cursorX, cursorY int
	quit             bool
}

func newSaveBar(j *Jo, quit bool) *saveBar {
	return &saveBar{jo: j, quit: quit, height: 1}
}

func (s *saveBar) SetPos(x, y, width, height int) {
	s.x = x
	s.y = y
	s.width = width
}

func (s *saveBar) Update(string)             {}
func (s *saveBar) Pos() (int, int, int, int) { return s.x, s.y, s.width, s.height }

func (s *saveBar) Render() {
	style := tcell.StyleDefault.Background(tcell.ColorLightYellow).Foreground(tcell.ColorBlack)
	for y := s.y; y < s.y+s.height; y++ {
		for x := s.x; x < s.x+s.width; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	prompt := "save changes?"
	if s.jo.editor.filename == "" {
		prompt = "save as:"
	}

	s.cursorX, s.cursorY = s.x, s.y
	for _, c := range prompt {
		screen.SetContent(s.cursorX, s.cursorY, c, nil, style)
		s.cursorX++
	}

	if s.jo.editor.filename == "" && len(s.filename) == 0 {
		placeholder := "file name"
		for i, c := range placeholder {
			screen.SetContent(s.cursorX+i, s.cursorY, c, nil, style.Foreground(tcell.ColorGray))
		}
	}

	if s.jo.editor.filename == "" && len(s.filename) != 0 {
		for _, c := range s.filename {
			screen.SetContent(s.cursorX, s.cursorY, c, nil, style)
			s.cursorX++
		}
	}

	keymap := "[enter]save | [esc]cancel | [ctrl+q]discard"
	for i, c := range keymap {
		// align right
		screen.SetContent(s.x+s.width-len(keymap)+i, s.y, c, nil, style)
	}
}

func (s *saveBar) ShowCursor() {
	screen.ShowCursor(s.cursorX, s.cursorY)
}

func (s *saveBar) LostFocus()  {}
func (s *saveBar) Fixed() bool { return true }

func (s *saveBar) HandleEvent(ev tcell.Event) {
	k, ok := ev.(*tcell.EventKey)
	if !ok {
		return
	}

	switch k.Key() {
	case tcell.KeyRune:
		if s.jo.editor.filename != "" {
			return
		}
		s.filename = append(s.filename, k.Rune())
	case tcell.KeyCtrlS, tcell.KeyEnter:
		var filename string
		if s.jo.editor.filename != "" {
			filename = s.jo.editor.filename
		} else if len(s.filename) != 0 {
			filename = string(s.filename)
		} else {
			// logger.Print("empty filename")
			return
		}

		f, err := os.Create(filename)
		if err != nil {
			logger.Print(err)
			return
		}
		defer f.Close()
		_, err = s.jo.editor.WriteTo(f)
		if err != nil {
			logger.Print(err)
			return
		}

		if s.quit {
			close(s.jo.done)
			return
		}
		if s.jo.editor.filename == "" {
			s.jo.editor.filename = filename
			s.jo.titleBar.Set(filename)
			s.jo.titleBar.Render()
		}
		s.jo.statusBar = newStatusBar(s.jo)
		s.jo.focus = s.jo.editor
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if s.jo.editor.filename != "" {
			return
		}
		if len(s.filename) == 0 {
			return
		}
		s.filename = s.filename[:len(s.filename)-1]
	case tcell.KeyESC:
		s.jo.statusBar = newStatusBar(s.jo)
		s.jo.focus = s.jo.editor
	}
}
