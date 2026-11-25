package main

import "fmt"

// 可调参数：发送几条消息
const repeat = 2

// 可调参数：最外层装饰器加的前缀
const outerPrefix = "[app] "

// Logger 是小接口：只描述"能写一条日志"这一项能力。
type Logger interface {
	Log(msg string)
}

// consoleLogger 是最小实现；首字母小写意味着它只在包内可见。
type consoleLogger struct {
	name string
}

func (c consoleLogger) Log(msg string) {
	fmt.Printf("  └─ %s 写出：%q\n", c.name, msg)
}

// prefixLogger 通过嵌入 Logger 获得 Log 方法，再在外层重新定义 Log 做装饰。
// 这不是继承：它"拥有"一个 Logger，并决定何时、加什么前缀后调用它。
type prefixLogger struct {
	Logger
	prefix string
	name   string
}

func (p prefixLogger) Log(msg string) {
	fmt.Printf("  ├─ %s 收到：%q\n", p.name, msg)
	p.Logger.Log(p.prefix + msg) // 显式委托给被嵌入的 Logger
}

func main() {
	base := consoleLogger{name: "console"}
	inner := prefixLogger{Logger: base, prefix: "[inner] ", name: "prefix-1"}
	outer := prefixLogger{Logger: inner, prefix: outerPrefix, name: "prefix-2"}

	// 调用方只认 Logger 小接口，看不到具体实现类型。
	var l Logger = outer
	for i := 1; i <= repeat; i++ {
		fmt.Printf("第 %d 条消息：\n", i)
		l.Log(fmt.Sprintf("hello-%d", i))
	}
	fmt.Println("委托链：prefix-2 → prefix-1 → console")
}
