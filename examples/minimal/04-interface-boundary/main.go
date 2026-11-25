package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// 可调参数
const (
	// 传给最小 Reader 的内容
	inputText = "go interface"
	// 每次 Read 的缓冲区大小；至少为 1
	maxRead = 4
)

// Reader 是调用方定义的最小接口：只要一个方法。
type Reader interface {
	Read(p []byte) (int, error)
}

func readAll(r Reader) (string, error) {
	var out strings.Builder
	buf := make([]byte, maxRead)
	for {
		n, err := r.Read(buf)
		out.Write(buf[:n])
		if err == io.EOF {
			return out.String(), nil
		}
		if err != nil {
			return out.String(), err
		}
	}
}

func main() {
	sr := strings.NewReader(inputText)
	got, err := readAll(sr)
	fmt.Printf("strings.Reader → %q  err=%v\n", got, err)

	bb := bytes.NewBufferString(inputText)
	got, err = readAll(bb)
	fmt.Printf("bytes.Buffer   → %q  err=%v\n", got, err)

	// typed nil：这里只判定，不调用 Read（调用会 panic）
	var nilBuf *bytes.Buffer
	var boxed Reader = nilBuf
	fmt.Printf("nilBuf == nil  ? %v\n", nilBuf == nil)
	fmt.Printf("boxed  == nil  ? %v\n", boxed == nil)
	fmt.Printf("boxed 的动态类型：%T\n", boxed)

	// 对照：真正的 nil 接口
	var empty Reader
	fmt.Printf("empty  == nil  ? %v\n", empty == nil)
}
