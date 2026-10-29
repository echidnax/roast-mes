package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/Echidnax/roast-mes/internal/plc"
)

func main() {
	machines := []plc.MachineConfig{
		{ID: "RS-01", Name: "烘豆機 01"},
		{ID: "RS-02", Name: "烘豆機 02"},
	}
	src := plc.NewSim(machines)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	readings := make(chan plc.Reading, 64)
	go src.Run(ctx, readings)

	for {
		select {
		case <-ctx.Done():
			return
		case r := <-readings:
			fmt.Printf("%s  %-16s %.1f\n", r.MachineID, r.Tag, r.Value)
		}
	}
}
