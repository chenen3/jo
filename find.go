package main

import "github.com/gdamore/tcell/v2"

type findBar struct {
	jo               *Jo
	keyword          []rune
	x1, y1           int
	x2, y2           int
	cursorX, cursorY int
}

func newFindBar(j *Jo) *findBar {
	return &findBar{jo: j}
}

func (f *findBar) Draw() {
	style := tcell.StyleDefault.Background(tcell.ColorLightYellow).Foreground(tcell.ColorBlack)
	width, height := f.jo.Size()
	f.x1, f.y1 = 0, height-1
	f.x2, f.y2 = width-1, height-1
	for y := f.y1; y <= f.y2; y++ {
		for x := f.x1; x <= f.x2; x++ {
			f.jo.SetContent(x, y, ' ', nil, style)
		}
	}

	s := "find:" + string(f.keyword)
	for i, c := range s {
		f.jo.SetContent(f.x1+i, f.y1, c, nil, style)
	}
	f.cursorX, f.cursorY = f.x1+len(s), f.y1

	keymap := "[enter] next | [ctrl+n] next | [ctrl+p] previous | [esc] cancel"
	for i, c := range keymap {
		if f.x1+i > f.x2 {
			break
		}
		// align right
		f.jo.SetContent(f.x2-len(keymap)+i, f.y1, c, nil, style)
	}
}

func (f *findBar) Position() (x1, y1, x2, y2 int) { return f.x1, f.y1, f.x2, f.y2 }

func (f *findBar) Update(_ string) {}

func (f *findBar) ShowCursor() {
	f.jo.ShowCursor(f.cursorX, f.cursorY)
}

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
	case tcell.KeyEnter, tcell.KeyCtrlN:
		f.jo.editor.FindNext()
	case tcell.KeyCtrlP:
		f.jo.editor.FindPrev()
	case tcell.KeyESC:
		f.jo.editor.ClearFind()
		f.jo.editor.Draw()
		f.jo.statusBar = newStatusBar(f.jo.Screen)
		f.jo.focus = f.jo.editor
	}
}
