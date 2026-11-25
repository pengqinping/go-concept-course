# 05 · 失败是值，不是意外

> 一句话模型：Go 把失败作为返回值，是为了让失败成为调用者必须处理的普通数据；`%w` 把一次失败变成可追溯的链。

## 先看一个现象

```go
_, err := loadUser(404)
fmt.Printf("%v\n", err)
// handler: fetch: id=404: lookup id=404 kind=missing: not found
fmt.Println(errors.Is(err, ErrNotFound)) // true
```

你以为：加了几层上下文，原始错误就被"换成"一个新错误了。实际：`%w` 把原因挂在新的错误值上，形成一条链；文本变长了，但 `errors.Is` 仍能沿着链找到最底下的哨兵。同一个失败，上层看到的是上下文，下层判定的是原因——这两件事可以同时成立。

再看一个反例：把代码里的 `%w` 改成 `%v`，文本一个字不变，`errors.Is` 却变成 `false`。说明被判定的是那条链，不是那串字。

```go
cause := errors.New("disk full")
wrapped := fmt.Errorf("write: %w", cause)
flat    := fmt.Errorf("write: %v", cause)

fmt.Println(wrapped.Error() == flat.Error()) // true：文本一样
fmt.Println(errors.Is(wrapped, cause))       // true
fmt.Println(errors.Is(flat, cause))          // false：链断了
```

两个错误打印出来都是 `write: disk full`，调用者却只有其中一个能追问原因。差别藏在值里面，不在文本里。

## 先建立直觉

:::figure 05-error-as-value|失败沿栈返回，%w 串成链，Is / As 沿链判定
图说明：左半边是调用栈，失败作为普通返回值逐层往回走；右半边是包装链，每层通过 `Unwrap()` 挂住原因。底部两条判定路径：`errors.Is` 沿链比较目标值，`errors.As` 沿链找目标类型并把结果写回。
:::

图里"返回值"和"链"是同一件事的两个视角：返回值负责把失败送到调用者，链负责让调用者在不破坏上下文的前提下追问原因。

## 机制拆成五步

### 1. 失败占据一个返回值位置

函数签名把失败写成第二个结果，调用者不接它就拿不到正常结果。为什么不用异常？因为异常会让失败从正常控制流里消失：函数签名看不出会失败，调用处也看不出该处理什么。`if err != nil` 只是一个普通分支，编译器不会替你跳过它。

对照 panic 的用途：panic 留给"程序不该继续"的 bug，比如不该发生的索引越界；"ID 没查到""输入格式不对"是可预期失败，应该走返回值。把预期失败写成 panic，等于把恢复责任推给最外层，还让中间层无法就地决策。

```go
func divide(a, b int) (int, error) {
    if b == 0 { return 0, errors.New("divisor is zero") }
    return a / b, nil
}
```

签名本身就是文档：调用者看到两个返回值，就知道第二个需要检查。若改成 `panic("divisor is zero")`，签名里什么也看不出来，调用者只能靠读实现或靠运行时崩溃才发现问题。

### 2. 哨兵错误：用 `errors.Is` 而不是字符串比较

错误文本是给人看的，改个措辞判定就崩。哨兵是包级可比较的错误值，比如 `var ErrNotFound = errors.New("not found")`。`errors.Is(err, ErrNotFound)` 会沿 `Unwrap` 链逐个比较，因此包装之后依然能回答"是哪一类失败"。

为什么不直接 `err == ErrNotFound`？只在确定没有包装、且动态类型可比较时它才成立；一旦经过 `%w`，比较就失败。`errors.Is` 还兼容自定义了 `Is(error) bool` 的类型，统一用它更稳。

```go
var ErrNotFound = errors.New("not found")
err := fmt.Errorf("load user: %w", ErrNotFound)
fmt.Println(err == ErrNotFound)          // false：已经不是同一个值
fmt.Println(errors.Is(err, ErrNotFound)) // true：链上还有它
```

### 3. `%w` 建立链，`%v` 只留下文本

`fmt.Errorf("fetch: %w", err)` 生成一个带 `Unwrap() error` 的包装错误，原错误对象仍挂在链上；`fmt.Errorf("fetch: %v", err)` 只是把原因格式化成字符串，原对象被丢弃，`Is` 和 `As` 再也走不下去。

为什么需要区分？因为"给错误加上下文"和"保留错误身份"是两件事： `%v` 只做了前者，`%w` 两件都做。判断一个包装是否可逆，看它有没有 `Unwrap` 就够了。

