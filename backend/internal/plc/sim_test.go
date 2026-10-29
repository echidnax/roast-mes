package plc

import (
	"context"
	"errors"
	"testing"
)

func TestStepCycle(t *testing.T) {
	st := &simState{temp: 60, setTemp: 205, phase: phaseIdle, left: 1}

	step(st)
	if st.phase != phaseRoasting || st.left != 90 {
		t.Fatalf("待機結束應進入烘焙: phase=%v left=%d", st.phase, st.left)
	}
	st.left = 1
	step(st)
	if st.phase != phaseDone || st.left != 5 {
		t.Fatalf("烘焙結束應進入完成: phase=%v left=%d", st.phase, st.left)
	}
	st.left = 1
	step(st)
	if st.phase != phaseIdle || st.left != 15 {
		t.Fatalf("完成後應回待機: phase=%v left=%d", st.phase, st.left)
	}
}

func TestRoastingHeatsUp(t *testing.T) {
	st := &simState{temp: 60, setTemp: 205, phase: phaseRoasting, left: 90}
	for i := 0; i < 40; i++ {
		step(st)
	}
	if st.temp < 190 || st.temp > 215 {
		t.Fatalf("40 秒後豆溫應接近 205,實際 %.1f", st.temp)
	}
	if st.airflow < 40 {
		t.Fatalf("烘焙中風門開度應升高,實際 %.1f", st.airflow)
	}
}

func TestFaultOverheats(t *testing.T) {
	st := &simState{temp: 205, setTemp: 205, phase: phaseFault, left: 15}
	for i := 0; i < 10; i++ {
		step(st)
	}
	if st.temp < 235 {
		t.Fatalf("故障 10 秒後溫度應明顯升高,實際 %.1f", st.temp)
	}
}

func TestCycleStateAndRound(t *testing.T) {
	if cycleState(phaseRoasting) != StateRoasting ||
		cycleState(phaseDone) != StateDone ||
		cycleState(phaseFault) != StateFault ||
		cycleState(phaseIdle) != StateIdle {
		t.Fatal("cycleState 對應錯誤")
	}
	if round1(1.26) != 1.3 {
		t.Fatalf("round1(1.26) = %v", round1(1.26))
	}
}

func TestWrite(t *testing.T) {
	s := NewSim([]MachineConfig{{ID: "RS-01", Name: "烘豆機 01"}})
	ctx := context.Background()

	if err := s.Write(ctx, "RS-01", TagSetTemperature, 210); err != nil {
		t.Fatal(err)
	}
	if s.st["RS-01"].setTemp != 210 {
		t.Fatal("目標溫度沒有被寫入")
	}
	if err := s.Write(ctx, "RS-99", TagSetTemperature, 210); !errors.Is(err, ErrUnknownTarget) {
		t.Fatalf("未知機台: got %v", err)
	}
	if err := s.Write(ctx, "RS-01", TagTemperature, 100); !errors.Is(err, ErrUnknownTarget) {
		t.Fatalf("唯讀點位不可寫: got %v", err)
	}
}
