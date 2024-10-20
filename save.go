package main

import (
	"github.com/gdamore/tcell/v2"
)

type saveBar struct {
	BaseView
	name   []rune
	prompt bool
}

func (s *saveBar) Draw(screen tcell.Screen) {
	style := tcell.StyleDefault.Background(tcell.ColorLightYellow).Foreground(tcell.ColorBlack)
	for y := s.y; y < s.y+s.height; y++ {
		for x := s.x; x < s.x+s.width; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	prompt := "save as: "
	s.cursorX, s.cursorY = s.x, s.y
	for _, c := range prompt {
		screen.SetContent(s.cursorX, s.cursorY, c, nil, style)
		s.cursorX++
	}

	if len(s.name) == 0 {
		placeholder := "file name"
		for i, c := range placeholder {
			screen.SetContent(s.cursorX+i, s.cursorY, c, nil, style.Foreground(tcell.ColorGray))
		}
	} else {
		for _, c := range s.name {
			screen.SetContent(s.cursorX, s.cursorY, c, nil, style)
			s.cursorX++
		}
	}
	if s.Focused() {
		screen.ShowCursor(s.cursorX, s.cursorY)
	}

	keymap := "<enter>save  <esc>cancel <ctrl+w>close"
	for i, c := range keymap {
		// align center
		screen.SetContent(s.x+(s.width-len(keymap))/2+i, s.y+s.height-1, c, nil, style)
	}
}

func (s *saveBar) FixedSize() bool { return true }