```go
wrapped := fmt.Errorf("load: %w", cause) // 有 Unwrap()，指向 cause
flat    := fmt.Errorf("load: %v", cause) // 只有格式化后的字符串
```

包装还可以继续叠加：每一层 `%w` 都在链上再挂一节，文本前缀逐层加长，链却始终连着最底下的原因。

### 4. `errors.As` 取回结构化错误

`Is` 只能回答"是不是这一类"。要读具体字段——比如某个具体错误的 `ID`、`Path`——就得用 `errors.As`：它沿链找第一个可赋值给目标类型的错误，把它写回目标变量。

目标必须是**指针的指针**：

```go
var le *LookupError
if errors.As(err, &le) { fmt.Println(le.ID) }
```

为什么多一层 `&`？因为 `As` 要修改调用者的变量，参数得是"指向 `*LookupError` 的指针"。直接传 `le` 不仅写不回去，还会 panic。

两个判定函数分工明确：

| 想问的问题 | 用什么 | 第二个参数 | 命中后得到什么 |
| --- | --- | --- | --- |
| "是不是这一类失败" | `errors.Is` | 一个错误值，如 `ErrNotFound` | true / false |
| "要读具体字段" | `errors.As` | 指针的指针，如 `&le` | true，`le` 被赋值 |

### 5. defer 让失败路径也完成清理

多返回值加 `if err != nil { return }`，让一个函数有很多个出口。把 `Close()` 交给 `defer`，就等于把清理绑在"函数退出"，而不是绑在某一条 `return` 上——成功和失败都执行。

一个细节：defer 按后进先出执行。如果想在清理后观察状态，观察用的 defer 要先注册，`Close` 的 defer 后注册，这样 `Close` 先跑，观察者才看得到已关闭的状态。

```go
rc := &tracingCloser{}
defer func() { fmt.Println("观察：closed =", rc.closed) }() // 先注册，后执行
defer rc.Close()                                          // 后注册，先执行
```

反过来注册，观察者就会看到 `closed=false`——不是清理没发生，而是它发生得更晚。demo 里的 `Close()` 自己打印了一行，所以这行出现在 `main` 的所有输出之前：`loadUser` 一返回，defer 就执行了。

## 动手验证

:::demo 05-error-as-value|包装一次失败，用 Is 和 As 从链上取回信息
先预测：`loadUser` 失败后，`errors.Is(err, ErrNotFound)` 与 `errors.As(err, &le)` 分别是 true 还是 false？`defer` 的 `Close()` 会不会在失败路径上执行？最后那个 `%v` 包装的 `Is` 结果是什么？
验证：运行 demo，对照错误文本与四行判定，注意 `Close()` 打印发生在任何 `main` 输出之前。再把 `layerTag` 改成别的字符串，只有文本前缀变化，两个判定保持不变。
:::

demo 里故意没有真实文件 IO：资源用内存中的 `tracingCloser` 代替，所以输出确定可核对，失败也一定是"没找到"。

## 容易误判的地方

- **"有 err 就该 panic，或干脆吞掉返回 nil"** → 预期失败是正常数据；吞掉它会让上层以为成功，比 panic 更难定位。panic 留给不可恢复的程序错误。
- **"`%v` 和 `%w` 只是打印格式不同"** → `%w` 保留可遍历的链，`%v` 把原因降级成文本。demo 最后一行 `Is=false` 就是证据。
- **"`errors.As(err, &le)` 里的 `&` 多余"** → 必须传 `*T` 的地址，也就是 `**T`；少一个 `&` 无法写回结果，还会触发 panic。

## 自己再做一次

1. 把 demo 里三处 `%w` 全部改成 `%v`。**预期结果**：错误文本完全不变，`errors.Is(err, ErrNotFound)` 和 `errors.As(err, &le)` 都变成 false。
2. 把 `targetID` 改成 7。**预期结果**：`err == nil`，`loadUser` 正常返回 `"alice"`；`defer` 那行 `Close()` 仍然打印——成功路径同样会清理。
3. 在 `main` 末尾再加一层 `err = fmt.Errorf("main: %w", err)`。**预期结果**：文本多一层 `main:` 前缀，而 `errors.Is` 与 `errors.As` 依旧为 true——链的中间层不影响两种判定。

> 下一篇：[defer 到底什么时候执行](06-defer-panic.md)
