# 04 · 接口是谁定义的

> 一句话模型：接口不是实现类要遵守的清单，而是调用方对能力的最小描述；实现关系由方法集合隐式满足。

## 先看一个现象

一个读取函数只写出它要的能力：

```go
func readAll(r io.Reader) (string, error) { /* ... */ }

readAll(strings.NewReader("go")) // 可以
readAll(bytes.NewBufferString("go")) // 也可以
```

你以为：一个类型要先声明"我实现 `io.Reader`"，才能传进去。实际：`io.Reader` 的全部内容只有一行 `Read(p []byte) (int, error)`，谁的方法集合里有这个方法，谁就能进来——`strings.Reader` 和 `bytes.Buffer` 都没有 import 这个接口，更没有写 `implements`。

场景换成写入也一样。如果签名写死具体类型，测试替身永远进不来：

```go
func save(b *bytes.Buffer) error { /* ... */ } // 写法一
// 或者：
func save(w io.Writer) error { /* ... */ }     // 写法二
```

第一行要求调用方交出 `bytes.Buffer`；第二行只要交出"能写字节的东西"，`bytes.Buffer`、文件、以及自己写的一个只有 `Write` 方法的假对象都能进来。**接口把"谁能被调用"的决定权，从实现方挪到了调用方手里。**

再看一个别扭的地方：`bytes.Buffer` 有几十个方法，`readAll` 却只能看到 `Read`。接口不是给对象贴的类别标签，而是调用方拿起对象时戴的一副手套：只露出需要的那根手指。

## 先建立直觉

:::figure 04-interface-boundary|调用方的小接口，与接口变量的剖面
图说明：左半边是调用方写下的最小接口，三个毫不相干的实现凭各自的 `Read` 方法隐式进入；右半边剖开一个接口变量，看到 `(动态类型, 动态值)` 两格。`typed nil` 只空了第二格，所以接口本身不是 nil。
:::

右半边那两格是本篇最容易被忽略的事实：接口变量从来不是"一个对象"，而是"类型 + 值"的一对信息。后文所有关于 nil 的结论都从这两格推出来。

## 机制拆成四步

### 1. 接口写在调用方，描述"我需要什么"

为什么由调用方定义？因为需求掌握在调用方手里：我只要"能读出字节"，就不该要求对方连 `Write`、`Seek`、`Close` 都实现。小接口让测试替身只写一个方法，也让标准库里原本毫不相干的类型直接可用。依赖方向因此倒过来：实现方不需要知道接口存在。

对比两种宽度，差别不在语法，而在谁有资格进来：

```go
type Writer interface { Write(p []byte) (int, error) } // 一个方法

type WriterPlus interface {
    Write(p []byte) (int, error)
    Flush() error
    Close() error
    Name() string
}
```

`WriterPlus` 逼着测试替身写四个方法，其中三个与"写入"无关；`Writer` 只要求一个。接口每多一个方法，能进来的实现就少一批。

这不是绝对规则。跨包公共契约（比如 `io.Reader`）适合集中定义，所有人才能复用同一个名字。判断标准始终是：这个接口是否准确表达了需求，它的变化会不会波及无关实现。

### 2. 满足关系是方法集合的隐式匹配

Go 没有 `implements` 关键字。编译器在**赋值或传参那一刻**检查：右侧类型的方法集合是否包含接口要求的方法，签名逐个一致就通过。

为什么错误出现在调用处，而不是类型定义处？因为"满足哪些接口"不是类型的固有属性——一个类型可以同时满足 `Reader` 和 `Writer`，也能被将来新写的接口满足。这里要记一条方法集合规则：`T` 的值方法集合只含值接收者方法，`*T` 才同时拥有值接收者和指针接收者方法。所以一个只用指针接收者定义 `Read` 的类型，`T{}` 传不进去，`&T{}` 可以。

```go
type OnlyPtr struct{}
func (o *OnlyPtr) Read(p []byte) (int, error) { return 0, io.EOF }

var bad Reader = OnlyPtr{}  // 编译失败：值方法集合不含 Read
var ok  Reader = &OnlyPtr{} // 通过：指针方法集合含 Read
```

报错位置是这两行赋值，不是 `OnlyPtr` 的定义处——同一个类型换个接口，结论可能完全不同。

### 3. 接口变量里装的是（动态类型，动态值）

