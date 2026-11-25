# 08 · 取消是通知，不是命令

> 一句话模型：Context 只关闭一条叫 Done 的通道来广播"请停止"，真正停下必须靠被调代码自己在 select 或检查点响应。

## 先看一个现象

调用方准备好了取消，worker 却收不到：

```go
ctx, cancel := context.WithCancel(context.Background())
go func() {
    for {
        work() // 循环里没有任何地方读 ctx
    }
}()
cancel() // 只关闭 ctx.Done()，不会中断 work()
```

你以为 `cancel()` 一调用，那个 goroutine 就被"叫停"了。实际它继续跑：循环里没有 `ctx` 的任何位置，信号发出去无人接收。

再看一个看似正确的 `select`：

```go
select {
case <-ctx.Done():
    return ctx.Err()
case result := <-results:
    return result, nil
}
```

你以为"取消优先"，只要 `Done` 关闭就立刻退出。实际当 `results` 里也有值、两个 case 同时就绪时，`select` 随机挑一个——取消不是高优先级命令，只是"又一条就绪的路"。

两段代码合起来说明一件事：Context 不拥有停止 goroutine 的权力。它只做一件很小的事——关闭 `Done()` 通道。剩下的，全靠被调代码配合。

## 先建立直觉

:::figure 08-context-cancel|取消沿 context 树向下传播
图说明：主 goroutine 调 `cancel()`，父 ctx 的 `Done` 被关闭；由它派生的每个子 ctx 的 `Done` 随之关闭；下游每个 worker 都在自己的 `select` 里看到 `Done` 就绪，才主动 `return ctx.Err()`。整条链路是"关闭 → 广播 → 各自决定停下"，没有哪一环能强杀 goroutine。
:::

图上最关键的一点：**关闭只发生在父节点，观察发生在每个 worker 自己身上。** 取消传的是信号，不是执行力。

## 机制拆成四步

### 1. Context 是一棵可取消的树

`context.WithCancel(parent)` 从父 ctx 派生子 ctx，并返回一个 `cancel` 函数；`WithTimeout`、`WithDeadline` 也一样，只是额外挂了一个定时器。每个子 ctx 记住自己的父节点，父节点取消时把信号沿树往下传。

为什么需要树？一个请求常常扇出多路工作：查缓存、调下游、写日志。你想让"客户端断开"这一件事同时停掉所有分支，树结构让你只取消根节点，所有分支的 `Done` 都会关闭。

```go
parent, cancelParent := context.WithCancel(context.Background())
child, cancelChild := context.WithTimeout(parent, 2*time.Second)
defer cancelParent()
defer cancelChild()
```

注意取消不反向传播：`cancelChild()` 只关闭 child 的 `Done`，parent 和兄弟节点毫无感觉。子节点也不能把截止时间"延长"：`WithTimeout(parent, 2s)` 在 parent 只剩 200ms 时，实际 200ms 就到头了。

### 2. Done() 是一条"只关闭、不发送"的通道

`ctx.Done()` 返回 `<-chan struct{}`。取消时运行时做的事是 `close(done)`，不是 `done <- struct{}{}`。

为什么用关闭？因为关闭天然是对所有接收者的广播：任何正在 `<-ctx.Done()` 等待的 goroutine 都立刻得到零值并往下走；已经关闭的通道在 `select` 里永远就绪。若改成发送一个值，你还得先知道有多少个监听者，而且没人接听时发送会阻塞——那就不叫广播了。

配套方法各回答一个问题：

- `Done()`：该停了吗（通道关闭即"是"）。
- `Err()`：为什么停（`context.Canceled` 或 `context.DeadlineExceeded`）。
- `Deadline()` / `Value()`：截止时间与请求元数据（本篇不展开）。

`Err()` 在取消之后稳定返回同一个错误，重复调用结果一致，所以判断时直接用 `errors.Is(err, context.Canceled)` 即可。

### 3. 停下是被调代码的协作义务

