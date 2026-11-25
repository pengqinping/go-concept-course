package main

import (
	"fmt"
	"sync"
	"time"
)

const (
	workerCount  = 3                     // 可调参数：worker 数量，改成 1 或 8，汇总结果应当不变
	abandonAfter = 50 * time.Millisecond // 可调参数：无人接收时等多久放弃
)

func main() {
	fmt.Println("[阶段A] fan-out / fan-in：N 个 worker，一个结果 channel")

	const jobCount = 8
	jobs := make(chan int)
	results := make(chan int, jobCount) // 缓冲 = 结果总数：worker 发送不阻塞，能走到 Done

	var wg sync.WaitGroup
	wg.Add(workerCount) // Add 在 go 之前：等待方要先看到未完成计数
	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done() // defer：无论哪条路径返回都交回计数
			for job := range jobs {
				results <- job * job // 接力：算完就交给结果 channel
			}
		}()
	}

	// 关闭责任：所有发送者都结束之后才轮到 close，这里交给协调 goroutine。
	go func() {
		wg.Wait()
		close(results)
	}()

	for job := 1; job <= jobCount; job++ {
		jobs <- job
	}
	close(jobs) // 任务发完，worker 的 range 才会结束

	sum, count := 0, 0
	for r := range results { // 等数据的另一种形式：range 到 close 为止
		sum += r
		count++
	}
	fmt.Printf("[阶段A] %d 个 worker 汇合完成，收到 %d 个结果，总和 %d\n", workerCount, count, sum)

	fmt.Println("[阶段B] 没有接收者的发送：给等待留一个出口")
	orphan := make(chan int)
	go func() {
		select {
		case orphan <- 99: // 没有接收方，这个分支永远不会就绪
			fmt.Println("[阶段B] 发送成功（按本 demo 的结构不会发生）")
		case <-time.After(abandonAfter):
			fmt.Println("[阶段B] 放弃：无人接收")
		}
	}()
	time.Sleep(2 * abandonAfter) // 故意不接收 orphan，观察超时分支
	fmt.Println("[阶段B] 主 goroutine 正常收尾，阻塞的发送没有拖垮程序")
}
