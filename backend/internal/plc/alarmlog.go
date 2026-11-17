package plc

import "sync"

// AlarmLog 是警報紀錄簿,最多保留 200 筆(目前放在記憶體,程式重啟會清空)
type AlarmLog struct {
	mu     sync.Mutex
	nextID int64
	items  []Alarm
}

func NewAlarmLog() *AlarmLog { return &AlarmLog{} }

// Add 記下一則警報,並幫它編號
func (l *AlarmLog) Add(a Alarm) Alarm {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.nextID++
	a.ID = l.nextID
	l.items = append(l.items, a)
	if len(l.items) > 200 {
		l.items = l.items[len(l.items)-200:]
	}
	return a
}

// List 回傳最近 limit 筆,新的在前面
func (l *AlarmLog) List(limit int) []Alarm {
	l.mu.Lock()
	defer l.mu.Unlock()

	out := make([]Alarm, 0, limit)
	for i := len(l.items) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, l.items[i])
	}
	return out
}
