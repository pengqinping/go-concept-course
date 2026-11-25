package main

import (
	"fmt"
	"runtime"
)

// 可调参数：构造多少个逃逸到堆的大对象
const escapedObjects = 2000

// 可调参数：闭包计数器递增几次
const closureSteps = 3

// block 故意做大一点，让 GC 前后的 HeapAlloc 差异盖过噪声。
type block struct {
	data [64]byte
}

// newValue 返回局部变量的地址：x 必须活过本函数，编译器把它挪到堆。
func newValue() *int {
	x := 42
	return &x
}

// newBlock 同样返回堆对象的指针，用来观察分配与回收。
func newBlock() *block {
	b := block{}
	b.data[0] = 1
	return &b
}

// counter 返回一个闭包：count 被闭包环境捕获，函数返回后仍须存活。
func counter() func() int {
	count := 0
	return func() int {
		count++
		return count
	}
}

func stats() (uint64, uint64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Mallocs, m.HeapAlloc
}

func main() {
	// 现象一：局部变量地址被返回后，数据仍然有效。
	p := newValue()
	fmt.Printf("返回局部变量指针：*p=%d\n", *p)

	// 现象二：闭包捕获的变量在外层函数返回后继续存活。
	next := counter()
	for i := 1; i <= closureSteps; i++ {
		fmt.Printf("闭包第 %d 次调用 = %d\n", i, next())
	}

	// 验证一：构造一批逃逸对象，累计分配次数只增不减。
	beforeMallocs, _ := stats()
	keep := make([]*block, 0, escapedObjects)
	for i := 0; i < escapedObjects; i++ {
		keep = append(keep, newBlock())
	}
	afterMallocs, _ := stats()
	fmt.Printf("构造 %d 个逃逸对象：Mallocs 增加 = %t\n", escapedObjects, afterMallocs > beforeMallocs)
	fmt.Printf("  累计分配次数：%d -> %d\n", beforeMallocs, afterMallocs)

	// 验证二：断开引用后这些对象不可达，GC 才可能回收它们。
	_, heapBeforeGC := stats()
	keep = nil
	runtime.GC()
	_, heapAfterGC := stats()
	fmt.Printf("断开引用并 GC：HeapAlloc 下降 = %t\n", heapAfterGC < heapBeforeGC)
	fmt.Printf("  HeapAlloc：%d -> %d（趋势，非精确字节）\n", heapBeforeGC, heapAfterGC)

	// 仍被变量引用的对象，GC 动不了它。
	live := newValue()
	runtime.GC()
	fmt.Printf("GC 后仍可达的 live：*live=%d\n", *live)
}
