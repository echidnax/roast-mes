package workorder

import (
	"errors"
	"sort"
	"sync"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrDuplicate   = errors.New("order_no already exists")
	ErrConflict    = errors.New("work order was changed by someone else, please reload")
	ErrMachineBusy = errors.New("machine already has a running work order")
)

// Store 是工單檔案櫃(目前放在記憶體,程式關掉就消失)
type Store struct {
	mu     sync.Mutex
	nextID int64
	orders map[int64]WorkOrder
}

func NewStore() *Store {
	return &Store{orders: map[int64]WorkOrder{}}
}

// Create 新增工單,編號重複會報錯,成功會幫工單填上 ID
func (s *Store) Create(w *WorkOrder) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, o := range s.orders {
		if o.OrderNo == w.OrderNo {
			return ErrDuplicate
		}
	}
	s.nextID++
	w.ID = s.nextID
	s.orders[w.ID] = *w
	return nil
}

// Get 取得一張工單的影本
func (s *Store) Get(id int64) (*WorkOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	o, ok := s.orders[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &o, nil
}

// List 列出所有工單,新的在前面
func (s *Store) List() []WorkOrder {
	s.mu.Lock()
	defer s.mu.Unlock()

	list := make([]WorkOrder, 0, len(s.orders))
	for _, o := range s.orders {
		list = append(list, o)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID > list[j].ID })
	return list
}

// Update 存回修改後的工單。
// 版本號對不上,代表你拿到資料之後,別人已經改過了,要擋下來。
func (s *Store) Update(w *WorkOrder) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur, ok := s.orders[w.ID]
	if !ok {
		return ErrNotFound
	}
	if cur.Version != w.Version {
		return ErrConflict
	}
	if w.Status == StatusRunning {
		for _, o := range s.orders {
			if o.ID != w.ID && o.Status == StatusRunning && o.MachineID == w.MachineID {
				return ErrMachineBusy
			}
		}
	}
	w.Version++
	s.orders[w.ID] = *w
	return nil
}
