package fyne

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/exp/mmap"
)

const logLineHeight = 20.0

type MmapLogView struct {
	widget.BaseWidget
	filePath string
	reader   *mmap.ReaderAt
	mu       sync.RWMutex
	lines    []int64
	fileSize int64

	scroll *container.Scroll
}

func NewMmapLogView(logFile io.ReadCloser) *MmapLogView {
	f, ok := logFile.(*os.File)
	if !ok {
		slog.Error("MmapLogView requires a file handle")
		return nil
	}
	path := f.Name()
	f.Close()

	l := &MmapLogView{
		filePath: path,
		lines:    []int64{0},
	}

	content := &logContent{view: l}
	content.ExtendBaseWidget(content)

	l.scroll = container.NewScroll(content)
	l.ExtendBaseWidget(l)

	go l.watchFile()

	return l
}

func (l *MmapLogView) Scrolled(ev *fyne.ScrollEvent) {
	l.scroll.Scrolled(ev)
	l.scroll.Content.Refresh()
}

func (l *MmapLogView) watchFile() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		info, err := os.Stat(l.filePath)
		if err != nil {
			continue
		}

		newSize := info.Size()
		l.mu.Lock()
		if newSize > l.fileSize {
			if l.reader != nil {
				l.reader.Close()
			}
			r, err := mmap.Open(l.filePath)
			if err != nil {
				slog.Error("failed to mmap log file", "err", err)
				l.mu.Unlock()
				continue
			}
			l.reader = r

			buf := make([]byte, newSize-l.fileSize)
			_, _ = l.reader.ReadAt(buf, l.fileSize)

			offset := l.fileSize
			for i, b := range buf {
				if b == '\n' {
					l.lines = append(l.lines, offset+int64(i)+1)
				}
			}

			// Auto-scroll if we were near the bottom
			wasAtBottom := l.scroll.Offset.Y >= float32(len(l.lines)-1)*logLineHeight-l.scroll.Size().Height-100

			l.fileSize = newSize
			l.mu.Unlock()

			fyne.Do(func() {
				l.scroll.Content.Refresh()
				if wasAtBottom {
					l.scroll.ScrollToBottom()
				}
			})
		} else {
			l.mu.Unlock()
		}
	}
}

func (l *MmapLogView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(l.scroll)
}

type logContent struct {
	widget.BaseWidget
	view *MmapLogView
}

func (c *logContent) CreateRenderer() fyne.WidgetRenderer {
	return &logContentRenderer{
		c:      c,
		labels: make([]*canvas.Text, 0),
	}
}

type logContentRenderer struct {
	c      *logContent
	labels []*canvas.Text
}

func (r *logContentRenderer) Layout(size fyne.Size) {}

func (r *logContentRenderer) MinSize() fyne.Size {
	r.c.view.mu.RLock()
	defer r.c.view.mu.RUnlock()
	// Use a large width to allow horizontal scrolling for long lines
	return fyne.NewSize(4000, float32(len(r.c.view.lines))*logLineHeight)
}

func (r *logContentRenderer) Objects() []fyne.CanvasObject {
	view := r.c.view
	scroll := view.scroll

	offsetY := scroll.Offset.Y
	viewHeight := scroll.Size().Height

	startLine := int(offsetY / logLineHeight)
	numLines := int(viewHeight/logLineHeight) + 2

	view.mu.RLock()
	defer view.mu.RUnlock()

	if startLine >= len(view.lines) {
		return nil
	}

	endLine := min(startLine+numLines, len(view.lines))

	needed := endLine - startLine
	if len(r.labels) < needed {
		for i := len(r.labels); i < needed; i++ {
			t := canvas.NewText("", theme.Color(theme.ColorNameForeground))
			t.TextStyle = fyne.TextStyle{Monospace: true}
			t.TextSize = theme.TextSize()
			r.labels = append(r.labels, t)
		}
	}

	res := make([]fyne.CanvasObject, 0, needed)
	for i := range needed {
		lineIdx := startLine + i
		start := view.lines[lineIdx]
		var end int64
		if lineIdx == len(view.lines)-1 {
			end = view.fileSize
		} else {
			end = view.lines[lineIdx+1]
		}

		label := r.labels[i]
		if end > start {
			readLen := min(end-start,
				// Limit line length for safety
				2048)
			buf := make([]byte, readLen)
			_, _ = view.reader.ReadAt(buf, start)
			label.Text = string(bytes.TrimRight(buf, "\r\n"))
		} else {
			label.Text = ""
		}
		label.Color = theme.Color(theme.ColorNameForeground)
		label.Move(fyne.NewPos(0, float32(lineIdx)*logLineHeight))
		label.Refresh()
		res = append(res, label)
	}

	return res
}

func (r *logContentRenderer) Refresh() {}

func (r *logContentRenderer) Destroy() {}
