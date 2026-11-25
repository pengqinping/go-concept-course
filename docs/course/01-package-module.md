# 01 · 包、模块与构建边界

> 一句话模型：包是可见性与编译单元，模块是依赖与版本边界；构建标签在编译前决定"哪些文件存在"。

## 先看一个现象

demo 里 printer 子包就在隔壁目录，main.go 里却出现了两个"意外"：

```go
import "example.com/go-course/01-package-module/printer"

func main() {
	fmt.Println(printer.Print("Go", 2))    // 正常
	fmt.Println(printer.decorate("Go", 2)) // 编译失败
}
```

你以为 import 写的是"隔壁目录的相对路径"，实际它写的是**模块路径 + 包目录**拼出的完整路径——写成 `import "printer"` 反而找不到包。你以为 `decorate` 只是"不建议外部调用"，实际它首字母小写、未导出，编译器直接拒绝：这不是风格建议，是语言边界。

再往前一步：如果三个 `.go` 文件里有两个定义了同名函数 `platformMessage`，编译却不报"重复定义"。你以为编译器会读完目录里所有文件；实际构建标签在编译开始前就把不匹配的文件筛掉了，编译器根本没见过它们。**编译不是从"目录里的全部文件"出发，而是从"被选中的文件"出发。**

## 先建立直觉

:::figure 01-package-module|模块定路径，标签筛文件，包定可见性
图说明：左半边是 main 与子包 printer 之间的可见性边界——`Print` 大写出得去，`decorate` 小写出不去；右半边是构建的三道关卡：`go.mod`/`go.sum` 先定 import 路径与依赖版本，构建标签再筛掉不参与编译的文件，剩下的按包编译，嵌入文件随二进制一起交付。
:::

记住两件事的位置：大小写这条线画在**包与包之间**；三道关卡按**时间先后**排序——模块解析最早，文件筛选次之，编译最后。后面每一步都在回答"这道关卡为什么放这里"。

## 机制拆成四步

### 1. 包：目录只是住址，package 声明才是身份

同一个目录下的文件必须声明同一个 `package`，编译器以包为单位编译。为什么以包为单位而不是以文件为单位？因为包是"一起编译、一起复用"的最小单元：改包里任何一个文件，整个包重新编译；包内标识符互相直接可见，不用 import 也不用前缀。

跨包的规则只有一条：标识符首字母大写即导出。为什么选首字母而不是 `export` 关键字？可见性写在定义处，读代码时一眼可辨，不需要工具或文档确认。于是 `printer.Print` 能被 main 调用，`printer.decorate` 不能——后者也没有"私有但可绕过"的口子，跨包调用就是编译失败。

```go
package printer

func Print(word string, edge int) string  { return decorate(word, "=", edge) }
func decorate(word, deco string, edge int) string { /* 未导出：只有包内可见 */ }
```

同目录、同包的另一个文件不需要这些仪式——互相直接用，连 import 都不写：

```go
// printer/page.go 与 printer.go 同包
package printer

func Page(s string) string { return decorate(s, "-", 4) }
```

包内不设防，包外不放过：可见性只有一个粒度，就是包。

还有一个隐式边界：`internal/` 目录下的包，只能被同一个模块内、以 `internal` 祖先目录为根的那部分代码导入。放进去的代码天然是"模块私有"的。

### 2. 模块：import 路径的前缀与版本边界

`go.mod` 第一行 `module example.com/go-course/01-package-module` 给模块内所有子包一个公共前缀。导入路径 = 模块路径 + 包在模块内的相对目录，所以 `printer/` 目录要写 `example.com/go-course/01-package-module/printer`。为什么不让 import 写相对路径？因为相对路径只在你的磁盘上成立，模块路径让这个包在任何人、任何机器上都能被唯一引用。

```text
module example.com/go-course/01-package-module

go 1.24
```

读完这一行就能推导入路径：模块内任何目录的包，路径都是"这行 + 相对目录"。版本怎么进路径，看两个例子：

```text
example.com/shop/catalog/item  → 模块 example.com/shop 里 item/ 目录的包
example.com/greeting/v2        → v2 写进路径本身，v1 与 v2 是两个模块
```

模块同时是版本边界：`require` 声明依赖哪个版本，`go.sum` 记录每个依赖内容的校验和，防止拿到的代码与声明不一致。多个依赖共同要求同一模块时，按最小版本选择取"满足所有要求的最低版本"，不会因为更高的版本存在就自动升级。一个细节：主版本从 v2 起要写进模块路径本身（`.../v2`），让新旧大版本作为两个不同模块共存。

