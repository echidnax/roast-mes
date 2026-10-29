package plc

import (
	"testing"
	"time"
)

func TestAlarmHighTemp(t *testing.T) {
	m := NewAlarmMonitor(240, 5) // 超過 240 響,降到 235 以下解除
	temp := func(v float64) *Alarm {
		return m.Evaluate(Reading{MachineID: "RS-01", Tag: TagTemperature, Value: v, At: time.Now()})
	}

	if a := temp(220); a != nil {
		t.Fatalf("220 度正常,不該警報: %+v", a)
	}
	if a := temp(241); a == nil || !a.Active {
		t.Fatalf("241 度應該警報,實際 %+v", a)
	}
	if a := temp(245); a != nil {
		t.Fatal("已經在響了,不該重複響")
	}
	if a := temp(238); a != nil {
		t.Fatal("238 還沒降到 235 以下,警報要繼續")
	}
	if a := temp(235); a == nil || a.Active {
		t.Fatalf("235 應該解除警報,實際 %+v", a)
	}
}

func TestAlarmFault(t *testing.T) {
	m := NewAlarmMonitor(240, 5)
	state := func(v float64) *Alarm {
		return m.Evaluate(Reading{MachineID: "RS-02", Tag: TagCycleState, Value: v, At: time.Now()})
	}

	if a := state(StateFault); a == nil || a.Kind != "machine_fault" || !a.Active {
		t.Fatalf("應該有故障警報,實際 %+v", a)
	}
	if a := state(StateIdle); a == nil || a.Active {
		t.Fatalf("故障解除應該通知,實際 %+v", a)
	}
}
