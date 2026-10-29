package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/Echidnax/roast-mes/internal/plc"
)

func main() {
	machines := []plc.MachineConfig{
		{ID: "RS-01", Name: "烘豆機 01"},
		{ID: "RS-02", Name: "烘豆機 02"},
	}
	sim := plc.NewSim(machines)
	monitor := plc.NewAlarmMonitor(240, 5)

	// Hub 判斷「真的變了」時,會呼叫這個函式
	hub := plc.NewHub(func(r plc.Reading) {
		fmt.Printf("變化  %s  %-16s %.1f\n", r.MachineID, r.Tag, r.Value)
		if a := monitor.Evaluate(r); a != nil {
			state := "解除"
			if a.Active {
				state = "發生"
			}
			fmt.Printf(">>>>> 警報%s  %s  %s\n", state, a.MachineID, a.Message)
		}
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	belt := make(chan plc.Reading, 64)
	go sim.Run(ctx, belt) // 假機器把資料放上輸送帶
	go hub.Run(ctx, belt) // Hub 從輸送帶拿資料

	// 8 秒後讓 RS-01 故障,看警報
	time.AfterFunc(8*time.Second, func() {
		sim.ForceFault("RS-01")
		fmt.Println("===== 8 秒到,RS-01 故障了 =====")
	})

	<-ctx.Done()
}
