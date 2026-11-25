# 13 · JSON 是另一套边界

> 一句话模型：结构体是你程序的类型世界，JSON 是外部文本世界；struct tag 是两套世界之间的翻译规则，而翻译永远有损——未导出字段、未知字段、时间与数字都各有边界。

## 先看一个现象

同一个 `user` 值，编码再解码，回不到原样：

```go
type user struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
	kind string // 内部分类，不该离开进程
}

u := user{ID: 7, Name: "ada", kind: "internal"}
b, _ := json.Marshal(u)
fmt.Println(string(b)) // {"id":7,"name":"ada"}

var back user
_ = json.Unmarshal(b, &back)
fmt.Println(back.kind) // 空串：internal 留在了门内
```

你以为 Marshal/Unmarshal 是"原样搬出去、原样搬回来"，实际 `kind` 根本没出门。

反方向同样有筛子。对方发来一个结构体里没有的字段：

```go
input := []byte(`{"id":7,"name":"ada","role":"admin"}`)

var lenient user
_ = json.Unmarshal(input, &lenient) // err = nil
fmt.Println(lenient)                // {7 ada  }，role 被静默丢弃
```

你以为多出来的字段至少会给个警告，实际什么都没有——哪怕它其实是 `nmae` 这样拼错的 `name`，也照样被当成未知字段吞掉，业务拿到一串零值还以为成功了。

还有第三处：缺失字段也不报错。`{"id":7}` 可以成功解码，`Name` 只是零值。翻译不是校验，语法通过不代表信息完整。

两次翻译，两次有损。判断数据穿过边界后会变成什么，靠的不是"字段有没有写 tag"，而是知道翻译规则在哪一侧生效、丢弃发生在哪一步。

## 先建立直觉

:::figure 13-json-boundary|结构体字段与 JSON 键之间的翻译剖面
图说明：左边是 Go 字段，右边是 JSON 键，中间的竖线就是 struct tag 定义的翻译边界。`id`、`name` 正常通行；`note` 带 `omitempty`，值为空时整个键缺席；未导出的 `kind` 和标注 `json:"-"` 的字段没有对应的键；JSON 侧多出来的 `role` 在默认规则下被丢弃。
:::

图要读出一件事：**翻译规则只覆盖两侧都"登记过"的字段。** 没登记的——未导出、主动排除、对方多发的——各有各的下场，而且大多是静默的。

四种门禁规则放在一起对照，它们分属三个不同的来源：

| 规则 | 来源 | 编码侧行为 | 解码侧行为 |
| --- | --- | --- | --- |
| 字段未导出 | Go 语言规则 | 不出门 | 不写入 |
| `json:"-"` | 开发者决定 | 主动排除 | 主动排除 |
| `omitempty` | tag 选项 | 零值时整键省略 | 无作用 |
| 默认规则（无开关） | 库的兼容性决策 | —— | 未知字段静默丢弃 |

## 机制拆成四步

### 1. 翻译规则写在 tag 里，但只有导出字段有资格参加

`encoding/json` 在你的包之外工作，而 Go 的可见性规则禁止包外代码读写未导出字段。所以未导出字段进不了翻译——这不是库的疏忽，是语言边界挡在那里。

tag 的另一个作用是改名：JSON 侧习惯小写或下划线命名，`ID` 映射成 `id` 由 tag 完成，不必让 Go 的导出命名向协议妥协。不写 tag 时默认用字段原名，`ID` 会变成 `ID`。

`json:"-"` 则是你主动把一个可导出的字段划出边界："这个字段允许包内其他代码看，但不许翻译出去。"三者性质不同：语言规则动不了，命名约定可以商量，开发者的排除写一行就能改。

### 2. omitempty 在编码侧省略零值：缺席与零值被合并

`omitempty` 让空串、0、nil、false 这类零值不出门。为什么说它有损？因为缺席只有一种形态，缺席的原因却有两种——"发送方没填"和"发送方填了零值"。

```go
a := user{ID: 7, Name: "ada"}                    // 没填 Note
b := user{ID: 7, Name: "ada", Note: ""}          // 明确填了空串
// Marshal(a) == Marshal(b)："note" 都不出现在文本里
```

解码侧只能观察到"键缺席"，后者的信息在编码那一步就被合并掉了。往返之后你手里只有零值，永远不知道它是不是被主动省略的。真要区分，得换成 `*string` 这类能表达"存在但为空"的表示，而不是指望 tag。

### 3. 解码默认忽略未知字段：这是兼容性决策，不是疏忽

为什么默认不报错？因为协议要演进：服务端新增一个字段，不能让所有旧客户端当场报错。忽略未知字段是向前兼容的代价。

代价的另一面是拼写错误（`nmae`）也被当未知字段吞掉。要拦住它，用 `Decoder` 收紧规则：

```go
dec := json.NewDecoder(bytes.NewReader(input))
dec.DisallowUnknownFields()
err := dec.Decode(&u) // json: unknown field "nmae"
```

