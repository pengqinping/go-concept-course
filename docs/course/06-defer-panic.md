# 06 · defer 到底什么时候执行

> 一句话模型：defer 在函数返回路径上按 LIFO 执行，参数在 defer 语句处求值；panic 是沿调用栈展开的紧急退出，recover 只能在 defer 里接住。

## 先看一个现象

同一个变量，两种 defer 写法，答案不同：

```go
func demo() {
	i := 42
	defer fmt.Println("A:", i) // 打印 A: 42
	i = 99
	defer func() { fmt.Println("B:", i) }() // 打印 B: 99
}
```

A 后注册却**后**执行（B 先打印），而且 A 打印的是注册时的旧值 42。你以为 defer 是"离开这段代码时执行、执行时再取变量"；实际它挂在**整个函数**的返回路径上，按后进先出执行，并且参数在 `defer` 语句那一行就求值固定了。

panic 的行为更像意外：

```go
func risky() {
	defer fmt.Println("defer 仍然执行")
	panic("boom")
	fmt.Println("到不了这里")
}
```

panic 让"到不了这里"永远不执行，但已经注册的 defer 照常跑——panic 不是"跳过一切"，而是"换一条退出路径"，原来注册的清理还在路径上。

## 先建立直觉

:::figure 06-defer-panic|返回路径上的 defer 栈，与 panic 展开中的 recover 拦截点
图说明：左半边是一个函数的返回路径——三个 defer 按注册顺序入栈，返回时按 ③②① 弹出，return 先把结果写进命名返回值、再走 defer，所以 defer 能改写它；右半边是 panic 的展开过程——panic 沿调用栈逐层向上，每层的 defer 照常执行，某一层的 defer 里 recover() 拿到非 nil 值，展开就地停止。
:::

两半边共享同一个机制：**defer 是函数退出时唯一还会执行的代码**。正常返回靠它收尾，panic 展开也靠它收尾——所以 recover 必须长在 defer 里。

## 机制拆成四步

### 1. defer 挂在函数上，不在代码块上

`defer f()` 把调用登记到当前函数的延迟列表，函数返回（或 panic 展开）时按注册的逆序执行。为什么是逆序？因为资源获取与释放天然对称嵌套：后打开的连接先关、内层的锁先解，LIFO 正好匹配。为什么不在 `if` 或循环结束时执行？因为 defer 的作用域就是函数——这也是循环里注册的 defer 要等整个函数返回才跑的原因，循环体不是函数。

```go
func paired() {
	defer fmt.Println("清理 1（先注册）")
	defer fmt.Println("清理 2（后注册）")
	fmt.Println("主体")
	// 输出：主体 → 清理 2 → 清理 1
}
```

先注册的离资源远、后注册的离资源近，逆序弹出正好从最内层开始收尾。

### 2. 参数在 defer 语句处求值

`defer fmt.Println(i)` 是一次**完整的函数调用**，只是执行被推迟：实参 `i` 在这一行就求值并复制，之后 `i` 怎么变都与它无关。而 `defer func() { fmt.Println(i) }()` 推迟的是一个闭包，闭包体在执行时才去读 `i`，读到的是当时的值。

两种写法都对，区别只在"冻结什么"：想快照就传参，想看最终值就闭包捕获。判断口诀：**看到 defer 先问"实参什么时候算"，再问"闭包体什么时候读"。**

```go
for i := 0; i < 3; i++ {
	defer fmt.Printf("%d ", i) // 实参当场求值：函数退出时输出 2 1 0
}
```

闭包版本在 Go 1.22 之后每轮循环变量互相独立，但规则没有变：**闭包体执行时才读**，读到的是执行那一刻的值。

### 3. return 不是原子操作，defer 可以改写返回值

`return x` 实际分两步：先把 `x` 写入返回值，再执行 defer，最后真正返回。只有**命名返回值**能让 defer 摸到返回值——它有名字，defer 闭包里可以直接赋值：

