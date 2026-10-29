package workorder

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	StatusCreated   Status = "created"
	StatusReleased  Status = "released"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusCancelled Status = "cancelled"
)

var (
	ErrValidation        = errors.New("validation failed")
	ErrInvalidTransition = errors.New("invalid status transition")
)
var allowed = map[Status][]Status{
	StatusCreated:  {StatusReleased, StatusCancelled},
	StatusReleased: {StatusRunning, StatusCancelled},
	StatusRunning:  {StatusCompleted, StatusCancelled},
}

type WorkOrder struct {
	ID          int64     `json:"id"`
	OrderNo     string    `json:"order_no"`
	ProductSpec string    `json:"product_spec"`
	PlannedQty  int       `json:"planned_qty"`
	ProducedQty int       `json:"produced_qty"`
	MachineID   string    `json:"machine_id"`
	Status      Status    `json:"status"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func New(orderNo, productSpec string, plannedQty int, now time.Time) (*WorkOrder, error) {
	orderNo = strings.TrimSpace(orderNo)
	productSpec = strings.TrimSpace(productSpec)

	if orderNo == "" {
		return nil, fmt.Errorf("%w: order_no is required", ErrValidation)
	}
	if productSpec == "" {
		return nil, fmt.Errorf("%w: product_spec is required", ErrValidation)
	}
	if plannedQty <= 0 {
		return nil, fmt.Errorf("%w: planned_qty must be greater than 0", ErrValidation)
	}

	return &WorkOrder{
		OrderNo:     orderNo,
		ProductSpec: productSpec,
		PlannedQty:  plannedQty,
		Status:      StatusCreated,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (w *WorkOrder) CanTransition(to Status) bool {
	for _, s := range allowed[w.Status] {
		if s == to {
			return true
		}
	}
	return false
}

func (w *WorkOrder) Transition(to Status, machineID string, now time.Time) error {
	if !w.CanTransition(to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, w.Status, to)
	}

	// 先算出最後會用哪台機台
	machine := w.MachineID
	if machineID != "" {
		machine = machineID
	}

	switch to {
	case StatusReleased, StatusRunning:
		if machine == "" {
			return fmt.Errorf("%w: machine_id is required", ErrValidation)
		}
	case StatusCompleted:
		if w.ProducedQty < w.PlannedQty {
			return fmt.Errorf("%w: produced_qty (%d) is below planned_qty (%d)",
				ErrValidation, w.ProducedQty, w.PlannedQty)
		}
	}

	// 全部驗證都通過了,才真正修改
	w.MachineID = machine
	w.Status = to
	w.UpdatedAt = now
	return nil
}

// Report回報產量。只有生產中才能回報，而且累計不能超過計畫數量。
func (w *WorkOrder) Report(qty int, now time.Time) error {
	if w.Status != StatusRunning {
		return fmt.Errorf("%w: can only report while running (now %s)", ErrInvalidTransition, w.Status)
	}
	if qty <= 0 {
		return fmt.Errorf("%w: qty must be greater than 0", ErrValidation)
	}
	if w.ProducedQty+qty > w.PlannedQty {
		return fmt.Errorf("%w: total would exceed planned_qty (%d)", ErrValidation, w.PlannedQty)
	}

	w.ProducedQty += qty
	w.UpdatedAt = now
	return nil
}
