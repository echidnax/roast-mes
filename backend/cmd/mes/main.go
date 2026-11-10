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

	"github.com/Echidnax/roast-mes/internal/pgstore"
	"github.com/Echidnax/roast-mes/internal/plc"
	"github.com/Echidnax/roast-mes/internal/workorder"
)

type machineView struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Online    bool                `json:"online"`
	Values    map[plc.Tag]float64 `json:"values"`
	UpdatedAt *time.Time          `json:"updated_at"`
}

func main() {
	machines := []plc.MachineConfig{
		{ID: "RS-01", Name: "烘豆機 01"},
		{ID: "RS-02", Name: "烘豆機 02"},
		{ID: "RS-03", Name: "烘豆機 03"},
	}

	// 設備來源:預設用內建模擬器,PLC_MODE=opcua 就改連 OPC UA 伺服器
	var src plc.Source
	if os.Getenv("PLC_MODE") == "opcua" {
		endpoint := os.Getenv("OPCUA_ENDPOINT")
		if endpoint == "" {
			endpoint = "opc.tcp://localhost:4840/UA/RoastMES"
		}
		src = plc.NewOPCUA(endpoint, machines)
		log.Println("設備來源: OPC UA", endpoint)
	} else {
		src = plc.NewSim(machines)
		log.Println("設備來源: 內建模擬器")
	}
	hub := plc.NewHub(nil)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	belt := make(chan plc.Reading, 256)
	go src.Run(ctx, belt)
	go hub.Run(ctx, belt)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/machines", func(w http.ResponseWriter, r *http.Request) {
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
				v.Online = time.Since(t) < 10*time.Second
			}
			list = append(list, v)
		}
		writeJSON(w, http.StatusOK, list)
	})

	// 設定目標溫度:寫回設備
	mux.HandleFunc("PUT /api/machines/{id}/set-temperature", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		found := false
		for _, m := range machines {
			if m.ID == id {
				found = true
			}
		}
		if !found {
			writeError(w, workorder.ErrNotFound)
			return
		}
		var body struct {
			Value float64 `json:"value"`
		}
		if !readBody(w, r, &body) {
			return
		}
		if body.Value < 150 || body.Value > 250 {
			writeError(w, errors.Join(workorder.ErrValidation, errors.New("目標溫度必須在 150 到 250 之間")))
			return
		}
		if err := src.Write(r.Context(), id, plc.TagSetTemperature, body.Value); err != nil {
			if errors.Is(err, plc.ErrNotConnected) {
				writeJSON(w, http.StatusServiceUnavailable, map[string]any{
					"error": map[string]string{"code": "plc_offline", "message": "設備目前沒有連線"},
				})
				return
			}
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]float64{"value": body.Value})
	})

	// 有設定 DATABASE_URL 就用 PostgreSQL,沒設定就用記憶體
	var repo workorder.Repository = workorder.NewStore()
	if url := os.Getenv("DATABASE_URL"); url != "" {
		pg, err := pgstore.New(ctx, url)
		if err != nil {
			log.Fatal(err)
		}
		defer pg.Close()
		repo = pg
		log.Println("工單儲存: PostgreSQL")
	} else {
		log.Println("工單儲存: 記憶體(關掉就消失)")
	}
	registerWorkOrderRoutes(mux, repo, machines)

	srv := &http.Server{Addr: ":8080", Handler: mux}
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

// 確保 json 套件有被用到(writeJSON 在 workorders.go 裡)
var _ = json.Marshal
