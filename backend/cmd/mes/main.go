package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/Echidnax/roast-mes/internal/plc"
)

// machineView 是「網頁看到的一台機器」長什麼樣子
type machineView struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Online    bool                `json:"online"`
	Values    map[plc.Tag]float64 `json:"values"`
	UpdatedAt *time.Time          `json:"updated_at"`
}

func main() {
	// 三台假烘豆機
	machines := []plc.MachineConfig{
		{ID: "RS-01", Name: "烘豆機 01"},
		{ID: "RS-02", Name: "烘豆機 02"},
		{ID: "RS-03", Name: "烘豆機 03"},
	}
	sim := plc.NewSim(machines)
	hub := plc.NewHub(nil)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	belt := make(chan plc.Reading, 256)
	go sim.Run(ctx, belt) // 假機器放資料上輸送帶
	go hub.Run(ctx, belt) // Hub 從輸送帶拿資料,記成最新狀態

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/machines", func(w http.ResponseWriter, r *http.Request) {
		// 先把 Hub 的卡片整理成「機台 ID → 卡片」,方便查
		cards := map[string]plc.Snapshot{}
		for _, s := range hub.Snapshots() {
			cards[s.MachineID] = s
		}

		list := make([]machineView, 0, len(machines))
		for _, m := range machines {
			v := machineView{ID: m.ID, Name: m.Name, Values: map[plc.Tag]float64{}}
			if card, ok := cards[m.ID]; ok {
				t := card.UpdatedAt
				v.Values = card.Values
				v.UpdatedAt = &t
				v.Online = time.Since(t) < 10*time.Second // 10 秒內有收到資料才算在線
			}
			list = append(list, v)
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(list)
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}

	// 按 Ctrl+C 時,讓伺服器把手上的事做完再關
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Println("listening on :8080")
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
