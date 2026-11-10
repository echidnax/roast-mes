package plc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

var ErrNotConnected = errors.New("opc ua not connected")

// 每個項目在 OPC UA 伺服器上的名稱
var tagNames = map[Tag]string{
	TagTemperature:    "Temperature",
	TagAirflow:        "Airflow",
	TagCycleState:     "CycleState",
	TagRemainingSec:   "RemainingSec",
	TagSetTemperature: "SetTemperature",
}

// nodeID 組出資料點的地址。RS-01 的豆溫 → ns=2;s=RS01.Temperature
func nodeID(machineID string, tag Tag) string {
	return fmt.Sprintf("ns=2;s=%s.%s", strings.ReplaceAll(machineID, "-", ""), tagNames[tag])
}

type item struct {
	machineID string
	tag       Tag
	nodeID    string
}

// OPCUASource 透過 OPC UA 連到設備,符合 Source 介面,可以取代模擬器
type OPCUASource struct {
	endpoint string
	items    []item // 位置編號就是訂閱時的「代號」

	mu     sync.RWMutex
	client *opcua.Client // 沒連上時是 nil
}

func NewOPCUA(endpoint string, machines []MachineConfig) *OPCUASource {
	s := &OPCUASource{endpoint: endpoint}
	for _, m := range machines {
		for tag := range tagNames {
			s.items = append(s.items, item{machineID: m.ID, tag: tag, nodeID: nodeID(m.ID, tag)})
		}
	}
	return s
}

// Run 連線並接收資料;斷線就等 3 秒再連,直到被叫停
func (s *OPCUASource) Run(ctx context.Context, out chan<- Reading) error {
	for ctx.Err() == nil {
		err := s.session(ctx, out)
		if ctx.Err() != nil {
			return nil
		}
		log.Printf("OPC UA 連線中斷: %v(3 秒後重試)", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(3 * time.Second):
		}
	}
	return nil
}

// session 是「一次連線的完整過程」:連上 → 訂閱 → 一直收資料,出事就回傳錯誤
func (s *OPCUASource) session(ctx context.Context, out chan<- Reading) error {
	c, err := opcua.NewClient(s.endpoint,
		opcua.SecurityPolicy("None"),
		opcua.SecurityMode(ua.MessageSecurityModeNone),
		opcua.AuthAnonymous(),
	)
	if err != nil {
		return fmt.Errorf("建立連線: %w", err)
	}
	if err := c.Connect(ctx); err != nil {
		return fmt.Errorf("連線: %w", err)
	}
	s.setClient(c)
	log.Println("OPC UA 已連線:", s.endpoint)

	// 離開時一定要收尾(用新的 context,因為原本的可能已被取消)
	defer func() {
		s.setClient(nil)
		cctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		c.Close(cctx)
	}()

	// 建立訂閱:伺服器每 500 毫秒檢查一次,有變動就送通知
	notifyCh := make(chan *opcua.PublishNotificationData, 256)
	sub, err := c.Subscribe(ctx, &opcua.SubscriptionParameters{Interval: 500 * time.Millisecond}, notifyCh)
	if err != nil {
		return fmt.Errorf("訂閱: %w", err)
	}
	defer func() {
		cctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		sub.Cancel(cctx)
	}()

	// 告訴伺服器要盯哪些資料點
	reqs := make([]*ua.MonitoredItemCreateRequest, 0, len(s.items))
	for i, it := range s.items {
		id, err := ua.ParseNodeID(it.nodeID)
		if err != nil {
			return fmt.Errorf("地址格式錯誤 %q: %w", it.nodeID, err)
		}
		reqs = append(reqs, opcua.NewMonitoredItemCreateRequestWithDefaults(id, ua.AttributeIDValue, uint32(i)))
	}
	res, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, reqs...)
	if err != nil {
		return fmt.Errorf("監控資料點: %w", err)
	}
	for i, r := range res.Results {
		if r.StatusCode != ua.StatusOK {
			log.Printf("找不到資料點 %s: %v", s.items[i].nodeID, r.StatusCode)
		}
	}

	// 一直等通知 (加入心跳檢查避免斷線卡死)
	heartbeat := time.NewTicker(2 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-heartbeat.C:
			// 透過讀取 ServerStatusCurrentTime 當作 Ping 心跳檢測
			node := c.Node(ua.NewNumericNodeID(0, 2258)) // i=2258: CurrentTime
			hctx, hcancel := context.WithTimeout(ctx, 1*time.Second)
			_, err := node.Value(hctx)
			hcancel()
			if err != nil {
				return fmt.Errorf("心跳檢測失敗，連線已中斷: %w", err)
			}
		case msg, ok := <-notifyCh:
			if !ok {
				return errors.New("通知管道已關閉")
			}
			if msg.Error != nil {
				return fmt.Errorf("收到錯誤: %w", msg.Error)
			}
			change, ok := msg.Value.(*ua.DataChangeNotification)
			if !ok {
				continue
			}
			for _, mi := range change.MonitoredItems {
				if int(mi.ClientHandle) >= len(s.items) || mi.Value == nil || mi.Value.Value == nil {
					continue
				}
				v, ok := toFloat(mi.Value.Value.Value())
				if !ok {
					continue
				}
				it := s.items[mi.ClientHandle]
				select {
				case out <- Reading{MachineID: it.machineID, Tag: it.tag, Value: v, At: time.Now()}:
				case <-ctx.Done():
					return nil
				}
			}
		}
	}
}

func (s *OPCUASource) setClient(c *opcua.Client) {
	s.mu.Lock()
	s.client = c
	s.mu.Unlock()
}

// Write 把一個數值寫進設備的資料點(例如改目標溫度)
func (s *OPCUASource) Write(ctx context.Context, machineID string, tag Tag, value float64) error {
	name, ok := tagNames[tag]
	if !ok || tag != TagSetTemperature {
		return fmt.Errorf("%w: %s/%s", ErrUnknownTarget, machineID, tag)
	}
	_ = name

	s.mu.RLock()
	c := s.client
	s.mu.RUnlock()
	if c == nil {
		return ErrNotConnected
	}

	id, err := ua.ParseNodeID(nodeID(machineID, tag))
	if err != nil {
		return err
	}
	v, err := ua.NewVariant(value)
	if err != nil {
		return err
	}
	resp, err := c.Write(ctx, &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{{
			NodeID:      id,
			AttributeID: ua.AttributeIDValue,
			Value:       &ua.DataValue{EncodingMask: ua.DataValueValue, Value: v},
		}},
	})
	if err != nil {
		return fmt.Errorf("寫入: %w", err)
	}
	if len(resp.Results) == 0 || resp.Results[0] != ua.StatusOK {
		return fmt.Errorf("伺服器拒絕寫入: %v", resp.Results)
	}
	return nil
}

// toFloat 把 OPC UA 各種數字型別統一轉成 float64
func toFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}
