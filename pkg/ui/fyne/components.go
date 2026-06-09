package fyne

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/v2/pkg/core"
)

func createHeader() fyne.CanvasObject {
	icon := canvas.NewImageFromResource(resourceDefaultIcon)
	icon.SetMinSize(fyne.NewSize(24, 24))
	icon.FillMode = canvas.ImageFillContain

	return container.NewHBox(
		icon,
		widget.NewLabel("SabaLauncher"),
		layout.NewSpacer(),
	)
}

type taskBar struct {
	label    *widget.Label
	progress *widget.ProgressBar
	vbox     *fyne.Container
}

type MultiProgress struct {
	widget.BaseWidget

	mainLabel    *widget.Label
	mainProgress *widget.ProgressBar

	bars          map[string]*taskBar
	barsContainer *fyne.Container
	scrollBars    *container.Scroll

	mu sync.Mutex

	container *fyne.Container
}

func NewMultiProgress(defaultText string) *MultiProgress {
	m := &MultiProgress{
		mainLabel:    widget.NewLabel(defaultText),
		mainProgress: widget.NewProgressBar(),
		bars:         make(map[string]*taskBar),
	}

	m.barsContainer = container.NewVBox()
	m.scrollBars = container.NewScroll(m.barsContainer)
	// Give it some sensible height, it will be wrapped by other layouts usually
	m.scrollBars.SetMinSize(fyne.NewSize(300, 400))
	m.scrollBars.Hide() // Hide initially if empty

	m.container = container.NewVBox(
		m.mainLabel,
		m.mainProgress,
		m.scrollBars,
	)

	m.ExtendBaseWidget(m)
	return m
}

func (m *MultiProgress) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(m.container)
}

func (m *MultiProgress) Update(p core.ProgressEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch p.Category {
	case core.ProgressCategoryMain, "":
		m.mainProgress.SetValue(p.Percentage / 100.0)
		if p.Status != "" {
			m.mainLabel.SetText(fmt.Sprintf("%s (%s)", p.TaskName, p.Status))
		} else {
			m.mainLabel.SetText(p.TaskName)
		}
	case core.ProgressCategoryDownload:
		tb, exists := m.bars[p.TaskName]
		if !exists {
			lbl := widget.NewLabel(p.TaskName)
			prog := widget.NewProgressBar()
			vb := container.NewVBox(lbl, prog)
			tb = &taskBar{label: lbl, progress: prog, vbox: vb}
			m.bars[p.TaskName] = tb
			m.barsContainer.Add(vb)

			if len(m.bars) == 1 {
				m.scrollBars.Show()
			}

			m.barsContainer.Refresh()
			m.scrollBars.ScrollToBottom()
		}

		tb.progress.SetValue(p.Percentage / 100.0)
		if p.Status != "" {
			tb.label.SetText(fmt.Sprintf("%s (%s)", p.TaskName, p.Status))
		} else {
			tb.label.SetText(p.TaskName)
		}

		if p.IsFinished || p.Percentage >= 100.0 {
			m.barsContainer.Remove(tb.vbox)
			delete(m.bars, p.TaskName)

			if len(m.bars) == 0 {
				m.scrollBars.Hide()
			}
		}
	}
}