Go 没有"杀掉一个 goroutine"的 API。为什么？强制终止会跳过 `defer`、留下没释放的锁和写了一半的状态，比让任务自己收尾危险得多。所以 Go 选择协作式取消：通知发出后，被调代码在合适的点自己退出。

worker 的标准形状，是在阻塞点把 `Done` 放进 `select`：

```go
select {
case n := <-jobs:
    handle(n)
case <-ctx.Done():
    return ctx.Err()
}
```

纯计算循环没有阻塞点，就自己加检查点：

```go
for i, v := range values {
    if i%1000 == 0 {
        if err := ctx.Err(); err != nil {
            return err
        }
    }
    process(v)
}
```

为什么不是每次都查 `Err()`？检查本身有成本，分段检查是"响应及时"和"开销可控"之间的折中。关键是别让一段不可中断的工作长到用户已经走了你还在算。

### 4. WithTimeout 只是"定时器到点自动调用 cancel"

`context.WithTimeout(parent, d)` 相当于在 `d` 之后自动调用 `cancel`，所以 worker 那边完全不用改：还是监听同一个 `Done`。区别只在 `Err()`：手动 `cancel()` 得到 `context.Canceled`，超时自动触发得到 `context.DeadlineExceeded`。

```go
ctx, cancel := context.WithTimeout(context.Background(), timeout)
defer cancel() // 提前完成时释放定时器与父子关联，必须写
```

为什么提前成功还要 `defer cancel()`？因为定时器还在跑、子 ctx 还挂在父节点的表里；不调用 `cancel()`，这些资源要等超时那一刻才释放。短请求反复这么做，累积的就是实实在在的泄漏。

最后一条边界：取消不保证优先。如果 `Done` 和数据同时就绪，`select` 可能先处理数据。业务上要求"取消后不再处理新任务"，就在使用数据前再查一次 `ctx.Err()`——判断发生在哪里，优先权就在哪里。

## 动手验证

:::demo 08-context-cancel|取消一次，看 worker 怎么退出
先预测：主 goroutine 调 `cancel()` 之后，worker 会立刻停止吗？阶段 B 没有任何人调用 `cancel()`，worker 又为什么会退出？
验证：`cd examples/minimal/08-context-cancel && go run .`。阶段 A 先同步处理 3 个任务，再取消父 ctx；worker 挂在子 ctx 上，仍从 `Done` 退出，打印 `context canceled`。阶段 B 用 `WithTimeout`，worker 一直空等，超时后自动触发取消，打印 `context deadline exceeded`。Demo 不打印真实耗时，只打印阶段标签与退出原因。
:::

输出里"处理任务"的行数等于 `workTicks`，把常量改大就多打印几行，但退出原因不变。这再次说明：停下的决定权在 worker 的 `select` 里，不在 `cancel()` 里。

## 容易误判的地方

- **`cancel()` 会终止 goroutine** → 它只关闭 `Done`。worker 没有监听点就继续跑；`context` 包里没有任何"强杀" API。
- **取消 case 比数据 case 优先** → `select` 同时就绪时随机挑一个。要求"取消后不开始新工作"，必须在使用数据前显式检查 `ctx.Err()`。
- **`WithTimeout` 之后可以省掉 `defer cancel()`** → 不行。提前成功时定时器仍在跑、父子关联仍在，必须调用 `cancel()` 释放。

## 自己再做一次

1. 把 demo 的 `workTicks` 改成 0。**预期结果**：阶段 A 不打印任何"处理任务"行，直接打印 `worker-A 退出，原因：context canceled`——取消发生在 worker 处理第一个任务之前。
2. 在阶段 A 把 `cancelParent()` 换成 `cancelChild()`。**预期结果**：worker 仍然退出（它挂在子 ctx 上），但父 ctx 的 `Done` 没关闭；这说明取消沿树向下，不向上。
3. 在 worker 的处理分支前加一句 `if err := ctx.Err(); err != nil { ... }`，并在发任务前先调用 `cancelParent()`。**预期结果**：任务被跳过（可能只有部分被处理）；证明"取消不优先"要靠显式检查来补。

> 下一篇：[共享状态怎么才安全](09-shared-state.md)