```go
func run() (err error) {
	defer func() { err = fmt.Errorf("recovered: %v", recover()) }()
	panic("boom") // err 被改写成接住的内容，调用者拿到 error 而不是崩溃
}
```

匿名返回值是复制到栈上的临时量，defer 改不到。所以"defer 修改返回值"不是 defer 自带的属性，而是命名返回值提供的入口。反例看匿名返回值：defer 只能改到自己的局部变量：

```go
func anon() string {
	s := "结果"
	defer func() { s += "！" }() // 改的是局部变量 s
	return s                     // 返回 "结果"，感叹号丢了
}
```

### 4. panic 展开，recover 在 defer 里拦截

panic 立即停止当前函数的正常控制流，然后沿调用栈逐层展开；每一层的 defer 照常执行——这正是资源清理仍然有效的原因。某一层 defer 中直接调用 `recover()` 并拿到非 nil 值，展开就地停止，包含这个 defer 的函数带着它的修改（比如改过的命名返回值）正常返回，调用者继续运行。若没有任何 recover，展开到栈底，程序打印 panic 值与堆栈后退出。

为什么 recover 必须写在 defer 里？panic 发生后，普通代码已经不在执行路径上，唯一还会被调用的就是 defer；放在任何普通位置都捕不到。另外 recover 只作用于**当前 goroutine**：

```go
go func() {
	defer func() { recover() }() // 每个 goroutine 要有自己的拦截点
	panic("worker boom")
}()
```

main 里的任何 recover 都救不了这个 goroutine——展开只沿它自己的栈走。

## 动手验证

:::demo 06-defer-panic|LIFO、参数求值时机、命名返回值与 recover
先预测：`值拷贝` 和 `闭包读取` 各打多少、谁先谁后？`namedReturn` 返回的字符串里有没有 defer 追加的那截？`safeCall` panic 之后，"先注册的 defer"还会执行吗？main 还能继续打印吗？
验证：`cd examples/minimal/06-defer-panic && go run .`，对照六行输出：`闭包读取: 84` 先于 `值拷贝: 42`（LIFO，且求值时机不同）；`先注册的 defer 也会执行` 出现在 recover 接住之后（同一函数的 defer 全部走完）；`safeCall 返回: 接住 panic: boom` 之后 main 继续运行。再把 `boomMsg` 改成别的字符串，看 recover 的打印跟着变。
:::

## 容易误判的地方

- **"defer 在离开 if/for 时执行"** → 它挂在函数上；循环里注册 1000 个 defer 就攒 1000 个，直到函数返回才执行。要每次迭代就清理，把迭代体抽成函数。
- **"闭包 defer 会拷贝变量"** → 闭包体执行时才读变量，读到的是最终值；想快照就传参（`defer f(i)`）。两种写法代表两种意图，不是对错之分。
- **"recover 之后回到 panic 的地方继续"** → 不会。panic 点之后的代码永远不执行；recover 只是让当前函数正常返回，调用者从调用处继续。

## 自己再做一次

1. 把 demo 里 `defer fmt.Printf("值拷贝:   %d\n", i)` 改成 `defer func() { fmt.Printf("值拷贝:   %d\n", i) }()`。**预期结果**：两行都打 84——闭包执行时才读 `i`，"值拷贝"这个标签已经名不副实。
2. 给 `safeCall` 再加一个 `defer fmt.Println("最后注册的 defer")`（放在 recover 那个 defer 之后）。**预期结果**：recover 那层先执行并接住 panic，随后"先注册的 defer"、"最后注册的 defer"依次执行——同一函数的 defer 无论是否 recover 都会走完。
3. 写 `func mustPositive(n int) (int, error)`，n ≤ 0 时 panic，调用方用 defer + recover 把它转成 error 返回。**预期结果**：`mustPositive(5)` 返回 `(5, nil)`；`mustPositive(-1)` 返回 `(0, 非 nil error)` 且进程不退出——panic 被边界拦下，调用者继续运行。

> 下一篇：[并发不是多开线程](07-concurrency-model.md)
