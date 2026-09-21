package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	"agent-plugs/sdk"
)

// ===== 网卡速率差分状态（scheduler 单在途保证串行，无锁）=====
type netSnap struct {
	rx, tx uint64 // 累计字节数
	time   time.Time
}

var lastNet = map[string]netSnap{}

// appendNet 从 /proc/net/dev 差分出收发速率（kB/s）。
// 过滤 lo；计数器回绕 / 距上轮超 1 分钟不输出；首轮无基线不输出。
// 多网卡合并为同一指标的多个 sample，条数恒定不随接口数增长。
func appendNet(out []sdk.DeclaredMetric, host string, labels []string) []sdk.DeclaredMetric {
	data, err := os.ReadFile(procPath("/net/dev"))
	if err != nil {
		sdk.Fprintln("[hard] net:", err)
		return out
	}

	now := time.Now()
	prev := lastNet
	lastNet = make(map[string]netSnap, len(prev)+2)

	recvSamples := make([]sdk.Sample, 0, 8)
	sentSamples := make([]sdk.Sample, 0, 8)
	for _, line := range strings.Split(string(data), "\n") {
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		iface := strings.TrimSpace(line[:idx])
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(line[idx+1:])
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64) // receive bytes
		tx, _ := strconv.ParseUint(fields[8], 10, 64) // transmit bytes
		lastNet[iface] = netSnap{rx: rx, tx: tx, time: now}
		if p, ok := prev[iface]; ok && now.Sub(p.time) <= time.Minute && rx >= p.rx && tx >= p.tx {
			dt := now.Sub(p.time).Seconds()
			recvSamples = append(recvSamples, sdk.Sample{Labels: map[string]string{"hostName": host, "iface": iface}, Value: float64(rx-p.rx) / 1024 / dt})
			sentSamples = append(sentSamples, sdk.Sample{Labels: map[string]string{"hostName": host, "iface": iface}, Value: float64(tx-p.tx) / 1024 / dt})
		}
	}
	if len(recvSamples) > 0 {
		out = append(out,
			buildMetricMulti("net_recv_kbps", "网卡接收速率 (kB/s)", netLabels, recvSamples),
			buildMetricMulti("net_sent_kbps", "网卡发送速率 (kB/s)", netLabels, sentSamples),
		)
	}

	// /proc/net/sockstat：sockets: used N / TCP: inuse N ... / UDP: inuse N ...
	if ss, err := os.ReadFile(procPath("/net/sockstat")); err == nil {
		var tcp, udp, socks uint64
		haveTCP, haveUDP, haveSock := false, false, false
		for _, line := range strings.Split(string(ss), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			switch {
			case fields[0] == "sockets:" && fields[1] == "used":
				socks, _ = strconv.ParseUint(fields[2], 10, 64)
				haveSock = true
			case fields[0] == "TCP:" && fields[1] == "inuse":
				tcp, _ = strconv.ParseUint(fields[2], 10, 64)
				haveTCP = true
			case fields[0] == "UDP:" && fields[1] == "inuse":
				udp, _ = strconv.ParseUint(fields[2], 10, 64)
				haveUDP = true
			}
		}
		if haveSock {
			out = append(out, BuildMetric("sockets_inuse", "socket 总数", labels, host, float64(socks)))
		}
		if haveTCP {
			out = append(out, BuildMetric("tcp_inuse", "TCP 连接数", labels, host, float64(tcp)))
		}
		if haveUDP {
			out = append(out, BuildMetric("udp_inuse", "UDP 连接数", labels, host, float64(udp)))
		}
	} else {
		sdk.Fprintln("[hard] sockstat:", err)
	}

	return out
}
