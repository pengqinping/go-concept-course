//go:build racedemo

package main

import (
	"sync"
	"testing"
)

// TestConcurrentCounterNeedsSync 故意制造一次数据竞态：
// 带 -race 运行时会报告 WARNING: DATA RACE，说明结果正确不等于同步正确。
func TestConcurrentCounterNeedsSync(t *testing.T) {
	counter := 0
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		counter++
	}()
	counter++
	wg.Wait()
}
