package main

import "github.com/gdamore/tcell/v2"

// Application framework
type App struct {
	body View

	focus  View
	done   chan struct{}
	mouseX int
	mouseY int
	keymap map[tcell.Key]func(*tcell.EventKey)
}

func NewApp() *App {
	return &App{
		done:   make(chan struct{}),
		keymap: make(map[tcell.Key]func(*tcell.EventKey)),
	}
}

func (a *App) SetBody(v View) {
	a.body = v
}

func (a *App) Redraw() {
	a.body.Draw()
}

func (a *App) Close() {
	close(a.done)
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
}

func (a *App) GetHover() View {
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

func (a *App) Handle(key tcell.Key, f func(*tcell.EventKey)) {
	a.keymap[key] = f
}

// Run will not return until Close
func (a *App) Run() {
	a.body.Draw()
	screen.Show()
	for {
		select {
		case <-a.done:
			return
		default:
		}

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			width, height := screen.Size()
			a.body.SetPos(0, 0, width, height)
			a.body.Draw()
			screen.Sync()
			continue
		case *tcell.EventMouse:
			x, y := ev.Position()
			switch ev.Buttons() {
			case tcell.Button1:
				view := a.GetHover()
				a.Focus(view)
				view.OnClick(x, y)
			case tcell.WheelUp:
				view := a.GetHover()
				delta := int(float32(y) * scrollSensitivity)
				view.ScrollUp(delta)
			case tcell.WheelDown:
				view := a.GetHover()
				delta := int(float32(y) * scrollSensitivity)
				view.ScrollDown(delta)
			default:
				a.mouseX = x
				a.mouseY = y
				// do not render on mouse motion
				continue
			}
		case *tcell.EventKey:
			if f, ok := a.keymap[ev.Key()]; ok {
				f(ev)
			} else {
				a.focus.HandleKey(ev)
			}
		}
		screen.Show()
	}
}
