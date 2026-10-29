package workorder

import (
	"errors"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)

func mustNew(t *testing.T) *WorkOrder {
	t.Helper()
	w, err := New("WO-1", "衣索比亞 淺焙", 10, now)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestNewValidation(t *testing.T) {
	cases := []struct {
		name, no, spec string
		qty            int
	}{
		{"編號空白", " ", "衣索比亞 淺焙", 1},
		{"規格空白", "WO-1", "", 1},
		{"數量為零", "WO-1", "衣索比亞 淺焙", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := New(c.no, c.spec, c.qty, now); !errors.Is(err, ErrValidation) {
				t.Fatalf("want ErrValidation, got %v", err)
			}
		})
	}
}

func TestTransitions(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(w *WorkOrder)
		to      Status
		machine string
		wantErr error
	}{
		{"派工要有機台", nil, StatusReleased, "", ErrValidation},
		{"派工成功", nil, StatusReleased, "RS-01", nil},
		{"不能跳過派工直接開工", nil, StatusRunning, "RS-01", ErrInvalidTransition},
		{"數量不足不能完工", func(w *WorkOrder) { w.Status = StatusRunning; w.MachineID = "RS-01"; w.ProducedQty = 9 }, StatusCompleted, "", ErrValidation},
		{"數量足夠可完工", func(w *WorkOrder) { w.Status = StatusRunning; w.MachineID = "RS-01"; w.ProducedQty = 10 }, StatusCompleted, "", nil},
		{"取消後是終點", func(w *WorkOrder) { w.Status = StatusCancelled }, StatusReleased, "RS-01", ErrInvalidTransition},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := mustNew(t)
			if c.prepare != nil {
				c.prepare(w)
			}
			before := *w
			err := w.Transition(c.to, c.machine, now.Add(time.Minute))
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("want %v, got %v", c.wantErr, err)
			}
			if err != nil && *w != before {
				t.Fatal("失敗時不可以改動工單")
			}
			if err == nil && w.Status != c.to {
				t.Fatalf("status = %s, want %s", w.Status, c.to)
			}
		})
	}
}

func TestReport(t *testing.T) {
	w := mustNew(t)
	if err := w.Report(1, now); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("未開工就回報: got %v", err)
	}
	w.Status = StatusRunning
	if err := w.Report(0, now); !errors.Is(err, ErrValidation) {
		t.Fatalf("數量 0: got %v", err)
	}
	if err := w.Report(6, now); err != nil {
		t.Fatal(err)
	}
	if err := w.Report(5, now); !errors.Is(err, ErrValidation) {
		t.Fatalf("超過計畫: got %v", err)
	}
	if err := w.Report(4, now); err != nil || w.ProducedQty != 10 {
		t.Fatalf("剛好達標: err=%v produced=%d", err, w.ProducedQty)
	}
}
