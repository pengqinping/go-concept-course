# 12 · IO 是组合出来的

> 一句话模型：io.Reader/Writer 是两个极小接口，能力靠包装层叠加；组合改变"经过时发生什么"，不改变字节本身；谁打开谁负责 Close。

## 先看一个现象

把一段字符串复制进缓冲区，顺便想知道"经过了多少字节"：

```go
var dst bytes.Buffer
written, _ := io.Copy(&dst, strings.NewReader(data))
```

你以为要统计字节数就得改复制循环，或者到处插打印。实际在数据源外面套一层自定义 Reader，`io.Copy` 毫无感知，账已经记完了：

```go
type countingReader struct {
    src   io.Reader
    total int
}

func (c *countingReader) Read(p []byte) (int, error) {
    n, err := c.src.Read(p)
    c.total += n
    return n, err
}

written, _ := io.Copy(&dst, &countingReader{src: strings.NewReader(data)})
```

你以为包装会动数据。实际字节一个不少：计数层只是站在 Read 的必经之路上记一笔，`io.Copy` 只认 `Read(p []byte) (n int, err error)` 这个签名，根本不知道自己读的是 `strings.Reader` 还是七层包装。再加一层 `io.TeeReader`，一份数据还能同时抄送给第二个出口——能力就是这样一层层叠上去的。

## 先建立直觉

:::figure 12-io-boundary|多层包装 Reader 的管道剖面与 Close 责任
图说明：上半部分看字节从源到汇的路径——计数层、TeeReader、io.Copy 各占一段管道，每层标着唯一职责，Tee 分出去的副本与主流内容一致；下半部分是 Close 责任归属——打开者挂着 defer Close，借用方（包括所有包装层）只用不关。
:::

图上最该带走的一句话：**每一层只做一件事，责任却不随字节一起往下传。** 包装可以无限叠加，Close 不会跟着叠——它始终停在打开资源的那一行。

所以读一段 IO 代码的正确姿势，是先把它拆成"哪些层在搬运字节、哪些层在观察字节、谁负责关闭"，而不是逐个记 API。下面按这个顺序拆机制。

## 机制拆成四步

### 1. 接口为什么这么小

`io.Reader` 与 `io.Writer` 各只有一个方法。为什么故意这么小？约束越少，能接入的东西越多：文件、网络连接、字符串、内存块、压缩流，只要能实现"把字节填进 p 并报告数量"就能当 Reader 用。同理，任何中间件只要也实现这个签名，就能插进任意两段 IO 代码之间，双方都不用改一行。

这就是组合的入口：`io.Copy(dst, src)` 不关心 src 是什么，于是"计数""限量""分流"全部可以做成外面包一层的 Reader，而不是给 Copy 加参数。

### 2. 每层包装只做一件事

标准库的包装层各自只负责一件事：`bufio.Reader` 攒批，减少底层调用次数；`io.TeeReader` 分流，读一路、抄送一路；`io.MultiReader` 拼接，把多个源粘成一个源；`io.LimitReader` 限量，到上限就报告 EOF。

组合的顺序决定行为：计数层包在最外层，看到的是 `io.Copy` 每次索要的量（默认 32KB 大缓冲）；包在 bufio 里面，看到的是攒批之后取底层的大块。**数据一样，经过方式不一样**——排链的时候要按"谁观察谁的节奏"决定谁包谁。

一条链写出来就是把源从里往外一层层包住：

```go
src := strings.NewReader(data)        // 源
counted := &countingReader{src: src}  // 第一层：记账
tee := io.TeeReader(counted, side)    // 第二层：分流
io.Copy(&dst, tee)                    // 汇聚：字节最终到这里
```

为什么 `io.TeeReader` 的副本是"边读边抄"而不是事后复制？它没有地方存放整份数据——它在每次 Read 成功后，立刻把刚读到的 `p[:n]` 写给分流目标。这决定了它的适用边界：适合"流过什么就留一份什么"，不适合"回头再看一遍"的随机访问需求。

### 3. io.Copy 的循环：读到 EOF 为止

