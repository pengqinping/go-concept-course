package main

import (
	"fmt"
	"time"
)

const (
	waitLimit   = 50 * time.Millisecond // 可调参数：等待发送就绪的上限，改大只改变等待时长
	bufferedCap = 1                     // 可调参数：缓冲格数，改成 0 会复现无缓冲的阻塞
)

func main() {
	fmt.Println("[阶段A] 无缓冲 channel：先发，再收")

	unbuffered := make(chan int)
	sent := make(chan struct{})
	go func() {
		unbuffered <- 7
		close(sent)
	}()

	// 没有接收方，发送永远不就绪；超时分支给出等待上界，避免死锁。
	select {
	case <-sent:
		fmt.Println("[阶段A] 发送已就绪（按本 demo 的结构不会发生）")
	case <-time.After(waitLimit):
		fmt.Println("[阶段A] 发送未就绪 → 超时退出，不是死锁")
	}

	// 补上接收，让阻塞的发送方完成，goroutine 正常退出。
	fmt.Printf("[阶段A] 补一个接收，收到 %d\n", <-unbuffered)

	fmt.Println("[阶段B] 缓冲 channel：先发，再收")
	buffered := make(chan int, bufferedCap)
	buffered <- 7
	fmt.Printf("[阶段B] 发送完成，缓冲区长度 %d\n", len(buffered))
	fmt.Printf("[阶段B] 收到 %d，缓冲区长度 %d\n", <-buffered, len(buffered))
}