把 `bytes.Buffer` 传给 `Reader`，接口拿到的是"当前类型 `*bytes.Buffer` + 当前值"这一对信息。为什么必须有类型？接口变量的静态类型只有 `Reader`，运行时得知道具体是谁，才能把 `Read` 分派到正确的方法体。为什么必须有值？方法需要接收者。

于是接口调用是一次运行时分派，而不是编译期写死的地址。这也接上了上一篇：装进接口的是值的副本；如果那个值本身是指针，复制的只是指针，底层对象并没有被复制。接口没有改变 Go 的值语义。

```go
bb := bytes.NewBufferString("go")
var r Reader = bb  // r 里：类型 *bytes.Buffer，值是 bb 这个指针的副本
bb.Reset()         // 底层对象被改，r 下一次 Read 看到的是新内容
```

`r` 持有的是指针副本，不是 `Buffer` 的副本——这一点和上一篇的 `t := s` 是同一个道理。

### 4. typed nil：为什么 `boxed == nil` 是 false

```go
var buf *bytes.Buffer
var r Reader = buf
// buf == nil → true，r == nil → false
```

为什么？nil 接口要求"类型和值都为空"，而 `r` 的类型栏写着 `*bytes.Buffer`，只空了值栏。此时调用 `r.Read(...)`，方法体解引用空指针就会 panic——接口看起来能用，其实是个空壳。

怎么处理？在构造边界就杜绝：创建失败时直接返回 nil 接口，而不是把失败的具体指针升级成接口。若某个实现确实支持 nil 接收者，要显式写明并测试，别假设所有实现都遵循同一约定。

把三栏摆在一起看更清楚：

| 写法 | 动态类型 | 动态值 | `== nil` |
| --- | --- | --- | --- |
| `var r Reader` | 空 | 空 | true |
| `var b *bytes.Buffer; var r Reader = b` | `*bytes.Buffer` | nil | false |
| `var r Reader = bytes.NewBufferString("go")` | `*bytes.Buffer` | 非 nil 指针 | false |

`== nil` 只能识别第一行。第二行是"有类型、没值"，第三行是"有类型、有值"。

## 动手验证

:::demo 04-interface-boundary|两种实现传入同一接口，再看 typed nil 的两栏
先预测：`strings.Reader` 和 `bytes.Buffer` 都能传进自定义的 `Reader` 吗？把 `(*bytes.Buffer)(nil)` 装进去后，`boxed == nil` 是 true 还是 false？`%T` 会打印什么？
验证：运行 demo，逐行对照输出。特别比较 `nilBuf == nil` 与 `boxed == nil`——它们可以同时一个 true、一个 false。demo 只做判定，不调用 `Read`，所以不会 panic。
:::

demo 的重点是建立一条检查路径：**先问动态类型在不在，再问动态值是不是 nil**；`== nil` 只完成了第一半。

## 容易误判的地方

- **"实现要写 `implements`"** → 隐式的方法集合匹配。编译错误发生在传参处，不是类型定义处；这也意味着第三方类型无需改动就能满足你的接口。
- **"接口里装的是对象副本"** → 装的是值的副本。值是指针时只复制指针，底层对象仍然共享。接口没有开启第二条值语义。
- **"`r == nil` 就足以防住空"** → 挡不住 typed nil。真正的 nil 接口是 `var r Reader` 这种两栏全空的变量，两类空必须分开判断。

## 自己再做一次

1. 写 `type Buf struct{}`，只定义 `func (b *Buf) Read(p []byte) (int, error)`，然后 `readAll(Buf{})`。**预期结果：编译失败**，`Buf` 的值方法集合不含 `Read`；改成 `readAll(&Buf{})` 通过。
2. 给 demo 里的 `Reader` 接口加一个方法 `Close() error`。**预期结果**：`strings.Reader` 和 `bytes.Buffer` 都没有这个方法，于是都不再满足，编译在 `readAll(sr)` 那一行失败——接口越大，能进来的实现越少。
3. 把 `var nilBuf *bytes.Buffer` 换成 `var nilBuf *strings.Reader`，其余不动。**预期结果**：`boxed == nil` 仍是 false，`%T` 变成 `*strings.Reader`；结论与具体类型无关，只取决于"类型栏非空"。

> 下一篇：[失败是值，不是意外](05-error-as-value.md)
