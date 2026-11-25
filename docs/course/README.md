# Go 概念课

这是 Go 学习材料的**唯一一套主线**：19 篇短教程，每篇一张真正的 SVG 概念图 + 一个可运行的 Go Demo，按"现象 → 直觉 → 机制 → 验证 → 边界"推进。读者起点是"有些 Go 基础，但概念总差一层"。

> 这套课程已完全取代早期的 60 篇百科式文档（其概念要点已吸收进对应章节；纯语法、API 字典类内容按设计不单独成章）。

## 19 篇主线

| 顺序 | 章节 | 一句话模型 |
|---:|---|---|
| 01 | [包、模块与构建边界](01-package-module.md) | 包是可见性与编译单元，模块是依赖边界；构建标签决定哪些文件参与编译。 |
| 02 | [赋值到底复制了什么](02-value-semantics.md) | 所有赋值都是值拷贝；所谓引用类型，拷贝的是指向共享数据的描述符。 |
| 03 | [字符串与 Map 的边界](03-strings-maps.md) | 字符串是只读字节序列；Map 无序、读零值安全、nil 不能写。 |
| 04 | [接口是谁定义的](04-interface-boundary.md) | 接口是调用方对能力的最小描述，实现关系由方法集合隐式满足。 |
| 05 | [失败是值，不是意外](05-error-as-value.md) | 失败作为返回值被显式处理；`%w` 把一次失败变成可追溯的链。 |
| 06 | [defer 到底什么时候执行](06-defer-panic.md) | defer 在返回路径上 LIFO 执行、参数即求值；recover 只能在 defer 里接住 panic。 |
| 07 | [并发不是多开线程](07-concurrency-model.md) | Goroutine 是可暂停的 continuation，Channel 是同步协议。 |
| 08 | [取消是通知，不是命令](08-context-cancel.md) | Context 只广播"请停止"，真正停下靠被调代码自己检查。 |
| 09 | [共享状态怎么才安全](09-shared-state.md) | 同时访问且至少一写就必须建立 happens-before；Mutex 保护临界区。 |
| 10 | [谁来等这些 goroutine](10-orchestration.md) | WaitGroup 管汇合、channel 管接力；泄漏来自没人接收的阻塞。 |
| 11 | [内存去哪了](11-memory-escape.md) | 栈或堆由逃逸分析决定，GC 只回收不可达对象。 |
| 12 | [IO 是组合出来的](12-io-boundary.md) | Reader/Writer 是极小接口，能力靠包装叠加；谁打开谁负责 Close。 |
| 13 | [JSON 是另一套边界](13-json-boundary.md) | struct tag 是类型世界与文本世界之间的翻译规则，翻译永远有损。 |
| 14 | [组合、嵌入与包边界](14-composition.md) | 复用靠小接口 + 组合；嵌入是委托语法，不是继承。 |
| 15 | [一次请求穿过哪些边界](15-http-boundary.md) | Server 管连接，Mux 管路由，Handler 管一次请求。 |
| 16 | [事务的边界](16-database-transaction.md) | 事务是多步写的全有全无边界；参数化查询把数据留在数据侧。 |
| 17 | [测试是设计反馈，不是验收仪式](17-testing-feedback.md) | 测试先逼你写清期望，再逼你隔离依赖。 |
| 18 | [泛型什么时候值得](18-generics-limits.md) | 泛型解决"多种类型重复同一算法"，不是替代接口的万能工具。 |
| 19 | [让程序在真实世界活下来](19-failure-in-production.md) | 可靠性来自边界、幂等键和停机顺序，不是某一层框架。 |

## 怎么读

1. **先预测，再看答案。** 每篇的"动手验证"都给了一个明确问题，先写下自己的判断。
2. **真运行 Demo。** 每个概念对应 `examples/minimal/<序号>-<slug>/`，进入目录执行 `go run .`；标有 `// 可调参数` 的常量就是旋钮。
3. **用 -race 复核并发结论。** 默认 `go test -race ./...` 全部通过；第 09 篇另有一个**故意失败**的可选演示：`go test -tags racedemo -race ./...` 会报告 DATA RACE。
4. **按序读，也可以跳。** 章节按依赖关系排列（01–06 语言机制，07–11 并发与内存，12–16 数据与 IO 边界，17–19 工程与可靠性），每篇自足，可按需进入。

## 双形态交付

- Markdown 是唯一源，普通 Markdown 预览器即可阅读；`:::figure` / `:::demo` 是扩展块。
- 离线阅读站：[course-site/index.html](../../course-site/index.html)。图与源码全部内嵌，章节可切换，打印可导出全书。
- 修改任一 Markdown、SVG 或 Demo 源码后，运行 `python3 course-site/build_site.py` 重建站点。

写作规范见 [STYLE.md](STYLE.md)。
