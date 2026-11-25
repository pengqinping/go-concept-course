// 这个 demo 演示字符串的两个视图（字节 / 码点）与 Map 的三种状态。
// 输出确定，可逐行核对。
package main

import (
	"fmt"
	"sort"
)

// 可调参数
const (
	text     = "Go语言" // 可调参数：换成任意字符串，观察 len 与 range 的差别
	sizeHint = 2      // 可调参数：make(map, n) 的容量提示，不影响正确性
)

func main() {
	// ① 字符串：len 数字节，码点数要转 []rune
	fmt.Printf("text=%q len=%d 码点=%d\n", text, len(text), len([]rune(text)))

	// ② range 按码点走，但下标是字节偏移
	fmt.Print("range 字节偏移:")
	for i := range text {
		fmt.Printf(" %d", i)
	}
	fmt.Println()

	// ③ 两种转换：[]byte 拿原始字节，[]rune 拿码点序列；改副本不动原串
	b := []byte(text)
	r := []rune(text)
	fmt.Printf("[]byte len=%d  []rune len=%d\n", len(b), len(r))
	b[0] = 'g'
	fmt.Printf("改 []byte 后原串不变: %s\n", text)

	// ④ Map：读零值安全，comma-ok 区分"不存在"与"值为零"
	m := make(map[string]int, sizeHint)
	zero := m["missing"]
	_, ok := m["missing"]
	fmt.Printf("缺失键: 值=%d ok=%v（读永远安全）\n", zero, ok)

	var nilMap map[string]int
	fmt.Printf("nil map 读: 值=%d（写才会 panic，demo 不演示）\n", nilMap["x"])

	// ⑤ 写入、读改写、delete（删除不存在的键也安全）
	m["go"] = 2
	m["zh"] = 1
	m["go"]++
	v, ok2 := m["go"]
	fmt.Printf("写入并自增: go=%d ok=%v\n", v, ok2)
	delete(m, "go")
	delete(m, "not-exist")
	_, ok3 := m["go"]
	fmt.Printf("delete 后: go ok=%v\n", ok3)

	// ⑥ 遍历顺序不可依赖：先收集键、排序后再输出
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Print("排序输出:")
	for _, k := range keys {
		fmt.Printf(" %s=%d", k, m[k])
	}
	fmt.Println()
}
