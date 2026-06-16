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
	maxChars int

	scroll *container.Scroll

	// Pre-allocated buffer for watching file changes
	readBuf []byte
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
		lines:    make([]int64, 1, 1048576), // Pre-allocate 1M lines
		maxChars: 0,
		readBuf:  make([]byte, 65536), // 64KB read buffer
	}
	l.lines[0] = 0

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

			remaining := newSize - l.fileSize
			currentOffset := l.fileSize
			for remaining > 0 {
				toRead := int64(len(l.readBuf))
				if remaining < toRead {
					toRead = remaining
				}
				_, _ = l.reader.ReadAt(l.readBuf[:toRead], currentOffset)

				for i := int64(0); i < toRead; i++ {
					if l.readBuf[i] == '\n' {
						lastLineStart := l.lines[len(l.lines)-1]
						lineEnd := currentOffset + i
						lineLen := int(lineEnd - lastLineStart)
						if lineLen > l.maxChars {
							l.maxChars = min(lineLen, 2048)
						}
						l.lines = append(l.lines, currentOffset+i+1)
					}
				}
				currentOffset += toRead
				remaining -= toRead
			}

			// Check current last line
			lastLineLen := int(newSize - l.lines[len(l.lines)-1])
			if lastLineLen > l.maxChars {
				l.maxChars = min(lastLineLen, 2048)
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

	// Reusable buffers
	objectBuf    []fyne.CanvasObject
	lineBuf      []byte
	labelOffsets []int64
}

func (r *logContentRenderer) Layout(size fyne.Size) {}

func (r *logContentRenderer) MinSize() fyne.Size {
	r.c.view.mu.RLock()
	defer r.c.view.mu.RUnlock()

	// Calculate width based on maxChars and monospace font size
	// 0.6 is a common width/height ratio for monospace fonts
	charWidth := theme.TextSize() * 0.6
	width := float32(r.c.view.maxChars) * charWidth

	// Add some padding
	width += theme.Padding() * 2

	return fyne.NewSize(width, float32(len(r.c.view.lines))*logLineHeight)
}

func (r *logContentRenderer) Objects() []fyne.CanvasObject {
	view := r.c.view
	scroll := view.scroll

	offsetY := scroll.Offset.Y
	viewHeight := scroll.Size().Height

	startLine := int(offsetY / logLineHeight)
	if startLine < 0 {
		startLine = 0
	}
	numLines := int(viewHeight/logLineHeight) + 2
	if numLines < 0 {
		numLines = 0
	}

	view.mu.RLock()
	defer view.mu.RUnlock()

	if len(view.lines) == 0 || startLine >= len(view.lines) {
		return nil
	}

	endLine := max(startLine, min(startLine+numLines, len(view.lines)))

	needed := endLine - startLine
	if len(r.labels) < needed {
		for i := len(r.labels); i < needed; i++ {
			t := canvas.NewText("", theme.Color(theme.ColorNameForeground))
			t.TextStyle = fyne.TextStyle{Monospace: true}
			t.TextSize = theme.TextSize()
			r.labels = append(r.labels, t)
			r.labelOffsets = append(r.labelOffsets, -1)
		}
	}

	if cap(r.objectBuf) < needed {
		r.objectBuf = make([]fyne.CanvasObject, needed)
	}
	r.objectBuf = r.objectBuf[:needed]

	if len(r.lineBuf) < 2048 {
		r.lineBuf = make([]byte, 2048)
	}

	for i := 0; i < needed; i++ {
		lineIdx := startLine + i
		start := view.lines[lineIdx]

		label := r.labels[i]
		isLastLine := lineIdx == len(view.lines)-1
		if r.labelOffsets[i] != start || isLastLine {
			var end int64
			if isLastLine {
				end = view.fileSize
			} else {
				end = view.lines[lineIdx+1]
			}

			if end > start {
				readLen := int(min(end-start, 2048))
				_, _ = view.reader.ReadAt(r.lineBuf[:readLen], start)
				newText := string(bytes.TrimRight(r.lineBuf[:readLen], "\r\n"))
				if label.Text != newText {
					label.Text = newText
					label.Refresh()
				}
			} else if label.Text != "" {
				label.Text = ""
				label.Refresh()
			}
			r.labelOffsets[i] = start
		}

		label.Color = theme.Color(theme.ColorNameForeground)
		label.Move(fyne.NewPos(0, float32(lineIdx)*logLineHeight))
		r.objectBuf[i] = label
	}

	return r.objectBuf
}

func (r *logContentRenderer) Refresh() {}

func (r *logContentRenderer) Destroy() {}
