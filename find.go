package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

type findBar struct {
	baseView
	jo      *Jo
	keyword []rune
	cursorX int
	cursorY int
}

func newFindBar(j *Jo) *findBar {
	return &findBar{jo: j, baseView: baseView{height: 1}}
}

func (f *findBar) OnClick(x, y int) {
	f.Focus()
}

func (f *findBar) Focus() {
	screen.ShowCursor(f.cursorX, f.cursorY)
	if f.jo.focus == f {
		return
	}
	if f.jo.focus != nil {
		f.jo.focus.Defocus()
	}
	f.jo.focus = f
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

func (f *findBar) FixedSize() bool { return true }

func (f *findBar) HandleEvent(ev tcell.Event) {
	k, ok := ev.(*tcell.EventKey)
	if !ok {
		return
	}
	switch k.Key() {
	case tcell.KeyRune:
		f.keyword = append(f.keyword, k.Rune())
		f.jo.editor.Find(string(f.keyword))
		f.Draw()
		screen.ShowCursor(f.cursorX, f.cursorY)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(f.keyword) == 0 {
			return
		}
		f.keyword = f.keyword[:len(f.keyword)-1]
		if len(f.keyword) == 0 {
			f.jo.editor.ClearFind()
			f.jo.editor.Draw()
		} else {
			f.jo.editor.Find(string(f.keyword))
		}
		f.Draw()
		screen.ShowCursor(f.cursorX, f.cursorY)
	case tcell.KeyEnter, tcell.KeyDown:
		f.jo.editor.FindNext()
	case tcell.KeyUp:
		f.jo.editor.FindPrev()
	case tcell.KeyESC:
		f.jo.status.Set(newStatusBar(f.jo))
		f.jo.status.Draw()
		f.jo.editor.Focus()
	}
}
