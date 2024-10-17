package main

import (
	"log"

	"github.com/gdamore/tcell/v2"
)

type findBar struct {
	BaseView
	keyword []rune
}

// TODO: consider adding the cursor to baseView
func (f *findBar) SetPos(x, y, w, h int) {
	f.BaseView.SetPos(x, y, w, h)
	f.cursorX = x
	f.cursorY = y
}

func (f *findBar) Draw(screen tcell.Screen) {
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

	// if len(f.jo.editor.findMatch) > 0 {
	// 	index := fmt.Sprintf("%d/%d", f.jo.editor.findIndex+1, len(f.jo.editor.findMatch))
	// 	for i, c := range index {
	// 		// align center
	// 		x := (f.width-len(index))/2 + i
	// 		if f.x+f.width <= x || x <= f.cursorX {
	// 			break
	// 		}
	// 		screen.SetContent(x, f.y+f.height, c, nil, style)
	// 	}
	// }

	keymap := "<down> next, <up> previous, <esc> cancel"
	for i, c := range keymap {
		// align right
		x := f.x + f.width - 1 - len(keymap) + i
		if f.x+f.width <= x || x <= f.cursorX {
			break
		}
		screen.SetContent(x, f.y, c, nil, style)
	}

	if f.Focused() {
		log.Print("show cursor ", f.cursorX, f.cursorY)
		screen.ShowCursor(f.cursorX, f.cursorY)
	}
}

func (f *findBar) FixedSize() bool { return true }
