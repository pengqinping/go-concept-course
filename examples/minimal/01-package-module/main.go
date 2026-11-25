// 这个 demo 演示三件事：包的可见性边界、模块路径如何拼出 import、
// go:embed 如何把文件变成编译期输入。输出确定，可逐行核对。
package main

import (
	_ "embed"
	"fmt"

	"example.com/go-course/01-package-module/printer"
)

//go:embed banner.txt
var banner string

// 可调参数
const (
	word = "Go" // 传给 printer.Print 的词
	edge = 2    // 装饰字符的重复次数，改成 4 看边框变宽
)

func main() {
	fmt.Printf("banner=%q  bytes=%d\n", banner, len(banner))
	fmt.Println("Print :", printer.Print(word, edge))

	// 取消下一行注释再运行：decorate 未导出，编译直接失败——
	// 可见性不是风格建议，是编译器执行的边界。
	// fmt.Println(printer.decorate(word, "=", edge))
}
