package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// 可调参数：ID 允许的最大数字位数
const maxIDLen = 6

// 可调参数：是否逐条打印用例细节
const showDetails = true

var (
	ErrEmpty    = errors.New("id is empty")
	ErrNonDigit = errors.New("id contains non-digit")
	ErrTooLong  = errors.New("id too long")
)

// ParseID 去掉首尾空白后，要求内容全是数字且位数不超过 maxIDLen。
func ParseID(raw string) (int, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, ErrEmpty
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, ErrNonDigit
		}
	}
	if len(s) > maxIDLen {
		return 0, ErrTooLong
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return n, nil
}

type testCase struct {
	name    string
	input   string
	want    int
	wantErr error
}

var cases = []testCase{
	{name: "纯数字", input: "42", want: 42},
	{name: "首尾空白", input: "  7  ", want: 7},
	{name: "前导零", input: "007", want: 7},
	{name: "空串", input: "   ", wantErr: ErrEmpty},
	{name: "含字母", input: "4a2", wantErr: ErrNonDigit},
	{name: "超长", input: "1234567", wantErr: ErrTooLong},
}

func main() {
	fmt.Printf("ParseID 表驱动自检（maxIDLen=%d）\n", maxIDLen)
	passed, failed := 0, 0
	for _, tc := range cases {
		got, err := ParseID(tc.input)
		ok := errors.Is(err, tc.wantErr) && (tc.wantErr != nil || got == tc.want)
		status := "FAIL"
		if ok {
			status = "PASS"
			passed++
		} else {
			failed++
		}
		if showDetails {
			fmt.Printf("[%s] %s | 输入=%q 期望=(%d, %v) 实际=(%d, %v)\n",
				status, tc.name, tc.input, tc.want, tc.wantErr, got, err)
		} else {
			fmt.Printf("[%s] %s\n", status, tc.name)
		}
	}
	fmt.Printf("合计：PASS=%d FAIL=%d\n", passed, failed)
}
