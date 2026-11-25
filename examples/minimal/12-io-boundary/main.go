package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

const readChunk = 4 // 可调参数：计数层每次最多交出的字节数，改成 8 只改变节奏，不改变总量

// countingReader 在 Read 路径上插一个观察点：字节原样通过，只是记一笔账。
type countingReader struct {
	src   io.Reader
	total int // 累计经过的字节数
	calls int // 累计被调用的次数
}

func (c *countingReader) Read(p []byte) (int, error) {
	if len(p) > readChunk {
		p = p[:readChunk] // 主动短读：合法行为，调用方必须能处理
	}
	n, err := c.src.Read(p)
	c.total += n
	c.calls++
	return n, err
}

// tracingCloser 演示 Close 责任：谁打开，谁负责关闭。
type tracingCloser struct {
	io.Reader
	name string
}

func (t *tracingCloser) Close() error {
	fmt.Printf("[关闭] %s：已关闭\n", t.name)
	return nil
}

func main() {
	fmt.Println("[管道] 源 → 计数 → 分流(Tee) → 汇聚(bytes.Buffer)")

	counter := &countingReader{src: strings.NewReader("hello, 组合出来的 io")}
	side := &bytes.Buffer{}
	tee := io.TeeReader(counter, side) // 分流：流入主路的同时抄送一份副本

	var dst bytes.Buffer
	written, err := io.Copy(&dst, tee)
	fmt.Printf("[复制] written=%d err=%v\n", written, err)
	fmt.Printf("[计数] 经 %d 次 Read，累计 %d 字节，与 written 一致：%v\n",
		counter.calls, counter.total, counter.total == int(written))
	fmt.Printf("[内容] 主路 dst=%q 副本 side=%q\n", dst.String(), side.String())

	fmt.Println("[关闭] Close 的责任：谁打开谁负责")
	src := &tracingCloser{Reader: strings.NewReader("借用中的数据"), name: "source"}
	defer src.Close() // 打开者负责关闭；包装层与 io.Copy 都不会替你调
	n, _ := io.Copy(io.Discard, src)
	fmt.Printf("[关闭] 借用期间读走 %d 字节，关闭仍归打开者\n", n)
}
