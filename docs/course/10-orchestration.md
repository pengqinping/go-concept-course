# 10 · 谁来等这些 goroutine

> 一句话模型：启动并发只是第一步，编排要回答"谁等谁、失败传给谁、泄漏怎么防"——WaitGroup 管汇合，channel 管接力，泄漏源于没人接收的阻塞。

## 先看一个现象

3 个 worker 各算一个结果，发到同一个 channel，主 goroutine 等它们都结束再取：

```go
results := make(chan int)
var wg sync.WaitGroup
wg.Add(3)
for i := 0; i < 3; i++ {
    go func() {
        defer wg.Done()
        results <- i * i * i // 第一步：交结果
    }()
}
wg.Wait()              // 第二步：等全部结束
fmt.Println(<-results) // 第三步：取结果
```

你以为 `wg.Wait()` 会等 worker 干完活，然后顺利取到结果。实际程序死锁：worker 卡在 `results <- ...` 上，发送没人接收就完不成，完不成就到不了 `defer wg.Done()`，计数器永远不归零，`Wait` 永不返回。"等完成"和"收结果"这两步的先后不是随手写的——每一步单独看都对的代码，顺序错了合起来就是死锁。

## 先建立直觉

:::figure 10-orchestration|fan-out/fan-in 结构与一条泄漏路径
图说明：上半部分看 N 个 worker 怎么从任务 channel 领活、把结果接力进同一个结果 channel，WaitGroup 在旁数人头、归零之后才轮到关闭结果 channel；下半部分是一条泄漏路径——发送方卡在没人接收的 channel 上，永远到不了自己的退出点，旁边是 select + time.After 给出的那条出口。
:::

图上最该带走的一句话：**汇合的完成依赖接力的畅通。** `Done()` 写在发送之后，"人都到齐"的前提是"货都送到了"；只要有一环阻塞且没人来解，整条编排就停在那里。

## 机制拆成四步

### 1. WaitGroup 只回答"这一批都结束了吗"

WaitGroup（完成计数器）只做一件事：`Add(n)` 登记尚未完成的任务数，每个任务结束时 `Done()` 交回一张，`Wait()` 等计数归零。

为什么 `Add` 必须写在 `go` 之前？`Wait` 判断的是"此刻还有多少没完成"；计数若在 goroutine 内部才加，`Wait` 可能在所有 `Add` 之前看到零，直接返回，等待形同虚设。为什么 `Done` 要写成 `defer`？函数无论从哪条路径返回都会交回计数，panic 也不例外；漏交一次，`Wait` 就永远等不齐。

同样要清楚它**不管**什么：不传结果、不传失败、不取消任务。计数归零只代表"都退出了"，不代表"都成功了"。

### 2. 结果走 channel：等数据是另一种"等"

fan-out/fan-in 是最常用的编排形状：主 goroutine 把任务发进 jobs channel，N 个 worker 领活，结果全部汇入同一个 results channel，主 goroutine 统一汇总。

```go
go func() {
    wg.Wait()      // 所有 worker 退出之后
    close(results) // 不会再有人发送，关闭才安全
}()
for r := range results { // range 到 close 为止：这是"等数据"
    sum += r
}
```

为什么关闭要交给一个单独的协调 goroutine？多个 worker 都是发送者，谁都不能自己关——对已关闭的 channel 再 `close` 会直接 panic；而"所有发送者都结束"这个时刻，只有 `wg.Wait()` 知道。于是分工明确：**WaitGroup 数人头，channel 传数据，等齐人头的那一方才有资格关闭。**

### 3. 先等完再收，为什么死锁

回头看开头的现象：`Done` 排在发送之后，`Wait` 等的是 `Done`，而接收又排在 `Wait` 之后——接收永远开始不了，发送永远完不成，等待链绕成一个环。

解开只有两个方向，对应两种正确形态：

- **让发送永不阻塞**：results 的缓冲容量 ≥ 结果总数，worker 放下结果就能走到 `Done`，汇合照常完成，主 goroutine 事后再收。
- **让接收与等待并行**：main 一边 `range results` 一边收，协调 goroutine 负责 `wg.Wait()` 后关闭，收完 range 自然结束。

