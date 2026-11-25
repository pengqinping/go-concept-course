package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// 可调参数：备注内容；保持空串观察 omitempty，改成 "vip" 再对比
const noteText = ""

// 可调参数：外部发来的 JSON，带一个结构体里没有的字段
const externalJSON = `{"id":7,"name":"ada","role":"admin"}`

// user 是程序内部的类型世界；struct tag 是它对文本世界的翻译规则。
type user struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
	kind string // 未导出：翻译规则的盲区，永远不出门
}

type stamp struct {
	T time.Time `json:"t"`
}

func main() {
	// 1) Marshal：哪些字段留在了门内
	u := user{ID: 7, Name: "ada", Note: noteText, kind: "internal"}
	b, err := json.Marshal(u)
	if err != nil {
		panic(err)
	}
	fmt.Printf("marshal = %s\n", b)
	fmt.Printf("  kind 不在输出里；note 为零值时被 omitempty 整键省略\n\n")

	// 2) Unmarshal 默认规则：未知字段静默丢弃
	var lenient user
	if err := json.Unmarshal([]byte(externalJSON), &lenient); err != nil {
		panic(err)
	}
	fmt.Printf("lenient unmarshal = %+v\n", lenient)
	fmt.Printf("  role 被丢弃，err = nil（拼写错误同样被吞掉）\n\n")

	// 3) 收紧规则：未知字段算错误
	strict := json.NewDecoder(strings.NewReader(externalJSON))
	strict.DisallowUnknownFields()
	var guarded user
	err = strict.Decode(&guarded)
	fmt.Printf("strict decode err = %v\n\n", err)

	// 4) time.Time 往返：值相等，文本表示是两套
	t := time.Date(2026, 11, 25, 7, 15, 55, 123000000, time.UTC)
	out, err := json.Marshal(stamp{T: t})
	if err != nil {
		panic(err)
	}
	var back stamp
	if err := json.Unmarshal(out, &back); err != nil {
		panic(err)
	}
	fmt.Printf("time round-trip:\n")
	fmt.Printf("  go 文本   = %s\n", t.String())
	fmt.Printf("  json 文本 = %s\n", out)
	fmt.Printf("  back.Equal(t) = %t\n", back.T.Equal(t))
}
