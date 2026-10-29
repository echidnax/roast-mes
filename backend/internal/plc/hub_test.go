package plc

import (
	"testing"
	"time"
)

func TestHubDeadband(t *testing.T) {
	count := 0
	h := NewHub(func(r Reading) { count++ })
	at := time.Now()

	steps := []struct {
		tag  Tag
		v    float64
		want bool
	}{
		{TagTemperature, 100.0, true},  // 第一筆一定算
		{TagTemperature, 100.3, false}, // 差 0.3,太小
		{TagTemperature, 100.6, true},  // 跟 100.0 差 0.6,夠大
		{TagTemperature, 100.7, false}, // 跟 100.6 差 0.1,太小
		{TagTemperature, 101.3, true},  // 跟 100.6 差 0.7,夠大
		{TagCycleState, 1, true},
		{TagCycleState, 1, false}, // 狀態沒變
		{TagCycleState, 2, true},  // 狀態變了
	}
	for i, s := range steps {
		got := h.Ingest(Reading{MachineID: "RS-01", Tag: s.tag, Value: s.v, At: at})
		if got != s.want {
			t.Fatalf("第 %d 步 (%s=%v): 期望 %v, 實際 %v", i, s.tag, s.v, s.want, got)
		}
	}
	if count != 5 {
		t.Fatalf("onChange 應該被呼叫 5 次,實際 %d", count)
	}
}

func TestHubSnapshotIsCopy(t *testing.T) {
	h := NewHub(nil)
	h.Ingest(Reading{MachineID: "RS-01", Tag: TagAirflow, Value: 12, At: time.Now()})

	snaps := h.Snapshots()
	if len(snaps) != 1 || snaps[0].Values[TagAirflow] != 12 {
		t.Fatalf("快照內容錯誤: %+v", snaps)
	}

	snaps[0].Values[TagAirflow] = 0 // 亂改影本
	if h.Snapshots()[0].Values[TagAirflow] != 12 {
		t.Fatal("改影本不應該影響 Hub")
	}
}