两种形态的共同点是打破环：**不能让"接收的开始"依赖"汇合的完成"，而汇合又依赖"发送能完成"。**

### 4. 泄漏：没人接收的阻塞

把一个值发给没有接收者的 channel：

```go
orphan := make(chan int)
go func() { orphan <- 99 }() // 没人接收，这条 goroutine 永远停在这里
```

为什么程序不报 deadlock？运行时只在整个程序所有 goroutine 都睡死时才报警；这里 main 还活着、还在往下跑，那条卡住的 goroutine 就被静默遗忘——它的栈、闭包引用的变量全部无法释放。这就是 goroutine 泄漏：**不崩溃，只积累**，在长期运行的进程里慢慢吃掉内存。

防泄漏的办法是给每个阻塞点留出口：`select` 配上 `time.After` 或 `ctx.Done()`，等待就有了"不等了"的分支。启动任何 goroutine 之前先问一句：它的每一步阻塞，谁负责解除？答不上来，就是在攒一个未来的泄漏。

### 5. 失败传给谁

worker 会出错，而错误和结果一样需要明确的接收方。最直接的做法是让结果自带错误字段，随同一个 channel 汇入：

```go
type result struct {
    val int
    err error // worker 把失败交给编排者，而不是自己打印了事
}
```

为什么不让 worker 自己处理失败？重试、跳过还是整体放弃，是"等待的那一方"才能做的决策——它看得见全局进度，单个 worker 看不见。错误塞进结果 channel 之后，编排者收到部分结果加一个错误时，必须当作"未完整完成"处理，不能当成全量成功。

| 你要等的 | 用什么等 |
| --- | --- |
| 一批工作全部退出 | WaitGroup：Add / Done / Wait |
| 数据流自然结束 | range 到 close |
| 不能等太久 | select + time.After / ctx.Done |

## 动手验证

:::demo 10-orchestration|fan-out/fan-in 汇合与一条被保护的发送
先预测：阶段 A 的 results 若改成无缓冲，这个程序还能跑完吗？阶段 B 那个没有接收者的发送，最后打印的是哪一行？先写下答案再揭晓。
验证：`cd examples/minimal/10-orchestration && go run .`。阶段 A 启动 `workerCount` 个 worker 领任务，结果汇入缓冲等于任务数的 results，协调 goroutine 在 `wg.Wait()` 后关闭它，main range 汇总后打印"收到 8 个结果，总和 204"。阶段 B 一个发送无人接收，在 `select` + `time.After` 保护下打印"放弃：无人接收"，随后主 goroutine 正常收尾。把 `workerCount` 改成 1 或 8：分工变了，总和 204 不变——编排改变的是组织方式，不是数据。
:::

## 容易误判的地方

- **"`wg.Wait()` 返回了，结果就都在 channel 里等我取"** → Wait 只保证人都退出了。worker 若卡在发送上，`Done` 执行不到，Wait 根本不返回。先设计接收路径，再谈等待。
- **"每个 worker 干完自己 close 一下 results 更省事"** → 多个发送者各自关闭，第二个 close 直接 panic。关闭权属于"确认所有发送者都结束"的那一方。
- **"goroutine 泄漏会崩给脸色看"** → 恰恰相反。泄漏不报错、不崩溃，只表现为 goroutine 数与内存缓慢上涨；等它显形，往往已经积累了几千条。

## 自己再做一次

1. 把 results 改成无缓冲，同时把"发任务"和"收结果"拆成并行的两个环节：main 用一个 goroutine range 收集汇总，自己只负责发任务。**预期结果**：程序正常打印"总和 204"——接收与发送并行后环就不存在了，缓冲不再是必需品。
2. 删掉阶段 B 的 `select`，让 goroutine 直接 `orphan <- 99`。**预期结果**：程序照常退出、不报任何错，但"放弃：无人接收"再也不会打印——那条 goroutine 被静默丢弃，这正是泄漏平时不被发现的样子。
3. 把 `abandonAfter` 改成 `time.Hour` 再运行。**预期结果**：程序会真的卡住约一小时才打印"放弃"——出口存在，但出口的时刻由参数决定，超时值是编排契约的一部分。

> 下一篇：[内存去哪了](11-memory-escape.md)
