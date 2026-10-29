package plc

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

type phase int

const (
	phaseIdle     phase = iota // 待機
	phaseRoasting              // 烘焙中
	phaseDone                  // 完成,降溫中
	phaseFault                 // 故障(加熱器卡住,溫度一直升)
)

// simState 是一台模擬機台目前的內部狀態
type simState struct {
	temp, setTemp, airflow float64
	phase                  phase
	left                   int     // 目前階段還剩幾秒
	faultChance            float64 // 烘焙中每秒發生故障的機率
}

// SimSource 模擬烘豆機:待機 → 烘焙 → 完成 → 待機,偶爾故障
type SimSource struct {
	machines []MachineConfig
	mu       sync.Mutex
	st       map[string]*simState
}

func NewSim(machines []MachineConfig) *SimSource {
	s := &SimSource{machines: machines, st: map[string]*simState{}}
	for i, m := range machines {
		// 每台錯開開始時間,畫面上才不會全部同步
		s.st[m.ID] = &simState{
			temp:        60,
			setTemp:     205,
			phase:       phaseIdle,
			left:        3 + i*12,
			faultChance: 0.002,
		}
	}
	return s
}

// Run 每秒產生一批資料,送進 out
func (s *SimSource) Run(ctx context.Context, out chan<- Reading) error {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-t.C:
			for _, r := range s.tick(now) {
				select {
				case out <- r:
				case <-ctx.Done():
					return nil
				}
			}
		}
	}
}

// tick 讓每台機台往前走一秒,回傳這一秒的所有讀數
func (s *SimSource) tick(now time.Time) []Reading {
	s.mu.Lock()
	defer s.mu.Unlock()

	var rs []Reading
	for _, m := range s.machines {
		st := s.st[m.ID]
		step(st)

		remaining := 0.0
		if st.phase == phaseRoasting {
			remaining = float64(st.left)
		}
		add := func(tag Tag, v float64) {
			rs = append(rs, Reading{MachineID: m.ID, Tag: tag, Value: v, At: now})
		}
		add(TagTemperature, round1(st.temp))
		add(TagAirflow, round1(st.airflow))
		add(TagCycleState, cycleState(st.phase))
		add(TagRemainingSec, remaining)
		add(TagSetTemperature, st.setTemp)
	}
	return rs
}

// step 讓一台機台前進一秒
func step(st *simState) {
	noise := rand.Float64() - 0.5 // -0.5 ~ 0.5 的隨機雜訊,模擬感測器抖動

	switch st.phase {
	case phaseIdle:
		st.temp += (60-st.temp)*0.1 + noise*0.2 // 慢慢回到室溫附近
		st.airflow += (0 - st.airflow) * 0.3
		st.left--
		if st.left <= 0 {
			st.phase, st.left = phaseRoasting, 90
		}

	case phaseRoasting:
		st.temp += (st.setTemp-st.temp)*0.12 + noise // 往目標溫度靠近
		st.airflow += (60-st.airflow)*0.2 + noise*2
		st.left--
		if st.left <= 0 {
			st.phase, st.left = phaseDone, 5
		} else if rand.Float64() < st.faultChance {
			st.phase, st.left = phaseFault, 15
		}

	case phaseDone:
		st.temp += (60 - st.temp) * 0.03
		st.airflow += (0 - st.airflow) * 0.5
		st.left--
		if st.left <= 0 {
			st.phase, st.left = phaseIdle, 15
		}

	case phaseFault:
		st.temp += 4 // 加熱器卡住,溫度一直升,之後會觸發超溫警報
		st.airflow += 1
		st.left--
		if st.left <= 0 {
			st.phase, st.left = phaseIdle, 15
		}
	}

	st.airflow = math.Max(0, st.airflow) // 風門開度不會是負的
}

func cycleState(p phase) float64 {
	switch p {
	case phaseRoasting:
		return StateRoasting
	case phaseDone:
		return StateDone
	case phaseFault:
		return StateFault
	}
	return StateIdle
}

// round1 四捨五入到小數 1 位
func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// Write 把值寫進模擬設備。目前只有「目標溫度」可以寫。
func (s *SimSource) Write(_ context.Context, machineID string, tag Tag, value float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.st[machineID]
	if !ok || tag != TagSetTemperature {
		return fmt.Errorf("%w: %s/%s", ErrUnknownTarget, machineID, tag)
	}
	st.setTemp = value
	return nil
}