这是用向前兼容换拼写检查：规则收紧后，服务端再加新字段，你的旧程序也会报错。所以它适合协议收口处——配置加载、内部服务之间的固定约定——而不是所有对外接口一刀切。它作用于标准解码路径上的每一层嵌套结构体，但自定义了 `UnmarshalJSON` 的类型会绕开默认字段处理，要靠嵌套类型的测试确认。

### 4. 时间与数字：类型信息不会跟着值过境

JSON 里没有时间类型。`time.Time` 能进 JSON，是因为它实现了 `MarshalJSON`，把自己翻译成 RFC 3339 格式的字符串，解码时再按同一格式解析回来：

```go
t := time.Date(2026, 11, 25, 7, 15, 55, 123000000, time.UTC)

b, _ := json.Marshal(t)
fmt.Println(string(b)) // "2026-11-25T07:15:55.123Z"

fmt.Println(t.String()) // 2026-11-25 07:15:55.123 +0000 UTC
```

往返后 `Equal` 成立，但两边文本是两套表示：Go 的 `String()` 和 JSON 的 RFC 3339 是两种书写习惯，编码时选哪套由类型的 `MarshalJSON` 决定，不由你每次调用决定。

数字同理。JSON 只规定"数字"这一种语法，不带 Go 类型：`int64` 的精确只保证到你这一侧。同一个 `9007199254740993`，Go 编码出去原样无误，接收方若把它解析成双精度浮点，末位就可能变成 `9007199254740992`。类型信息不随值过境，所以跨系统协议必须连"接收方用什么类型"一起约定，不能只约定字段名。

## 动手验证

:::demo 13-json-boundary|四种有损：未导出、omitempty、未知字段、时间格式
先预测：`noteText` 为空串时，`marshal` 的输出里有 `"note"` 吗？同一个带 `role` 的 JSON，默认 `Unmarshal` 和 `DisallowUnknownFields` 各返回什么？`time.Time` 往返后两边的文本一样吗，`Equal` 是多少？
验证：`cd examples/minimal/13-json-boundary && go run .`。`marshal = {"id":7,"name":"ada"}`——`kind` 不在输出里，空的 `note` 整键缺席。`lenient` 行显示 `role` 被丢弃且 `err = nil`；`strict` 行报 `json: unknown field "role"`。最后一段打印两个世界的文本：Go 侧是 `2026-11-25 07:15:55.123 +0000 UTC`，JSON 侧是 `"2026-11-25T07:15:55.123Z"`，`back.Equal(t) = true`——值回来了，表示换过了。把 `noteText` 改成 `"vip"` 再运行，`note` 键就出现了。
:::

demo 的重点不是背这四种行为，而是养成一个习惯：**字段进出边界前，先问它在另一侧对应什么**——没有对应物的字段，要么留在门内，要么被静默处理。

## 容易误判的地方

- **"Unmarshal 成功 = 数据没问题"** → 成功只说明语法与已知字段的类型对上了。未知字段被吞、缺失字段保持零值，都算"成功"；业务必填与取值范围要在解码之后自己校验，能用自定义类型表达的不变量应前移到类型边界。
- **"omitempty 能区分'没提供'和'提供了零值'"** → 不能。编码后文本里只有"键缺席"一种形态；需要区分就用指针类型，让"存在但为空"可以被表示。
- **"Marshal 和 Unmarshal 是互逆运算"** → 每次穿边界都在筛字段、换表示。未导出字段、被省略的字段、对方多发的字段，往返之后都回不到原来的信息量；需要完整保留时，把它们显式写进协议，而不是指望翻译替你记住。

## 自己再做一次

1. 把 `noteText` 改成 `"vip"` 再运行。**预期结果**：`marshal` 输出变成 `{"id":7,"name":"ada","note":"vip"}`；`lenient` 行的 `Note` 字段仍是空串——因为 `externalJSON` 里没有 `note` 键。
2. 把 `externalJSON` 里的 `role` 改成 `nmae`（拼错的 `name`）。**预期结果**：默认解码 `err = nil` 且 `Name` 为空串——拼写错误被当成未知字段吞掉；strict 版报 `json: unknown field "nmae"`，这才拦得住。
3. 把 demo 里的时间改成 `time.Date(2026, 11, 25, 7, 15, 55, 123456789, time.UTC)`。**预期结果**：JSON 文本保留全部纳秒位 `2026-11-25T07:15:55.123456789Z`，`back.Equal(t)` 仍为 `true`——精度在 RFC 3339 的表示范围内没有损失。
4. 给 `user` 加一个导出但不该出门的字段 `Debug bool`。**预期结果**：不加任何处理它会被编码出去；写上 `json:"-"` 后再运行，输出恢复为 `{"id":7,"name":"ada"}`——"要不要翻译"是开发者的一行决定。

> 下一篇：[组合、嵌入与包边界](14-composition.md)
