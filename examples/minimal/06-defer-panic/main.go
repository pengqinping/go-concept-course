// 这个 demo 演示 defer 的三件事：LIFO、参数求值时机、修改命名返回值；
// 以及 panic 如何沿返回路径被 defer 里的 recover 接住。输出确定，可逐行核对。
package main

import "fmt"

// 可调参数
const (
	deferValue = 42     // 可调参数：第一个 defer 捕获的初始值
	boomMsg    = "boom" // 可调参数：panic 携带的内容，改它看 recover 打印什么
)

// evalTiming：两个 defer，一个拷贝参数、一个闭包读变量。
func evalTiming() {
	i := deferValue
	defer fmt.Printf("值拷贝:   %d\n", i) // 实参在这一行就求值并固定
	i = deferValue * 2
	defer func() { fmt.Printf("闭包读取: %d\n", i) }() // 执行时才读 i
}

// namedReturn：defer 修改命名返回值。
func namedReturn() (result string) {
	defer func() { result += "（defer 追加）" }()
	return "正常返回"
}

// safeCall：panic 沿返回路径展开，recover 在 defer 里接住。
func safeCall() (msg string) {
	defer fmt.Println("先注册的 defer 也会执行")
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprintf("接住 panic: %v", r)
		}
	}()
	panic(boomMsg)
}

func main() {
	evalTiming()
	fmt.Println("named return:", namedReturn())
	fmt.Println("safeCall 返回:", safeCall())
	fmt.Println("main 继续运行")
}
