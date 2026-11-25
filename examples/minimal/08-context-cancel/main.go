package main

import (
	"context"
	"fmt"
	"time"
)

const (
	workTicks = 3                     // 可调参数：手动取消前处理的任务数
	timeout   = 30 * time.Millisecond // 可调参数：WithTimeout 的超时时长，改大只改变等待时长
)

func worker(name string, ctx context.Context, jobs <-chan int, done chan<- string) {
	for {
		select {
		case n := <-jobs:
			fmt.Printf("[%s] 处理任务 %d\n", name, n)
		case <-ctx.Done():
			done <- fmt.Sprintf("[%s] 退出，原因：%v", name, ctx.Err())
			return
		}
	}
}

func main() {
	fmt.Println("[阶段A] 手动取消父 ctx，worker 监听子 ctx")

	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	child, cancelChild := context.WithCancel(parent)
	defer cancelChild()

	jobs := make(chan int)
	done := make(chan string, 1)
	go worker("worker-A", child, jobs, done)

	for i := 1; i <= workTicks; i++ {
		jobs <- i // 同步交接：send 返回时 worker 已经接走任务
	}
	cancelParent() // 父取消，子 ctx 的 Done 一并关闭
	fmt.Println(<-done)

	fmt.Println("[阶段B] WithTimeout 自动触发，无人调用 cancel")

	tctx, cancelTimeout := context.WithTimeout(context.Background(), timeout)
	defer cancelTimeout()
	jobsB := make(chan int)
	doneB := make(chan string, 1)
	go worker("worker-B", tctx, jobsB, doneB)

	fmt.Println(<-doneB) // 没有任务，worker 等到超时后从 Done 退出
}
