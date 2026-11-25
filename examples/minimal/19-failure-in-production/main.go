package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// 可调参数：前 N 次下游调用故意失败（用计数器，不用随机数）
const failFirst = 2

// 可调参数：单次尝试的超时（毫秒）
const timeoutMS = 30

type downstream struct {
	attempts int
	seen     map[string]string // 幂等键 -> 第一次的结果
}

func newDownstream() *downstream {
	return &downstream{seen: make(map[string]string)}
}

// charge 模拟一次有副作用的下游调用。
func (d *downstream) charge(ctx context.Context, key, amount string) (string, error) {
	if res, ok := d.seen[key]; ok {
		return "replay:" + res, nil // 命中幂等键，不消耗失败预算
	}
	d.attempts++
	if d.attempts <= failFirst {
		return "", fmt.Errorf("transient failure #%d", d.attempts)
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	d.seen[key] = amount
	return "charged:" + amount, nil
}

// slow 模拟迟迟不返回的下游，用来展示超时只中断本地等待。
func (d *downstream) slow(ctx context.Context) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func callWithRetry(ctx context.Context, d *downstream, key, amount string, maxAttempts int) (string, int, error) {
	retries := 0
	for i := 1; i <= maxAttempts; i++ {
		attemptCtx, cancel := context.WithTimeout(ctx, timeoutMS*time.Millisecond)
		result, err := d.charge(attemptCtx, key, amount)
		cancel()
		if err == nil {
			return result, retries, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", retries, err // 预算已耗尽，不再重试
		}
		if i < maxAttempts {
			retries++
			fmt.Printf("  第 %d 次尝试失败：%v → 重试（幂等键=%s）\n", i, err, key)
		}
	}
	return "", retries, fmt.Errorf("all %d attempts failed", maxAttempts)
}

func main() {
	d := newDownstream()

	fmt.Println("场景一：前两次失败，第三次成功")
	res, retries, err := callWithRetry(context.Background(), d, "k1", "100", 3)
	fmt.Printf("  最终：result=%s retries=%d err=%v 下游调用=%d\n", res, retries, err, d.attempts)

	replay, err := d.charge(context.Background(), "k1", "100")
	fmt.Printf("  同键重放：result=%s err=%v 下游调用=%d（未增加）\n", replay, err, d.attempts)

	fmt.Println("场景二：单次尝试超时")
	slowCtx, cancelSlow := context.WithTimeout(context.Background(), timeoutMS*time.Millisecond)
	_, err = d.slow(slowCtx)
	cancelSlow()
	fmt.Printf("  结果：%v（本地没等到 ≠ 远端没做事）\n", err)

	fmt.Println("场景三：优雅停机顺序")
	root, stop := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-root.Done()
		fmt.Println("  [停机] 收到取消通知：停止接收新请求")
	}()
	inflight, inflightRetries, inflightErr := callWithRetry(root, d, "k3", "25", 3)
	fmt.Printf("  [在途] 结果=%s retries=%d err=%v\n", inflight, inflightRetries, inflightErr)
	stop()
	<-stopped
	fmt.Println("  [停机] 在途工作已收尾，进程退出")
}
