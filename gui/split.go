package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// deferredSplit перемещает разделитель во время перетаскивания и изменяет
// размеры панелей только при отпускании: сложные вкладки не перестраиваются
// на каждом событии движения мыши.
type deferredSplit struct {
	widget.BaseWidget
	leading, trailing fyne.CanvasObject
	offset            float32
	handle            *splitHandle
}

func newDeferredSplit(leading, trailing fyne.CanvasObject) *deferredSplit {
	s := &deferredSplit{leading: leading, trailing: trailing, offset: 0.25}
	s.ExtendBaseWidget(s)
	s.handle = &splitHandle{split: s}
	s.handle.ExtendBaseWidget(s.handle)
	return s
}

func (s *deferredSplit) CreateRenderer() fyne.WidgetRenderer {
	return &deferredSplitRenderer{split: s, objects: []fyne.CanvasObject{s.leading, s.trailing, s.handle}}
}

func (s *deferredSplit) bounds(width float32) (float32, float32) {
	available := fyne.Max(0, width-s.handle.MinSize().Width)
	left, right := s.leading.MinSize().Width, s.trailing.MinSize().Width
	if left+right > available {
		if left+right == 0 {
			return 0, available
		}
		pos := available * left / (left + right)
		return pos, pos
	}
	return left, available - right
}

func (s *deferredSplit) clamp(pos, width float32) float32 {
	min, max := s.bounds(width)
	return fyne.Min(max, fyne.Max(min, pos))
}

type deferredSplitRenderer struct {
	split   *deferredSplit
	objects []fyne.CanvasObject
}

func (r *deferredSplitRenderer) Layout(size fyne.Size) {
	s := r.split
	w := s.handle.MinSize().Width
	available := fyne.Max(0, size.Width-w)
	pos := s.clamp(available*s.offset, size.Width)
	s.leading.Move(fyne.NewPos(0, 0))
	s.leading.Resize(fyne.NewSize(pos, size.Height))
	s.trailing.Move(fyne.NewPos(pos+w, 0))
	s.trailing.Resize(fyne.NewSize(fyne.Max(0, available-pos), size.Height))
	s.handle.Move(fyne.NewPos(pos, 0))
	s.handle.Resize(fyne.NewSize(w, size.Height))
}

func (r *deferredSplitRenderer) MinSize() fyne.Size {
	l, t := r.split.leading.MinSize(), r.split.trailing.MinSize()
	return fyne.NewSize(l.Width+t.Width+r.split.handle.MinSize().Width, fyne.Max(l.Height, t.Height))
}
func (r *deferredSplitRenderer) Refresh()                     { r.Layout(r.split.Size()); canvas.Refresh(r.split) }
func (r *deferredSplitRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *deferredSplitRenderer) Destroy()                     {}

type splitHandle struct {
	widget.BaseWidget
	split    *deferredSplit
	dragging bool
	position float32
}

var _ fyne.Draggable = (*splitHandle)(nil)
var _ desktop.Cursorable = (*splitHandle)(nil)

func (h *splitHandle) Cursor() desktop.Cursor { return desktop.HResizeCursor }
func (h *splitHandle) Dragged(e *fyne.DragEvent) {
	if !h.dragging {
		h.dragging = true
		h.position = h.Position().X
	}
	h.position = h.split.clamp(h.position+e.Dragged.DX, h.split.Size().Width)
	h.Move(fyne.NewPos(h.position, 0))
}
func (h *splitHandle) DragEnd() {
	if !h.dragging {
		return
	}
	h.dragging = false
	available := h.split.Size().Width - h.MinSize().Width
	if available > 0 {
		h.split.offset = h.position / available
	}
	h.split.Refresh()
}

func (h *splitHandle) CreateRenderer() fyne.WidgetRenderer {
	r := &splitHandleRenderer{handle: h, background: canvas.NewRectangle(nil), mark: canvas.NewRectangle(nil)}
	r.objects = []fyne.CanvasObject{r.background, r.mark}
	r.Refresh()
	return r
}

type splitHandleRenderer struct {
	handle           *splitHandle
	background, mark *canvas.Rectangle
	objects          []fyne.CanvasObject
}

func (r *splitHandleRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	w := theme.Padding() / 2
	r.mark.Move(fyne.NewPos((size.Width-w)/2, (size.Height-theme.Padding()*4)/2))
	r.mark.Resize(fyne.NewSize(w, theme.Padding()*4))
}
func (r *splitHandleRenderer) MinSize() fyne.Size {
	return fyne.NewSize(theme.Padding()*2, theme.Padding()*6)
}
func (r *splitHandleRenderer) Refresh() {
	r.background.FillColor = theme.Color(theme.ColorNameShadow)
	r.mark.FillColor = theme.Color(theme.ColorNameForeground)
	r.background.Refresh()
	r.mark.Refresh()
}
func (r *splitHandleRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *splitHandleRenderer) Destroy()                     {}
