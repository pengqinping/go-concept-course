package main

import (
	"fmt"
	"strconv"
)

// 可调参数：演示时取输入列表的前几个元素
const sampleSize = 3

// 可调参数：是否打印 any 版本作对照
const showAnyVersion = true

// Map 把 []In 逐项转换成 []Out，函数体只做遍历、调用 f、写入切片。
func Map[In, Out any](in []In, f func(In) Out) []Out {
	out := make([]Out, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

// mapAny 放弃了类型信息：结果只能当作 []any，取用时要自己断言。
func mapAny(in []any, f func(any) any) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

func main() {
	nums := []int{1, 2, 3, 4, 5}
	n := sampleSize
	if n < 1 {
		n = 1
	}
	if n > len(nums) {
		n = len(nums)
	}
	in := nums[:n]

	labels := Map(in, strconv.Itoa) // In/Out 由参数推断
	var _ []string = labels         // 编译期就确认结果类型
	squares := Map[int, int](in, func(x int) int { return x * x })

	fmt.Printf("输入：%v (%T)\n", in, in)
	fmt.Printf("泛型 Map(strconv.Itoa)：%v (%T)，首项直接是 string：%q\n", labels, labels, labels[0])
	fmt.Printf("泛型 Map(平方，显式实例化)：%v (%T)\n", squares, squares)

	if showAnyVersion {
		anyIn := make([]any, n)
		for i := range in {
			anyIn[i] = in[i]
		}
		anyOut := mapAny(anyIn, func(v any) any {
			return strconv.Itoa(v.(int))
		})
		fmt.Printf("any 版：%v (%T)，首项必须断言：%q\n", anyOut, anyOut, anyOut[0].(string))
	}
}
