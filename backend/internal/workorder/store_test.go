package workorder

import (
	"errors"
	"testing"
)

func newOrder(t *testing.T, no string) *WorkOrder {
	t.Helper()
	w, err := New(no, "spec", 5, now)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestStoreDuplicateAndConflict(t *testing.T) {
	s := NewStore()
	a := newOrder(t, "WO-1")
	if err := s.Create(a); err != nil {
		t.Fatal(err)
	}
	if a.ID != 1 {
		t.Fatalf("第一張工單 ID 應為 1,實際 %d", a.ID)
	}
	if err := s.Create(newOrder(t, "WO-1")); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("編號重複應報錯: got %v", err)
	}

	// 兩個人同時拿到同一張工單
	x, _ := s.Get(a.ID)
	y, _ := s.Get(a.ID)
	if err := x.Transition(StatusCancelled, "", now); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(x); err != nil {
		t.Fatal(err)
	}
	// y 手上的是舊版本,存回去要被擋下來
	if err := s.Update(y); !errors.Is(err, ErrConflict) {
		t.Fatalf("舊版本應該衝突: got %v", err)
	}
}

func TestStoreMachineBusy(t *testing.T) {
	s := NewStore()
	a := newOrder(t, "WO-1")
	b := newOrder(t, "WO-2")
	s.Create(a)
	s.Create(b)

	for _, w := range []*WorkOrder{a, b} {
		if err := w.Transition(StatusReleased, "RS-01", now); err != nil {
			t.Fatal(err)
		}
		if err := s.Update(w); err != nil {
			t.Fatal(err)
		}
	}

	if err := a.Transition(StatusRunning, "", now); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}

	if err := b.Transition(StatusRunning, "", now); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(b); !errors.Is(err, ErrMachineBusy) {
		t.Fatalf("同一台機器不能同時開兩張工單: got %v", err)
	}
}
