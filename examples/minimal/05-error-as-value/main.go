package main

import (
	"errors"
	"fmt"
)

// 可调参数
const (
	// 要查找的 ID；与库里唯一的 7 不同，保证走失败路径
	targetID = 404
	// 最外层附加的上下文
	layerTag = "handler"
)

var ErrNotFound = errors.New("not found")

// LookupError 携带结构化字段，模拟标准库里 *os.PathError 那样的具体错误。
type LookupError struct {
	ID    int
	Kind  string
	cause error
}

func (e *LookupError) Error() string {
	return fmt.Sprintf("lookup id=%d kind=%s: %v", e.ID, e.Kind, e.cause)
}

func (e *LookupError) Unwrap() error { return e.cause }

// tracingCloser 是不碰文件系统的资源替身，只记录 Close 是否发生。
type tracingCloser struct {
	closed bool
}

func (c *tracingCloser) Close() error {
	c.closed = true
	fmt.Println("defer: Close() 被调用，资源已释放")
	return nil
}

// fetch 是最底层：把"没找到"同时表达为哨兵和结构化错误。
func fetch(id int) (string, error) {
	if id != 7 {
		return "", fmt.Errorf("id=%d: %w", id,
			&LookupError{ID: id, Kind: "missing", cause: ErrNotFound})
	}
	return "alice", nil
}

func loadUser(id int) (string, error) {
	rc := &tracingCloser{}
	defer rc.Close()

	name, err := fetch(id)
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}
	return name, nil
}

func main() {
	_, err := loadUser(targetID)
	if err == nil {
		return
	}
	err = fmt.Errorf("%s: %w", layerTag, err)

	fmt.Printf("错误文本：%v\n", err)
	fmt.Printf("errors.Is(err, ErrNotFound) = %v\n", errors.Is(err, ErrNotFound))

	var le *LookupError
	fmt.Printf("errors.As(err, &le)         = %v\n", errors.As(err, &le))
	if errors.As(err, &le) {
		fmt.Printf("取回的结构：ID=%d Kind=%s\n", le.ID, le.Kind)
	}

	broken := fmt.Errorf("id=%d: %v", targetID, ErrNotFound)
	fmt.Printf("用 %%v 包装：Is=%v（文本=%q）\n", errors.Is(broken, ErrNotFound), broken.Error())
}
