package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Echidnax/roast-mes/internal/workorder"
)

// 程式啟動時會執行這段,資料表不存在就建立(已存在就跳過)
const schema = `
CREATE TABLE IF NOT EXISTS work_orders (
    id           BIGSERIAL PRIMARY KEY,
    order_no     TEXT        NOT NULL UNIQUE,
    product_spec TEXT        NOT NULL,
    planned_qty  INT         NOT NULL CHECK (planned_qty > 0),
    produced_qty INT         NOT NULL DEFAULT 0 CHECK (produced_qty >= 0),
    machine_id   TEXT,
    status       TEXT        NOT NULL,
    version      INT         NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL
);

-- 同一台機器同時只能有一張「生產中」的工單,由資料庫自己把關
CREATE UNIQUE INDEX IF NOT EXISTS one_running_per_machine
    ON work_orders (machine_id) WHERE status = 'running';
`

type Store struct {
	pool *pgxpool.Pool
}

var _ workorder.Repository = (*Store)(nil)

// New 連上資料庫、確認連得通、建立資料表
func New(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// 每次查詢最多等 5 秒,避免資料庫卡住時整個程式跟著卡死
func timeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

const columns = `id, order_no, product_spec, planned_qty, produced_qty,
	COALESCE(machine_id, ''), status, version, created_at, updated_at`

type scanner interface{ Scan(dest ...any) error }

func scan(row scanner) (*workorder.WorkOrder, error) {
	var w workorder.WorkOrder
	var status string
	err := row.Scan(&w.ID, &w.OrderNo, &w.ProductSpec, &w.PlannedQty, &w.ProducedQty,
		&w.MachineID, &status, &w.Version, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	w.Status = workorder.Status(status)
	return &w, nil
}

// mapErr 把資料庫的「重複」錯誤,翻成我們自己定義的錯誤
func mapErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // 23505 = 違反唯一限制
		switch pgErr.ConstraintName {
		case "one_running_per_machine":
			return workorder.ErrMachineBusy
		case "work_orders_order_no_key":
			return workorder.ErrDuplicate
		}
	}
	return err
}

func (s *Store) Create(w *workorder.WorkOrder) error {
	ctx, cancel := timeout()
	defer cancel()

	err := s.pool.QueryRow(ctx, `
		INSERT INTO work_orders
			(order_no, product_spec, planned_qty, produced_qty, machine_id, status, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, $9)
		RETURNING id`,
		w.OrderNo, w.ProductSpec, w.PlannedQty, w.ProducedQty, w.MachineID,
		string(w.Status), w.Version, w.CreatedAt, w.UpdatedAt,
	).Scan(&w.ID)
	return mapErr(err)
}

func (s *Store) Get(id int64) (*workorder.WorkOrder, error) {
	ctx, cancel := timeout()
	defer cancel()

	w, err := scan(s.pool.QueryRow(ctx, `SELECT `+columns+` FROM work_orders WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, workorder.ErrNotFound
	}
	return w, err
}

func (s *Store) List() ([]workorder.WorkOrder, error) {
	ctx, cancel := timeout()
	defer cancel()

	rows, err := s.pool.Query(ctx, `SELECT `+columns+` FROM work_orders ORDER BY id DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []workorder.WorkOrder{}
	for rows.Next() {
		w, err := scan(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *w)
	}
	return list, rows.Err()
}

// Update 只有在「版本號還對得上」時才會修改,對不上就代表別人先改過了
func (s *Store) Update(w *workorder.WorkOrder) error {
	ctx, cancel := timeout()
	defer cancel()

	tag, err := s.pool.Exec(ctx, `
		UPDATE work_orders
		SET produced_qty = $1, machine_id = NULLIF($2, ''), status = $3,
		    version = version + 1, updated_at = $4
		WHERE id = $5 AND version = $6`,
		w.ProducedQty, w.MachineID, string(w.Status), w.UpdatedAt, w.ID, w.Version)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM work_orders WHERE id = $1)`, w.ID).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return workorder.ErrNotFound
		}
		return workorder.ErrConflict
	}
	w.Version++
	return nil
}
