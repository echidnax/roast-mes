package plc

import "testing"

func TestAlarmLog(t *testing.T) {
	l := NewAlarmLog()
	for i := 0; i < 3; i++ {
		l.Add(Alarm{MachineID: "RS-01", Kind: "machine_fault", Active: true})
	}

	got := l.List(2)
	if len(got) != 2 || got[0].ID != 3 || got[1].ID != 2 {
		t.Fatalf("應回傳最新兩筆(編號 3、2),實際 %+v", got)
	}
}
