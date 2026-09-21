package api

import (
	"encoding/json"
	"time"

	"agent/config"
	"agent/modle"
)

const heartbeatInterval = 15 * time.Second

// Heartbeat 心跳采集：固定 15s 周期，壳直接采 is_active=1。
// 不经过插件。
type Heartbeat struct {
	cfg  *config.ConfigCenter
	xfer *Transport
}

// NewHeartbeat 构造心跳。
func NewHeartbeat(cfg *config.ConfigCenter, xfer *Transport) *Heartbeat {
	return &Heartbeat{cfg: cfg, xfer: xfer}
}

// Run 阻塞直到 ctx 取消。
func (h *Heartbeat) Run(ctx config.Done) {
	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()

	// 启动立即打一次
	h.beat()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.beat()
		}
	}
}

func (h *Heartbeat) beat() {
	dm := []modle.DeclaredMetric{
		{
			Name:   "is_active",
			Help:   "agent shell heartbeat: 1 means alive",
			Type:   modle.MetricTypeGauge,
			Labels: []string{},
			Samples: []modle.Sample{
				{Labels: map[string]string{}, Value: 1},
			},
		},
	}
	h.xfer.OnCollect("__heartbeat__", marshalQuiet(dm))
}

// marshalQuiet 工具函数，忽略 json.Marshal error（壳不会构造出非法结构）。
func marshalQuiet(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
