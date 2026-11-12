package gui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestSplitDefersPanelResizeUntilDragEnd(t *testing.T) {
	_, w := newTestApp(t, 0)
	s, ok := w.Content().(*deferredSplit)
	if !ok {
		t.Fatal("в окне отсутствует разделитель панелей")
	}
	// Создаём рендерер и достаточно широкое окно, чтобы у панелей был запас.
	_ = test.WidgetRenderer(s)
	s.Resize(fyne.NewSize(2000, 760))
	initial := s.leading.Size().Width
	for i := 0; i < 40; i++ {
		s.handle.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DX: 2}})
		if s.leading.Size().Width != initial {
			t.Fatal("панель изменила размер во время перетаскивания")
		}
	}
	if s.handle.Position().X <= initial {
		t.Fatal("разделитель не переместился")
	}
	s.handle.DragEnd()
	if s.leading.Size().Width <= initial || s.handle.Position().X != s.leading.Size().Width {
		t.Fatal("панели не приняли новое положение после отпускания")
	}
}

func TestSplitClampsAndHandlesSmallSize(t *testing.T) {
	_, w := newTestApp(t, 0)
	s := w.Content().(*deferredSplit)
	_ = test.WidgetRenderer(s)
	s.Resize(fyne.NewSize(2000, 760))
	s.handle.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DX: 100000}})
	_, max := s.bounds(s.Size().Width)
	if s.handle.Position().X != max {
		t.Fatal("разделитель вышел за допустимую границу")
	}
	s.handle.DragEnd()
	s.Resize(fyne.NewSize(100, 100))
	if s.leading.Size().Width < 0 || s.trailing.Size().Width < 0 {
		t.Fatal("при малом размере получены отрицательные размеры")
	}
}

func TestSplitDragThroughCanvas(t *testing.T) {
	_, w := newTestApp(t, 0)
	w.Resize(fyne.NewSize(2000, 760))
	s := w.Content().(*deferredSplit)
	_ = test.WidgetRenderer(s)
	before := s.leading.Size().Width
	pos := fyne.NewPos(s.handle.Position().X+s.handle.Size().Width/2, s.handle.Size().Height/2)
	test.Drag(w.Canvas(), pos, 60, 0)
	if s.leading.Size().Width <= before {
		t.Fatal("перетаскивание через окно не изменило ширину панели")
	}
}
