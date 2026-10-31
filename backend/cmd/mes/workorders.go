package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Echidnax/roast-mes/internal/plc"
	"github.com/Echidnax/roast-mes/internal/workorder"
)

func registerWorkOrderRoutes(mux *http.ServeMux, store workorder.Repository, machines []plc.MachineConfig) {
	hasMachine := func(id string) bool {
		for _, m := range machines {
			if m.ID == id {
				return true
			}
		}
		return false
	}

	// 取出網址裡的工單編號,並從檔案櫃找出那張工單
	load := func(w http.ResponseWriter, r *http.Request) (*workorder.WorkOrder, bool) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, workorder.ErrNotFound)
			return nil, false
		}
		wo, err := store.Get(id)
		if err != nil {
			writeError(w, err)
			return nil, false
		}
		return wo, true
	}

	// 列出所有工單
	mux.HandleFunc("GET /api/work-orders", func(w http.ResponseWriter, r *http.Request) {
		list, err := store.List()
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})

	// 新增工單
	mux.HandleFunc("POST /api/work-orders", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OrderNo     string `json:"order_no"`
			ProductSpec string `json:"product_spec"`
			PlannedQty  int    `json:"planned_qty"`
		}
		if !readBody(w, r, &body) {
			return
		}
		wo, err := workorder.New(body.OrderNo, body.ProductSpec, body.PlannedQty, time.Now())
		if err == nil {
			err = store.Create(wo)
		}
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, wo)
	})

	// 看單張工單
	mux.HandleFunc("GET /api/work-orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		if wo, ok := load(w, r); ok {
			writeJSON(w, http.StatusOK, wo)
		}
	})

	// 改狀態:派工、開工、完工、取消
	mux.HandleFunc("POST /api/work-orders/{id}/transition", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			To        workorder.Status `json:"to"`
			MachineID string           `json:"machine_id"`
			Version   int              `json:"version"`
		}
		if !readBody(w, r, &body) {
			return
		}
		wo, ok := load(w, r)
		if !ok {
			return
		}
		if body.Version != wo.Version {
			writeError(w, workorder.ErrConflict)
			return
		}
		if body.MachineID != "" && !hasMachine(body.MachineID) {
			writeError(w, errors.Join(workorder.ErrValidation, errors.New("unknown machine "+body.MachineID)))
			return
		}
		if err := wo.Transition(body.To, body.MachineID, time.Now()); err != nil {
			writeError(w, err)
			return
		}
		if err := store.Update(wo); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, wo)
	})

	// 回報產量
	mux.HandleFunc("POST /api/work-orders/{id}/report", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Qty     int `json:"qty"`
			Version int `json:"version"`
		}
		if !readBody(w, r, &body) {
			return
		}
		wo, ok := load(w, r)
		if !ok {
			return
		}
		if body.Version != wo.Version {
			writeError(w, workorder.ErrConflict)
			return
		}
		if err := wo.Report(body.Qty, time.Now()); err != nil {
			writeError(w, err)
			return
		}
		if err := store.Update(wo); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, wo)
	})
}

// ---- 小工具 ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// readBody 把請求內容讀進 dst,格式不對就回報 400
func readBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, errors.Join(workorder.ErrValidation, errors.New("invalid JSON body")))
		return false
	}
	return true
}

// writeError 依錯誤種類,決定要回哪個 HTTP 狀態碼
func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal"
	switch {
	case errors.Is(err, workorder.ErrValidation):
		status, code = http.StatusBadRequest, "validation"
	case errors.Is(err, workorder.ErrInvalidTransition):
		status, code = http.StatusConflict, "invalid_transition"
	case errors.Is(err, workorder.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, workorder.ErrConflict):
		status, code = http.StatusConflict, "version_conflict"
	case errors.Is(err, workorder.ErrDuplicate):
		status, code = http.StatusConflict, "duplicate"
	case errors.Is(err, workorder.ErrMachineBusy):
		status, code = http.StatusConflict, "machine_busy"
	}
	msg := err.Error()
	if status == http.StatusInternalServerError {
		msg = "系統發生錯誤,請稍後再試" // 不把內部細節給外面看
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}
