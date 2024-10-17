package main

import (
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type App struct {
	body *vstack

	editors   *hstack
	statusBar *statusBar
	findBar   *findBar
	saveBar   *saveBar
	gotoBar   *gotoBar

	focus  View
	editor *editor // current or last focused editor
	done   chan struct{}
	mouseX int
	mouseY int
}

func (a *App) Focus(v View) {
	if a.focus == v {
		return
	}

	if a.focus != nil {
		a.focus.OnBlur()
	}
	a.focus = v
	v.OnFocus()
	if e, ok := v.(*editor); ok {
		a.editor = e
	}
}

func (a *App) getHover() View {
	return getHover(a.body, a.mouseX, a.mouseY)
}

func getHover(view View, x, y int) View {
	if !inView(view, x, y) {
		return nil
	}

	if s, ok := view.(*vstack); ok {
		for _, v := range s.Views {
			hover := getHover(v, x, y)
			if hover != nil {
				return hover
			}
		}
	}
	if s, ok := view.(*hstack); ok {
		for _, v := range s.Views {
			hover := getHover(v, x, y)
			if hover != nil {
				return hover
			}
		}
	}
	return view
}

func (a *App) splitEditor(name string) {
	e := newEditor(name, a.statusBar.Status)
	a.editors.Views = append(a.editors.Views, e)
	a.Focus(e)
}

func (a *App) closeEditor() {
	a.editor.CloseBuffer()
	if len(a.editor.titleBar.names) > 0 {
		return
	}

	if len(a.editors.Views) == 1 {
		close(a.done)
		return
	}

	// delete editor
	var i int
	for i = range a.editors.Views {
		if a.editors.Views[i] == a.editor {
			break
		}
	}
	a.editors.Views = slices.Delete(a.editors.Views, i, i+1)
	j := i - 1
	if j < 0 {
		j = 0
	}
	prevE := a.editors.Views[j].(*editor)
	a.Focus(prevE)
}

func (a *App) Draw() {
	a.body.Draw()
}

var screen tcell.Screen

// A multiplier to be used on the deltaX and deltaY of mouse wheel scroll events
const scrollSensitivity = 0.125

func main() {
	logFile, err := os.OpenFile("/tmp/jo.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(logFile)

	s, err := tcell.NewScreen()
	if err != nil {
		log.Print(err)
		return
	}
	if err = s.Init(); err != nil {
		log.Print(err)
		return
	}
	screen = s
	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset))
	screen.EnableMouse()
	screen.SetCursorStyle(tcell.CursorStyleBlinkingBlock)
	screen.EnablePaste()
	screen.Clear()
	defer screen.Fini()

	var filename string
	if len(os.Args) > 1 {
		filename = os.Args[1]
	}

	app := &App{
		done: make(chan struct{}),
	}
	app.statusBar = newStatusBar(app)
	app.editor = newEditor(filename, app.statusBar.Status)
	app.editors = HStack(app.editor)
	app.body = VStack(app.editors, app.statusBar)
	width, height := screen.Size()
	app.body.SetPos(0, 0, width, height)

	fb := new(findBar)
	fb.SetPos(width-40, 1, 40, 1)
	fb.Handle(tcell.KeyRune, func(k *tcell.EventKey) {
		fb.keyword = append(fb.keyword, k.Rune())
		app.editor.Find(string(fb.keyword))
		fb.Draw()
	})
	fb.Handle(tcell.KeyBackspace, func(*tcell.EventKey) {
		if len(fb.keyword) == 0 {
			return
		}
		fb.keyword = fb.keyword[:len(fb.keyword)-1]
		if len(fb.keyword) == 0 {
			app.editor.ClearFind()
			app.editor.Draw()
		} else {
			app.editor.Find(string(fb.keyword))
		}
		fb.Draw()
	})
	fb.Handle(tcell.KeyBackspace2, func(*tcell.EventKey) {
		if len(fb.keyword) == 0 {
			return
		}
		fb.keyword = fb.keyword[:len(fb.keyword)-1]
		if len(fb.keyword) == 0 {
			app.editor.ClearFind()
			app.editor.Draw()
		} else {
			app.editor.Find(string(fb.keyword))
		}
		fb.Draw()
	})
	fb.Handle(tcell.KeyEnter, func(*tcell.EventKey) {
		app.editor.FindNext()
	})
	fb.Handle(tcell.KeyDown, func(*tcell.EventKey) {
		app.editor.FindNext()
	})
	fb.Handle(tcell.KeyUp, func(*tcell.EventKey) {
		app.editor.FindPrev()
	})
	fb.Handle(tcell.KeyESC, func(*tcell.EventKey) {
		fb.keyword = nil
		app.Focus(app.editor)
		app.editor.Draw() // cover the findbar
	})
	app.findBar = fb

	sb := new(saveBar)
	sb.SetPos((width-40)/2, (height-3)/2, 40, 3) // align center
	sb.Handle(tcell.KeyRune, func(k *tcell.EventKey) {
		sb.name = append(sb.name, k.Rune())
		sb.Draw()
	})
	sb.Handle(tcell.KeyBackspace2, func(k *tcell.EventKey) {
		if len(sb.name) == 0 {
			return
		}
		sb.name = sb.name[:len(sb.name)-1]
		sb.Draw()
	})

	sb.Handle(tcell.KeyEnter, func(k *tcell.EventKey) {
		if len(sb.name) == 0 {
			return
		}
		f, err := os.Create(string(sb.name))
		if err != nil {
			log.Print(err)
			return
		}
		defer f.Close()
		_, err = app.editor.WriteTo(f)
		if err != nil {
			log.Print(err)
			return
		}

		app.editor.Load(string(sb.name))
		app.Focus(app.editor)
		app.editor.Draw()
	})
	sb.Handle(tcell.KeyESC, func(k *tcell.EventKey) {
		sb.name = nil
		app.Focus(app.editor)
		app.editor.Draw() // cover the savebar
	})
	app.saveBar = sb

	gb := newGotoBar()
	gb.Handle(tcell.KeyEsc, func(*tcell.EventKey) {
		gb.keyword = nil
		app.editor.Draw()
		app.Focus(app.editor)
	})
	gb.Handle(tcell.KeyRune, func(k *tcell.EventKey) {
		defer gb.Draw()
		app.editor.Draw() // clear previous options
		gb.keyword = append(gb.keyword, k.Rune())
		if gb.keyword[0] == ':' {
			return
		}
		options := make([]string, 0, len(files))
		for _, f := range files {
			if strings.Contains(strings.ToLower(f), string(gb.keyword)) {
				options = append(options, f)
			}
		}
		gb.options = options
	})
	gb.Handle(tcell.KeyBackspace2, func(*tcell.EventKey) {
		if len(gb.keyword) == 0 {
			return
		}
		defer gb.Draw()
		app.editor.Draw() // clear previous options
		gb.keyword = gb.keyword[:len(gb.keyword)-1]
		if len(gb.keyword) == 0 {
			return
		}
		if gb.keyword[0] == ':' {
			return
		}
		options := make([]string, 0, len(files))
		for _, f := range files {
			if strings.Contains(strings.ToLower(f), string(gb.keyword)) {
				options = append(options, f)
			}
		}
		gb.options = options
	})
	gb.Handle(tcell.KeyEnter, func(*tcell.EventKey) {
		// go to line
		if len(gb.keyword) > 0 && gb.keyword[0] == ':' {
			line, err := strconv.Atoi(string(gb.keyword[1:]))
			if err != nil {
				log.Printf("goto: invalid line number: %s", err)
				return
			}
			if line < 1 || line > len(app.editor.buf)+1 {
				log.Printf("goto: line number out of range")
				return
			}
			app.editor.line = line
			app.editor.column = 1
			if line <= app.editor.PageSize()/2 {
				app.editor.startLine = 1
			} else {
				app.editor.startLine = line - app.editor.PageSize()/2
			}
			app.editor.Draw()
			app.Focus(app.editor)
			gb.index = 0
			gb.keyword = nil
			return
		}
		// go to file
		if len(gb.options) > 0 {
			app.editor.Load(gb.options[gb.index])
			app.editor.Draw()
			app.Focus(app.editor)
			gb.keyword = nil
			gb.index = 0
		}
	})
	gb.Handle(tcell.KeyUp, func(*tcell.EventKey) {
		gb.index--
		if gb.index < 0 {
			gb.index = len(files) - 1
		}
		gb.Draw()
	})
	gb.Handle(tcell.KeyDown, func(*tcell.EventKey) {
		gb.index++
		if gb.index > len(files)-1 {
			gb.index = 0
		}
		gb.Draw()
	})
	gb.Handle(tcell.KeyCtrlBackslash, func(*tcell.EventKey) {
		app.splitEditor(gb.options[gb.index])
		app.Draw()
	})
	app.gotoBar = gb

	app.Draw()
	app.Focus(app.editor)
	screen.Show()

	for {
		select {
		case <-app.done:
			return
		default:
		}

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			width, height := screen.Size()
			app.body.SetPos(0, 0, width, height)
			app.Draw()
			screen.Sync()
			continue
		case *tcell.EventMouse:
			x, y := ev.Position()
			switch ev.Buttons() {
			case tcell.Button1:
				view := app.getHover()
				app.Focus(view)
				view.OnClick(x, y)
			case tcell.WheelUp:
				view := app.getHover()
				if e, ok := view.(*editor); ok {
					delta := int(float32(y) * scrollSensitivity)
					e.ScrollUp(delta)
				}
			case tcell.WheelDown:
				view := app.getHover()
				if e, ok := view.(*editor); ok {
					delta := int(float32(y) * scrollSensitivity)
					e.ScrollDown(delta)
				}
			default:
				app.mouseX = x
				app.mouseY = y
				// do not render on mouse motion
				continue
			}
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyCtrlQ {
				// force exit, discard any changes
				return
			}
			if ev.Key() == tcell.KeyCtrlF {
				app.Focus(app.findBar)
				app.findBar.Draw()
				break
			}
			if ev.Key() == tcell.KeyCtrlS {
				if !app.editor.dirty {
					break
				}
				if app.editor.filename == "" {
					app.Focus(app.saveBar)
					app.saveBar.Draw()
				} else {
					f, err := os.Create(app.editor.filename)
					if err != nil {
						log.Print(err)
						break
					}
					_, err = app.editor.WriteTo(f)
					f.Close()
					if err != nil {
						log.Print(err)
						break
					}
				}
				break
			}
			if ev.Key() == tcell.KeyCtrlP {
				app.gotoBar.Draw()
				app.Focus(app.gotoBar)
				break
			}
			if ev.Key() == tcell.KeyCtrlG {
				app.gotoBar.keyword = []rune{':'}
				app.gotoBar.Draw()
				app.Focus(app.gotoBar)
				break
			}
			if ev.Key() == tcell.KeyCtrlW {
				if app.editor.dirty {
					app.Focus(app.saveBar)
					app.saveBar.Draw()
				} else {
					app.closeEditor()
					app.Draw()
				}
				break
			}
			app.focus.HandleKey(ev)
		}
		screen.Show()
	}
}
