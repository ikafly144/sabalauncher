package window

import (
	"fmt"
	"log/slog"
	"sync"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget/material"
)

var (
	wg sync.WaitGroup
)

func Wait() {
	wg.Wait()
}

func NewWindow[R IFRender](id string, r R) *Window[R] {
	w := &Window[R]{
		id:         id,
		r:          r,
		eventsChan: make(chan func(), 1),
	}
	w.window = new(app.Window)
	return w
}

type Window[R IFRender] struct {
	id     string
	window *app.Window

	closeHandler func()

	eventsChan chan func()

	r R
}

func (w *Window[R]) HandleClose(handler func()) {
	w.closeHandler = handler
}

func (w *Window[R]) InjectEvent() chan<- func() {
	return w.eventsChan
}

func (w *Window[R]) HookAsync() {
	wg.Add(1)
	go w.hook()
}

func (w *Window[R]) Hook() error {
	wg.Add(1)
	w.hook()
	return nil
}

func (w *Window[R]) hook() {
	defer func() {
		wg.Done()
	}()
	if err := w.handleEvent(); err != nil {
		slog.Error("Error handling event", "error", err, "id", w.id)
	}
	slog.Info("Window closed", "id", w.id)
}

func (w *Window[R]) handleEvent() error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	events := make(chan event.Event)
	acks := make(chan struct{})

	go func() {
		for {
			ev := w.window.Event()
			events <- ev
			<-acks
			if _, ok := ev.(app.DestroyEvent); ok {
				return
			}
		}
	}()

	go func() {
		for f := range w.eventsChan {
			f()
		}
	}()

	var ops op.Ops

	for e := range events {
		switch e := e.(type) {
		case app.DestroyEvent:
			acks <- struct{}{}
			if w.closeHandler != nil {
				w.closeHandler()
			}
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			w.r.Render(&RenderContext{
				Gtx:   gtx,
				Theme: th,
			})
			e.Frame(gtx.Ops)
		}
		acks <- struct{}{}
	}
	slog.Warn("Window closed unexpectedly", "id", w.id)
	return fmt.Errorf("Window closed unexpectedly")
}

func (w *Window[R]) Option(opts ...app.Option) {
	w.window.Option(opts...)
}
