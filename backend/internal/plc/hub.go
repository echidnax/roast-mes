package plc

import (
	"context"
	"math"
	"sync"
	"time"
)

// Snapshot 是一台機器「現在」的狀態,像公告欄上的一張卡片
type Snapshot struct {
	MachineID string
	Values    map[Tag]float64
	UpdatedAt time.Time
}

// 變化小於這個數字,就當作沒變(感測器抖動)
var deadband = map[Tag]float64{
	TagTemperature: 0.5, // 豆溫差不到 0.5 度,不算變
	TagAirflow:     1,   // 風門開度差不到 1%,不算變
}

type Hub struct {
	mu       sync.Mutex
	latest   map[string]*Snapshot
	onChange func(Reading) // 有「真的變化」時,通知這個函式
}

func NewHub(onChange func(Reading)) *Hub {
	return &Hub{latest: map[string]*Snapshot{}, onChange: onChange}
}

// Ingest 收一筆資料,回傳「這筆是不是真的變化」
func (h *Hub) Ingest(r Reading) bool {
	h.mu.Lock()

	// 找這台機器的卡片,沒有就新做一張
	snap, ok := h.latest[r.MachineID]
	if !ok {
		snap = &Snapshot{MachineID: r.MachineID, Values: map[Tag]float64{}}
		h.latest[r.MachineID] = snap
	}
	snap.UpdatedAt = r.At // 不管有沒有變,都記下「剛剛有收到」,之後判斷離線用

	// 跟「上一次被接受的值」比較
	old, had := snap.Values[r.Tag]
	changed := !had || math.Abs(r.Value-old) > deadband[r.Tag]
	if changed {
		snap.Values[r.Tag] = r.Value
	}
	h.mu.Unlock()

	if changed && h.onChange != nil {
		h.onChange(r)
	}
	return changed
}

// Run 一直從輸送帶拿資料,交給 Ingest
func (h *Hub) Run(ctx context.Context, in <-chan Reading) {
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-in:
			h.Ingest(r)
		}
	}
}

// Snapshots 回傳所有機器的狀態「影本」,外面改影本不會影響 Hub
func (h *Hub) Snapshots() []Snapshot {
	h.mu.Lock()
	defer h.mu.Unlock()

	out := make([]Snapshot, 0, len(h.latest))
	for _, s := range h.latest {
		cp := Snapshot{MachineID: s.MachineID, UpdatedAt: s.UpdatedAt, Values: map[Tag]float64{}}
		for k, v := range s.Values {
			cp.Values[k] = v
		}
		out = append(out, cp)
	}
	return out
}
