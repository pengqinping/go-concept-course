package main

import "fmt"

// 可调参数
const (
	baseLen   = 3 // 初始元素个数
	extraRoom = 1 // 第一组预留的额外容量；改成 0 会让第一组 append 也扩容
)

func show(name string, s []int) {
	fmt.Printf("%-10s %v  len=%d cap=%d\n", name, s, len(s), cap(s))
}

func filled(n, capHint int) []int {
	s := make([]int, n, capHint)
	for i := range s {
		s[i] = i + 1
	}
	return s
}

func main() {
	// ① struct：值就是整块数据，复制后互不相干
	type Point struct{ X, Y int }
	p := Point{X: 1, Y: 2}
	q := p
	q.X = 99
	fmt.Printf("struct     p=%+v  q=%+v   （改 q 不影响 p）\n", p, q)

	// ② array：和 struct 一样是整块值
	arr := [4]int{1, 2, 3, 4}
	arr2 := arr
	arr2[0] = 99
	fmt.Printf("array      arr=%v  arr2=%v （改 arr2 不影响 arr）\n", arr, arr2)

	// ③ slice：复制的是描述符，两个变量指向同一底层数组
	s := filled(baseLen, baseLen+extraRoom)
	show("s 初始", s)
	t := s
	t[0] = 99
	show("s 改后", s)
	show("t 改后", t)

	// ④ append：cap 够 -> 仍共享；cap 不够 -> 新数组，从此分离
	shared := filled(baseLen, baseLen+extraRoom)
	shared2 := append(shared, 100)
	shared2[0] = 77
	fmt.Println("--- append 有空位 ---")
	show("shared", shared)
	show("shared2", shared2)
	fmt.Printf("shared[0]==77 ? %v\n", shared[0] == 77)

	separate := filled(baseLen, baseLen)
	separate2 := append(separate, 100)
	separate2[0] = 77
	fmt.Println("--- append 触发扩容 ---")
	show("separate", separate)
	show("separate2", separate2)
	fmt.Printf("separate[0]==77 ? %v\n", separate[0] == 77)
}
