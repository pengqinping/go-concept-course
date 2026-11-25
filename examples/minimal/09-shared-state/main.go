package main

import (
	"fmt"
	"sync"
)

const (
	goroutineCount = 8    // 可调参数：并发 goroutine 数
	increments     = 1000 // 可调参数：每个 goroutine 的累加次数
)

func addUnlocked(counter *int, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < increments; i++ {
		*counter = *counter + 1 // 读-改-写 unprotected：结果不可依赖
	}
}

func addLocked(counter *int, mu *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < increments; i++ {
		mu.Lock()
		*counter = *counter + 1 // 整段读-改-写都在临界区内
		mu.Unlock()
	}
}

func main() {
	fmt.Printf("[阶段A] 无锁累加：%d 个 goroutine × %d 次\n", goroutineCount, increments)
	counter := 0
	var wg sync.WaitGroup
	wg.Add(goroutineCount)
	for i := 0; i < goroutineCount; i++ {
		go addUnlocked(&counter, &wg)
	}
	wg.Wait()
	fmt.Println("[阶段A] 结果不确定，因此不打印数值；用 go test -race 复核数据竞态")

	fmt.Println("[阶段B] Mutex 保护临界区：同样次数")
	counter = 0
	var wg2 sync.WaitGroup
	var mu sync.Mutex
	wg2.Add(goroutineCount)
	for i := 0; i < goroutineCount; i++ {
		go addLocked(&counter, &mu, &wg2)
	}
	wg2.Wait()
	want := goroutineCount * increments
	fmt.Printf("[阶段B] 期望 %d，实际 %d，一致：%v\n", want, counter, counter == want)
}
