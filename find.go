package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

type findBar struct {
	jo      *Jo
	keyword []rune
	x, y    int
	width   int
	height  int
	cursorX int
	cursorY int
}

func newFindBar(j *Jo) *findBar {
	return &findBar{jo: j, height: 1}
}

func (f *findBar) SetPos(x, y, width, height int) {
	f.x = x
	f.y = y
	f.width = width
}

func (f *findBar) Draw() {
	style := tcell.StyleDefault.Background(tcell.ColorLightYellow).Foreground(tcell.ColorBlack)
	for y := f.y; y < f.y+f.height; y++ {
		for x := f.x; x < f.x+f.width; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}

	s := "find:" + string(f.keyword)
	for i, c := range s {
		screen.SetContent(f.x+i, f.y, c, nil, style)
	}
	f.cursorX, f.cursorY = f.x+len(s), f.y

	if len(f.jo.editor.findMatch) > 0 {
		index := fmt.Sprintf("%d/%d", f.jo.editor.findIndex+1, len(f.jo.editor.findMatch))
		for i, c := range index {
			// align center
			x := (f.width-len(index))/2 + i
			if f.x+f.width <= x || x <= f.cursorX {
				break
			}
			screen.SetContent(x, f.y+f.height, c, nil, style)
		}
	}

	keymap := "<down> next, <up> previous, <esc> cancel"
	for i, c := range keymap {
		// align right
		x := f.x + f.width - 1 - len(keymap) + i
		if f.x+f.width <= x || x <= f.cursorX {
			break
		}
		screen.SetContent(x, f.y, c, nil, style)
	}
}

func (f *findBar) Pos() (x1, y1, x2, y2 int) {
	return f.x, f.y, f.width, f.height
}

func (f *findBar) ShowCursor() {
	screen.ShowCursor(f.cursorX, f.cursorY)
}

func (f *findBar) LostFocus()  {}
func (f *findBar) Fixed() bool { return true }

func (f *findBar) HandleEvent(ev tcell.Event) {
	k, ok := ev.(*tcell.EventKey)
	if !ok {
		return
	}
	switch k.Key() {
	case tcell.KeyRune:
		f.keyword = append(f.keyword, k.Rune())
		f.jo.editor.Find(string(f.keyword))
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(f.keyword) == 0 {
			return
		}
		f.keyword = f.keyword[:len(f.keyword)-1]
		f.jo.editor.Find(string(f.keyword))
	case tcell.KeyEnter, tcell.KeyDown:
		f.jo.editor.FindNext()
	case tcell.KeyUp:
		f.jo.editor.FindPrev()
	}
}