### 3. 构建标签：编译前的文件筛选

`//go:build linux` 写在文件顶部、`package` 之前、隔一个空行，意思是"本文件只在目标平台为 linux 时参与构建"。`&&`、`||`、`!` 可以组合；`message_linux.go` 这类文件后缀是等价的简写。没选中的文件不是"编译时跳过"，而是**从一开始就不存在**——所以同名的重复定义不报错，文件里未使用的 import 也不报错。

为什么要放在编译之前？因为平台差异常常是 API 级别的：某个系统调用只在 Linux 上存在，`if runtime.GOOS == "linux"` 这种运行时判断救不了编译——代码必须先通过类型检查。把互斥的实现放进按标签选择的文件里，另一个平台才编得过。

自定义标签用于可选功能，机制完全相同：

```go
//go:build demo

package main

func debugBanner() string { return "DEBUG BUILD" }
```

平时 `debugBanner` 根本不存在；`go run -tags demo .` 时它才参与编译——想加调试能力就加标签，不必为调试代码留一个运行时开关。

与其猜，不如问工具：

```bash
go list -f '{{.GoFiles}}' .             # 本机这次构建用了哪些文件
GOOS=linux go list -f '{{.GoFiles}}' .  # 交叉目标会用哪些文件
```

### 4. go:embed：把文件变成编译期输入

```go
//go:embed banner.txt
var banner string
```

嵌入路径相对**源文件所在目录**，与程序运行时的工作目录无关；嵌入发生在编译期，`banner` 只是拿到初始值。为什么做成编译期而不是运行时读文件？因为静态资源随二进制一起交付，部署时少一个"文件没拷齐"的故障点——代价是改了文件必须重新构建，且嵌入不等于加密，拿到二进制的人能提取内容。只嵌一个变量用 `_ "embed"` 空白导入；要嵌入一组文件就声明 `embed.FS`。嵌入类型按用途选：

- `string`：文本、模板、提示语。
- `[]byte`：需要按字节处理的数据。
- `embed.FS`：一组文件，用 `//go:embed assets/*` 一并收进来。

## 动手验证

:::demo 01-package-module|导出边界、模块路径与嵌入文件
先预测：取消 `printer.decorate` 那行注释，编译错误指向哪个文件？`banner.txt` 里是 5 个可见字符加一个换行，`len(banner)` 打多少？把 `Print` 改名 `print`，报错出现在 main.go 还是 printer.go？
验证：`cd examples/minimal/01-package-module && go run .`，对照 `banner` 与 `Print` 两行输出；依次做三件事——取消注释看编译失败、`edge` 改成 4 看边框变宽、`go list -f '{{.GoFiles}}' .` 看本次构建实际包含哪些文件。
:::

demo 里没有构建标签文件，但 `go list` 这一步可以直接迁移到任何项目：核对"哪些文件存在"，永远比猜可靠。

## 容易误判的地方

- **"同一个目录就是同一个包，目录名就是包名"** → 身份来自每个文件顶部的 `package` 声明；目录只是磁盘布局。目录名与包名可以不同，import 路径末段与包名也可以不同——读代码时两个都要看。
- **"构建标签是运行时开关"** → 它在编译前筛选文件，选错了文件集合，某个平台直接编译不过；需要运行时切换的是配置，不是标签。
- **"go.sum 是依赖版本列表"** → 版本写在 `go.mod` 的 `require` 里；`go.sum` 是内容校验和，由 `go mod tidy` 维护，不应手工编辑。

## 自己再做一次

1. 给 printer 包加一个导出函数 `Repeat(s string, n int) string`，在 main 里调用。**预期结果**：编译通过；把它改成小写 `repeat` 后，编译失败且报错指向 main.go 的调用处——可见性检查发生在使用它的地方。
2. 新建 `platform_linux.go`，顶部写 `//go:build linux` 并定义 `func platform() string`，在 main.go 里调用它。**预期结果**：本机 `go build .` 报 `undefined: platform`，`GOOS=linux go build` 通过——同一个 main，文件集合不同。
3. 先 `go build` 出二进制，再修改 `banner.txt` 内容，运行旧产物。**预期结果**：打印的仍是旧内容——嵌入发生在编译期，改文件不等于改产物。
4. 把 main.go 的导入改成 `p "example.com/go-course/01-package-module/printer"`，用 `p.Print` 调用。**预期结果**：编译通过，输出不变——别名只改调用处的写法，不改变可见性规则。

> 下一篇：[赋值到底复制了什么](02-value-semantics.md)