`io.Copy` 的全部工作是"读一块、写一块，直到 EOF 或出错"。为什么它不要求一次读满？因为 Read 的契约允许短读：`n` 可以小于 `len(p)`，甚至 `n > 0` 和 `err == io.EOF` 同一次调用一起返回。手工写一遍这个循环，契约就摆在明面上：

```go
buf := make([]byte, 32*1024)
for {
    n, err := src.Read(buf)
    if n > 0 {
        dst.Write(buf[:n]) // 先处理读到的，短读不是错误
    }
    if err == io.EOF {
        break // EOF 是正常结束，不是失败
    }
}
```

这个契约正是计数 Demo 能工作的原因：计数层每次只交 4 个字节，`io.Copy` 不抱怨、不假设，照常一轮轮循环，最终 written 与累计字节分毫不差。

### 4. Close 的责任：谁打开谁负责

包装层叠加能力，但不接管生命周期：`io.TeeReader`、`io.LimitReader`、`io.MultiReader` 都不会替你关闭底层 Reader。为什么？它们只是"借用"——如果包装层擅自 Close，一个只读前几个字节的临时包装就会把还要用的资源关掉。

规则因此很朴素：**谁打开，谁负责 Close；要转移所有权，就显式交接**——签名收 `io.ReadCloser` 并说明"会关闭"，或反过来明确"借用、不关闭"。defer 写在打开的那一行旁边，panic 路径也覆盖得到。

| 你想要的 | 组合方式 |
| --- | --- |
| 统计经过的字节 | 自定义计数 Reader 包住源 |
| 一份数据两处用 | io.TeeReader |
| 两个源当一个用 | io.MultiReader |
| 最多读 N 字节 | io.LimitReader |

## 动手验证

:::demo 12-io-boundary|计数、分流与一条被记着的 Close
先预测：计数层每次 Read 最多交 `readChunk` 字节，最终计数还会等于 `io.Copy` 的 written 吗？TeeReader 抄送的 side 和流入 dst 的内容一致吗？"已关闭"会在程序的什么时刻打印？
验证：`cd examples/minimal/12-io-boundary && go run .`。Demo 用 strings.Reader 做源，包一层计数 Reader，再包 TeeReader，`io.Copy` 进 bytes.Buffer，打印 written、Read 次数与累计字节、两路内容。把 `readChunk` 改成 8：Read 次数变少，written 与累计字节不变——节奏变了，账没变。结尾的 tracingCloser 用 defer 关闭，"已关闭"打印在 main 返回前。
:::

## 容易误判的地方

- **"包了一层，数据会被复制或改动"** → 包装层只决定"经过时发生什么"，字节原样通过。Tee 抄送副本，主流内容一个字节不动。
- **"包装层会顺手替我关闭底层"** → 不会。TeeReader、LimitReader、MultiReader 都不持有关闭责任，Close 停留在打开者的 defer 里。
- **"Read 会把我给的缓冲区填满"** → 短读合法且常见。消费代码必须按 `n` 推进、容忍 `n>0` 与 EOF 同帧返回，否则丢数据或死循环都有可能。

## 自己再做一次

1. 把 Demo 的 `readChunk` 改成 8，运行两次对比 Read 次数。**预期结果**：次数明显变少，但 written、累计字节、两路内容完全不变——包装层改节奏，不改数据。
2. 用 `io.MultiReader(strings.NewReader("abc"), strings.NewReader("def"))` Copy 进一个 bytes.Buffer。**预期结果**：written=6，内容为 `abcdef`，两个独立源表现得像一个连续源。
3. 把 Demo 的 side 从 `bytes.Buffer` 换成 `io.Discard`。**预期结果**：dst 与 written 都不变——分流目标只是多一个出口，不影响主流。
4. 删掉 tracingCloser 的 `defer src.Close()`。**预期结果**：程序正常结束，但"已关闭"不再打印——漏关不会报错，Close 纪律只能靠归属规则维持。

> 下一篇：[JSON 是另一套边界](13-json-boundary.md)
