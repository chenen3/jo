package main

import (
	"os"

	"github.com/gdamore/tcell/v2"
)

type saveBar struct {
	jo               *Jo
	filename         []rune
	x1, y1           int
	x2, y2           int
	cursorX, cursorY int
	quit             bool
}

func newSaveBar(j *Jo, quit bool) *saveBar {
	return &saveBar{jo: j, quit: quit}
}

func (s *saveBar) Update(string)                  {}
func (s *saveBar) Position() (int, int, int, int) { return s.x1, s.y1, s.x2, s.y2 }

func (s *saveBar) Draw() {
	style := tcell.StyleDefault.Background(tcell.ColorLightYellow).Foreground(tcell.ColorBlack)
	width, height := s.jo.Size()
	s.x1, s.y1 = 0, height-1
	s.x2, s.y2 = width-1, height-1
	for y := s.y1; y <= s.y2; y++ {
		for x := s.x1; x <= s.x2; x++ {
			s.jo.SetContent(x, y, ' ', nil, style)
		}
	}

	prompt := "save changes?"
	if s.jo.filename == "" {
		prompt = "save as:"
	}
	if s.quit {
		prompt = "quit and " + prompt
	}
	str := prompt + string(s.filename)
	for i, c := range str {
		s.jo.SetContent(s.x1+i, s.y1, c, nil, style)
	}
	s.cursorX, s.cursorY = s.x1+len(str), s.y1

	keymap := "[enter] save | [esc] cancel | [ctrl+q] discard"
	for i, c := range keymap {
		if s.x1+i > s.x2 {
			break
		}
		// align right
		s.jo.SetContent(s.x2-len(keymap)+i, s.y1, c, nil, style)
	}
}

func (s *saveBar) ShowCursor() {
	s.jo.ShowCursor(s.cursorX, s.cursorY)
}

func (s *saveBar) HandleEvent(ev tcell.Event) {
	k, ok := ev.(*tcell.EventKey)
	if !ok {
		return
	}

	switch k.Key() {
	case tcell.KeyRune:
		if s.jo.filename != "" {
			return
		}
		s.filename = append(s.filename, k.Rune())
	case tcell.KeyEnter:
		var filename string
		if s.jo.filename != "" {
			filename = s.jo.filename
		} else if len(s.filename) != 0 {
			filename = string(s.filename)
		} else {
			logger.Print("empty filename")
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
		s.jo.filename = filename
		s.jo.statusBar = newStatusBar(s.jo)
		s.jo.focus = s.jo.editor
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if s.jo.filename != "" {
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
