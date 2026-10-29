package plc

import (
	"context"
	"errors"
	"time"
)

// Tag 是 PLC 上一個點位的名稱
type Tag string

const (
	TagTemperature    Tag = "temperature"     // 豆溫 °C
	TagAirflow        Tag = "airflow"         // 風門開度 %
	TagCycleState     Tag = "cycle_state"     // 機台狀態,見下方 State 常數
	TagRemainingSec   Tag = "remaining_sec"   // 剩餘烘焙秒數
	TagSetTemperature Tag = "set_temperature" // 目標溫度(可從網頁寫入)
)

// cycle_state 的數值意義
const (
	StateIdle     float64 = 0
	StateRoasting float64 = 1
	StateDone     float64 = 2
	StateFault    float64 = 3
)

var (
	ErrUnknownTarget = errors.New("unknown machine or tag")
	ErrOffline       = errors.New("plc offline")
)

// MachineConfig 描述一台機台
type MachineConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Reading 是某台機台某個點位在某個時間的數值
type Reading struct {
	MachineID string    `json:"machine_id"`
	Tag       Tag       `json:"tag"`
	Value     float64   `json:"value"`
	At        time.Time `json:"at"`
}

// Source 就是「設備」:模擬器和真正的 OPC UA 都要符合這個介面
type Source interface {
	// Run 會一直執行,直到 ctx 被取消,把讀到的資料丟進 out
	Run(ctx context.Context, out chan<- Reading) error
	// Write 把一個值寫進設備的某個點位
	Write(ctx context.Context, machineID string, tag Tag, value float64) error
}
