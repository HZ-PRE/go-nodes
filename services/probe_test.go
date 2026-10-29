package services

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"nodes/config"
	"nodes/utils"
)

func TestProbeAllConcurrencyAndCompletion(t *testing.T) {
	old := runtime.GOMAXPROCS(2)
	t.Cleanup(func() { runtime.GOMAXPROCS(old) })
	synctest.Test(t, func(t *testing.T) {
		items := make([]int, 13)
		for i := range items {
			items[i] = i
		}
		var active, peak atomic.Int32
		var results []int
		done := make(chan struct{})
		go func() {
			results = probeAll(items, func(i int) int {
				n := active.Add(1)
				defer active.Add(-1)
				for p := peak.Load(); n > p; p = peak.Load() {
					if peak.CompareAndSwap(p, n) {
						break
					}
				}
				if i%3 == 0 {
					return -1 // 失败任务也必须完成，不占住 worker。
				}
				time.Sleep(time.Second)
				return i * 2
			})
			close(done)
		}()
		synctest.Sleep(10 * time.Second)
		select {
		case <-done:
		default:
			t.Fatal("probe batch did not complete")
		}
		if got := peak.Load(); got != int32(utils.ProbeWorkers()) {
			t.Fatalf("peak concurrency = %d, want %d", got, utils.ProbeWorkers())
		}
		for i, got := range results {
			want := i * 2
			if i%3 == 0 {
				want = -1
			}
			if got != want {
				t.Fatalf("result[%d] = %d, want %d", i, got, want)
			}
		}
	})
}

func TestProbeAllEmpty(t *testing.T) {
	results := probeAll([]int(nil), func(i int) int {
		t.Error("probe called for empty input")
		return i
	})
	if len(results) != 0 {
		t.Fatalf("unexpected results: %v", results)
	}
}

func TestProbesCompleteOnInvalidDNS(t *testing.T) {
	oldConfig, oldErrors := config.AppConfig, errIpMap
	t.Cleanup(func() { config.AppConfig, errIpMap = oldConfig, oldErrors })
	config.AppConfig = &config.Config{Node: config.NodeConfig{URL: []string{"http://unused.invalid"}}}
	errIpMap = make(map[string]uint)
	endpoints := make(map[string]string, utils.ProbeWorkers()+1)
	for i := range utils.ProbeWorkers() + 1 {
		host := fmt.Sprintf("invalid host %d", i) // 空格使解析在本地失败，无需外部 DNS。
		endpoints[host+":80"] = host
	}
	if YCIpBC(endpoints, false) {
		t.Fatal("DNS failures reported as successful probes")
	}
	for endpoint := range endpoints {
		if errIpMap[endpoint] != 1 {
			t.Fatalf("missing failure for %s", endpoint)
		}
	}
}
