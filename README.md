# Go 概念课 · go-concept-course

19 篇中文 Go 概念教程，按「现象 → 直觉 → 机制 → 验证 → 边界」推进，每篇一张手绘 SVG 概念图 + 一个真实可运行的最小 Demo。目标是让读者"自己推演结果"，而不是记 API。

- 课程入口：[docs/course/README.md](docs/course/README.md)
- 离线阅读站：[course-site/index.html](course-site/index.html)（图与源码全部内嵌，双击即可离线打开，浏览器打印可导出全书）
- 章节目录：

| 部分 | 章节 |
|---|---|
| 语言机制 | 01 包与模块 · 02 值语义 · 03 字符串与 Map · 04 接口 · 05 错误 · 06 defer/panic |
| 并发与内存 | 07 并发模型 · 08 Context · 09 共享状态 · 10 并发编排 · 11 内存与逃逸 |
| 数据与 IO | 12 IO 组合 · 13 JSON · 14 组合与嵌入 · 15 HTTP 边界 · 16 事务 |
| 工程与可靠性 | 17 测试 · 18 泛型 · 19 生产可靠性 |

## 运行 Demo

```bash
cd examples/minimal/02-value-semantics
go run .
```

每个 Demo 只用标准库，文件顶部标有 `// 可调参数` 的常量就是实验旋钮。

## 重建离线站

```bash
python3 course-site/build_site.py
```

写作规范见 [docs/course/STYLE.md](docs/course/STYLE.md)。
