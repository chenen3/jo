package main

import (
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// recent focused editor
var recentE *EditorGroup

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

	var filename string
	if len(os.Args) > 1 {
		filename = os.Args[1]
	}

	app, err := NewApp()
	if err != nil {
		log.Print(err)
		return
	}
	defer func() {
		err := recover()
		if err != nil {
			app.Close()
			log.Panic(err)
		}
	}()

	statusBar := newStatusBar()
	statusBar.Status = BindStr("", func() {
		statusBar.Draw(app.Screen())
	})

	e := NewEditorGroup(app.Screen(), statusBar.Status)
	if filename != "" {
		e.Open(filename)
	}
	recentE = e
	editors := HStack(e)
	app.SetBody(VStack(editors, statusBar))

	width, height := app.Screen().Size()
	fb := new(findBar)
	fb.SetPos(width-40, 1, 40, 1)
	fb.Handle(tcell.KeyRune, func(k *tcell.EventKey, screen tcell.Screen) {
		fb.keyword = append(fb.keyword, k.Rune())
		recentE.editor.Find(string(fb.keyword))
		recentE.Draw(screen)
		fb.Draw(screen)
	})
	fb.Handle(tcell.KeyBackspace, func(k *tcell.EventKey, screen tcell.Screen) {
		if len(fb.keyword) == 0 {
			return
		}
		fb.keyword = fb.keyword[:len(fb.keyword)-1]
		if len(fb.keyword) == 0 {
			recentE.editor.ClearFind()
			recentE.Draw(screen)
		} else {
			recentE.editor.Find(string(fb.keyword))
			recentE.Draw(screen)
		}
		fb.Draw(screen)
	})
	fb.Handle(tcell.KeyBackspace2, func(k *tcell.EventKey, screen tcell.Screen) {
		if len(fb.keyword) == 0 {
			return
		}
		fb.keyword = fb.keyword[:len(fb.keyword)-1]
		if len(fb.keyword) == 0 {
			recentE.editor.ClearFind()
			recentE.Draw(screen)
		} else {
			recentE.editor.Find(string(fb.keyword))
			recentE.Draw(screen)
		}
		fb.Draw(screen)
	})
	fb.Handle(tcell.KeyEnter, func(k *tcell.EventKey, screen tcell.Screen) {
		recentE.editor.FindNext()
		recentE.Draw(screen)
		fb.Draw(screen)
	})
	fb.Handle(tcell.KeyDown, func(k *tcell.EventKey, screen tcell.Screen) {
		recentE.editor.FindNext()
		recentE.Draw(screen)
		fb.Draw(screen)
	})
	fb.Handle(tcell.KeyUp, func(k *tcell.EventKey, screen tcell.Screen) {
		recentE.editor.FindPrev()
		recentE.Draw(screen)
		fb.Draw(screen)
	})
	fb.Handle(tcell.KeyESC, func(k *tcell.EventKey, screen tcell.Screen) {
		fb.keyword = nil
		app.Focus(recentE)
		recentE.editor.ClearFind()
		recentE.Draw(screen) // cover the findbar
	})

	sb := new(saveBar)
	sb.SetPos((width-40)/2, (height-3)/2, 40, 3) // align center
	sb.Handle(tcell.KeyRune, func(k *tcell.EventKey, screen tcell.Screen) {
		sb.name = append(sb.name, k.Rune())
		sb.Draw(screen)
	})
	sb.Handle(tcell.KeyBackspace2, func(k *tcell.EventKey, screen tcell.Screen) {
		if len(sb.name) == 0 {
			return
		}
		sb.name = sb.name[:len(sb.name)-1]
		sb.Draw(screen)
	})

	sb.Handle(tcell.KeyEnter, func(k *tcell.EventKey, screen tcell.Screen) {
		if len(sb.name) == 0 {
			return
		}
		f, err := os.Create(string(sb.name))
		if err != nil {
			log.Print(err)
			return
		}
		defer f.Close()
		_, err = recentE.editor.WriteTo(f)
		if err != nil {
			log.Print(err)
			return
		}

		recentE.Open(string(sb.name))
		app.Focus(recentE)
		recentE.Draw(screen)
		sb.name = nil
		sb.prompt = false
	})
	sb.Handle(tcell.KeyESC, func(k *tcell.EventKey, screen tcell.Screen) {
		sb.name = nil
		sb.prompt = false
		app.Focus(recentE)
		app.Redraw() // cover the savebar
	})

	gb := new(gotoBar)
	gb.SetPos((width-optionWidth)/2, 3, optionWidth, 1)
	gb.Handle(tcell.KeyEsc, func(k *tcell.EventKey, screen tcell.Screen) {
		app.Redraw()
		app.Focus(recentE)
	})
	gb.Handle(tcell.KeyRune, func(k *tcell.EventKey, screen tcell.Screen) {
		defer gb.Draw(screen)
		app.Redraw() // clear previous options
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
	gb.Handle(tcell.KeyBackspace2, func(k *tcell.EventKey, screen tcell.Screen) {
		if len(gb.keyword) == 0 {
			return
		}
		defer gb.Draw(screen)
		recentE.Draw(screen) // clear previous options
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
	gb.Handle(tcell.KeyEnter, func(k *tcell.EventKey, screen tcell.Screen) {
		// go to line
		if len(gb.keyword) > 0 && gb.keyword[0] == ':' {
			line, err := strconv.Atoi(string(gb.keyword[1:]))
			if err != nil {
				log.Printf("goto: invalid line number: %s", err)
				return
			}
			if line < 1 || line > len(recentE.editor.buf)+1 {
				log.Printf("goto: line number out of range")
				return
			}
			recentE.editor.cursor.row = line - 1
			recentE.editor.cursor.col = 0
			if line <= recentE.editor.PageSize()/2 {
				recentE.editor.top = 1
			} else {
				recentE.editor.top = line - recentE.editor.PageSize()/2
			}
			app.Redraw()
			app.Focus(recentE)
			return
		}
		// go to file
		if len(gb.options) > 0 {
			recentE.Open(gb.options[gb.index])
			app.Redraw()
			app.Focus(recentE)
		}
	})
	gb.Handle(tcell.KeyUp, func(k *tcell.EventKey, screen tcell.Screen) {
		gb.index--
		if gb.index < 0 {
			gb.index = len(files) - 1
		}
		gb.Draw(screen)
	})
	gb.Handle(tcell.KeyDown, func(k *tcell.EventKey, screen tcell.Screen) {
		gb.index++
		if gb.index > len(files)-1 {
			gb.index = 0
		}
		gb.Draw(screen)
	})
	gb.Handle(tcell.KeyCtrlBackslash, func(k *tcell.EventKey, screen tcell.Screen) {
		g := NewEditorGroup(app.Screen(), statusBar.Status)
		g.Open(gb.options[gb.index])
		editors.Views = append(editors.Views, g)
		app.Focus(g)
		app.Redraw()
	})

	app.Handle(tcell.KeyCtrlQ, func(*tcell.EventKey) {
		// force quit
		app.Close()
	})
	app.Handle(tcell.KeyCtrlF, func(*tcell.EventKey) {
		recentE.editor.find.line = recentE.editor.cursor.row
		width, _ := app.Screen().Size()
		fb.SetPos(width-40, 1, 40, 1)
		app.Focus(fb)
		fb.Draw(app.Screen())
	})
	app.Handle(tcell.KeyCtrlS, func(*tcell.EventKey) {
		if !recentE.editor.dirty {
			return
		}
		if recentE.editor.filename == "" {
			app.Focus(sb)
			sb.Draw(app.Screen())
		} else {
			f, err := os.Create(recentE.editor.filename)
			if err != nil {
				log.Print(err)
				return
			}
			_, err = recentE.editor.WriteTo(f)
			f.Close()
			if err != nil {
				log.Print(err)
				return
			}
		}
	})
	app.Handle(tcell.KeyCtrlP, func(*tcell.EventKey) {
		gb.keyword = nil
		gb.index = 0
		width, _ := app.Screen().Size()
		gb.SetPos((width-optionWidth)/2, 3, optionWidth, 1)
		gb.Draw(app.Screen())
		app.Focus(gb)
	})
	app.Handle(tcell.KeyCtrlG, func(*tcell.EventKey) {
		gb.keyword = nil
		gb.index = 0
		width, _ := app.Screen().Size()
		gb.SetPos((width-optionWidth)/2, 3, optionWidth, 1)
		gb.keyword = []rune{':'}
		gb.Draw(app.Screen())
		app.Focus(gb)
	})
	app.Handle(tcell.KeyCtrlW, func(*tcell.EventKey) {
		if recentE.editor.dirty && !sb.prompt {
			app.Focus(sb)
			sb.name = []rune(e.editor.filename)
			sb.prompt = true
			sb.Draw(app.Screen())
			return
		}

		recentE.CloseOne()
		if len(recentE.titleBar.names) > 0 {
			recentE.Draw(app.Screen())
			return
		}

		if len(editors.Views) == 1 {
			app.Close()
			return
		}

		// delete editor
		var i int
		for i = range editors.Views {
			if editors.Views[i] == recentE {
				break
			}
		}
		editors.Views = slices.Delete(editors.Views, i, i+1)
		j := i - 1
		if j < 0 {
			j = 0
		}
		prevE := editors.Views[j].(*EditorGroup)
		app.Focus(prevE)
		app.Redraw()
	})
	app.Focus(e)
	app.Run()
}
