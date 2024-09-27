package main

import (
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
)

// type Jo struct {
// 	tcell.Screen
// 	titleBar  *titleBar
// 	editor    *editor
// 	statusBar View
// 	done      chan struct{}
// 	focus     View // the focused view will handle event
// }

type Jo struct {
	View  View
	focus View // handle event
	done  chan struct{}
}

var logger *log.Logger
var screen tcell.Screen

// A multiplier to be used on the deltaX and deltaY of mouse wheel scroll events
const wheelScrollSensitivity = 0.125

func main() {
	tmp, err := os.OpenFile("/tmp/jo.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer tmp.Close()
	logger = log.New(tmp, "", log.LstdFlags|log.Lshortfile)

	s, err := tcell.NewScreen()
	if err != nil {
		logger.Print(err)
		return
	}
	if err = s.Init(); err != nil {
		logger.Print(err)
		return
	}
	screen = s
	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset))
	screen.EnableMouse()
	screen.SetCursorStyle(tcell.CursorStyleBlinkingBlock)
	screen.EnablePaste()
	screen.Clear()
	defer screen.Fini()

	j := &Jo{
		done: make(chan struct{}),
		View: &HStack{Views: []View{new(titleBar)}},
	}
	width, height := screen.Size()
	j.View.SetPos(0, 0, width, height)
	j.View.Render()
	err = j.Serve()
	if err != nil {
		logger.Print(err)
		return
	}

	// if len(os.Args) > 1 {
	// 	filename := os.Args[1]
	// 	j.titleBar = newTitleBar(j, filename)
	// 	j.editor = newEditor(j, filename)
	// } else {
	// 	j.titleBar = newTitleBar(j, "")
	// 	j.editor = newEditor(j, "")
	// }

	// j.titleBar.Render()
	// j.editor.Render()
	// j.statusBar = newStatusBar(j)
	// j.statusBar.Render()

	// j.focus = j.editor
	// for {
	// 	select {
	// 	case <-j.done:
	// 		return
	// 	default:
	// 	}

	// 	// j.statusBar.Render()
	// 	j.focus.ShowCursor()
	// 	screen.Show()

	// 	ev := screen.PollEvent()
	// 	switch ev := ev.(type) {
	// 	case *tcell.EventResize:
	// 		j.titleBar.Render()
	// 		j.statusBar.Render()
	// 		j.editor.Render()
	// 		j.Sync()
	// 		continue
	// 	case *tcell.EventKey:
	// 		if ev.Key() == tcell.KeyCtrlQ {
	// 			_, ok := j.statusBar.(*saveBar)
	// 			if !ok && j.editor.dirty {
	// 				j.statusBar = newSaveBar(j, true)
	// 				j.focus = j.statusBar
	// 				continue
	// 			}
	// 			return
	// 		}
	// 		if ev.Key() == tcell.KeyCtrlF {
	// 			if _, ok := j.statusBar.(*findBar); !ok {
	// 				j.statusBar = newFindBar(j)
	// 			}
	// 			j.focus = j.statusBar
	// 			break
	// 		}
	// 		if ev.Key() == tcell.KeyCtrlS {
	// 			if _, ok := j.statusBar.(*saveBar); !ok {
	// 				j.statusBar = newSaveBar(j, false)
	// 			}
	// 			j.focus = j.statusBar
	// 			break
	// 		}
	// 		if ev.Key() == tcell.KeyCtrlP {
	// 			if _, ok := j.statusBar.(*gotoBar); !ok {
	// 				j.statusBar = newGotoBar(j)
	// 			}
	// 			j.focus = j.statusBar
	// 			break
	// 		}
	// 		if ev.Key() == tcell.KeyCtrlG {
	// 			j.statusBar = &gotoBar{jo: j, keyword: []rune{':'}} // TODO
	// 			j.focus = j.statusBar
	// 			break
	// 		}
	// 	case *tcell.EventMouse:
	// 		if ev.Buttons() == tcell.Button1 {
	// 			x, y := ev.Position()
	// 			if inView(j.editor, x, y) {
	// 				if _, ok := j.focus.(*editor); ok {
	// 					break
	// 				}
	// 				j.focus.LostFocus()
	// 				j.focus = j.editor
	// 			} else if inView(j.statusBar, x, y) {
	// 				j.focus.LostFocus()
	// 				j.focus = j.statusBar
	// 			}
	// 		}
	// 	}
	// 	j.focus.HandleEvent(ev)
	// }
}

func (j *Jo) Serve() error {
	for {
		select {
		case <-j.done:
			return nil
		default:
		}
		screen.Show()
		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyCtrlQ {
				close(j.done)
			}
		}
	}
}

// func inView(v View, x, y int) bool {
// 	x1, y1, x2, y2 := v.Pos()
// 	return x1 <= x && x <= x2 && y1 <= y && y <= y2
// }
