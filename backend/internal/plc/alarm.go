package plc

import (
	"fmt"
	"sync"
	"time"
)

// Alarm 是一則警報事件:「發生」或「解除」各算一筆
type Alarm struct {
	ID         int64     `json:"id"`
	MachineID  string    `json:"machine_id"`
	Kind       string    `json:"kind"`
	Message    string    `json:"message"`
	Value      float64   `json:"value"`
	Active     bool      `json:"active"` // true=發生 false=解除
	OccurredAt time.Time `json:"occurred_at"`
}

type AlarmMonitor struct {
	mu         sync.Mutex
	highTemp   float64         // 超過這個溫度就警報
	hysteresis float64         // 要降到「上限減這個數」以下才解除
	active     map[string]bool // 目前正在響的警報
}

func NewAlarmMonitor(highTemp, hysteresis float64) *AlarmMonitor {
	return &AlarmMonitor{highTemp: highTemp, hysteresis: hysteresis, active: map[string]bool{}}
}

// Evaluate 看一筆資料,如果造成警報「發生」或「解除」就回傳,否則回傳 nil
func (m *AlarmMonitor) Evaluate(r Reading) *Alarm {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch r.Tag {
	case TagTemperature:
		return m.check(r, "temperature_high",
			r.Value > m.highTemp,
			r.Value <= m.highTemp-m.hysteresis,
			fmt.Sprintf("豆溫 %.1f°C 超過上限 %.0f°C", r.Value, m.highTemp),
			"豆溫已回到正常範圍")
	case TagCycleState:
		return m.check(r, "machine_fault",
			r.Value == StateFault,
			r.Value != StateFault,
			"機台回報故障",
			"機台故障已解除")
	}
	return nil
}

func (m *AlarmMonitor) check(r Reading, kind string, shouldRaise, shouldClear bool, raiseMsg, clearMsg string) *Alarm {
	key := r.MachineID + "/" + kind

	if shouldRaise && !m.active[key] { // 剛開始出問題:響一次
		m.active[key] = true
		return &Alarm{MachineID: r.MachineID, Kind: kind, Message: raiseMsg, Value: r.Value, Active: true, OccurredAt: r.At}
	}
	if shouldClear && m.active[key] { // 剛恢復正常:通知一次
		delete(m.active, key)
		return &Alarm{MachineID: r.MachineID, Kind: kind, Message: clearMsg, Value: r.Value, Active: false, OccurredAt: r.At}
	}
	return nil
}
