package main

import (
	"fmt"
	"strings"
)

// 可调参数：转账金额
const amount = 300

// 可调参数：查询输入；这个值会把拼接版 SQL 变成另一种语法
const userInput = "' OR '1'='1"

// account 与 store 模拟一张两列的表。
type account struct {
	name    string
	balance int
}

type store struct {
	rows map[string]*account
}

func newStore(initial map[string]int) *store {
	s := &store{rows: map[string]*account{}}
	for name, bal := range initial {
		s.rows[name] = &account{name: name, balance: bal}
	}
	return s
}

// miniTx 是内存版事务：exec 只写草稿区，commit 一次性生效，rollback 整体丢弃。
// 真实项目请用 database/sql + 驱动的事务，边界语义与此一致，草稿区由数据库管理。
type miniTx struct {
	s      *store
	staged map[string]int // 草稿区：名字 → 提交后的余额
	failed error          // 第一步失败后，后续语句不再执行
}

func (s *store) begin() *miniTx {
	return &miniTx{s: s, staged: map[string]int{}}
}

func (tx *miniTx) exec(name string, delta int) error {
	if tx.failed != nil {
		return tx.failed
	}
	row, ok := tx.s.rows[name]
	if !ok {
		tx.failed = fmt.Errorf("account %q not found", name)
		return tx.failed
	}
	tx.staged[name] = row.balance + delta
	return nil
}

func (tx *miniTx) commit() error {
	if tx.failed != nil {
		return tx.failed
	}
	for name, bal := range tx.staged {
		tx.s.rows[name].balance = bal
	}
	return nil
}

func (tx *miniTx) rollback() {
	tx.staged = nil // 草稿整体丢弃，第一步的修改一并撤销
}

// transfer 是典型的两步写：任何一步失败，整体都必须当作没发生。
func transfer(s *store, from, to string, amt int) error {
	tx := s.begin()
	defer tx.rollback() // 惯用法：只承担失败路径的清理；commit 之后它什么也不做
	if err := tx.exec(from, -amt); err != nil {
		return err
	}
	if err := tx.exec(to, amt); err != nil {
		return err
	}
	return tx.commit()
}

func balances(s *store, names ...string) string {
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s=%d", n, s.rows[n].balance))
	}
	return strings.Join(parts, " ")
}

// evalConcat 化装成"按 SQL 语法解析条件文本"的引擎。
// 真实数据库会解析整段文本；这个模拟器只复现最有代表性的一步：
// 拼接后的文本里出现恒真分支时，条件对每一行都成立。
func evalConcat(cond string) func(name string) bool {
	if strings.Contains(cond, " OR ") {
		return func(string) bool { return true } // OR '1'='1' 恒为真
	}
	lit := strings.TrimSuffix(strings.TrimPrefix(cond, "name = '"), "'")
	return func(name string) bool { return name == lit }
}

// evalParam 是参数化版引擎：文本固定为 name = $1，输入永远只是与字段比较的值。
func evalParam(arg string) func(name string) bool {
	return func(name string) bool { return name == arg }
}

func matchRows(names []string, match func(string) bool) []string {
	var matched []string
	for _, n := range names {
		if match(n) {
			matched = append(matched, n)
		}
	}
	return matched
}

func main() {
	// 路径一：两步全成功 → Commit，草稿一次性生效
	s1 := newStore(map[string]int{"alice": 1000, "bob": 500})
	fmt.Printf("commit 路径   转账前: %s\n", balances(s1, "alice", "bob"))
	err := transfer(s1, "alice", "bob", amount)
	fmt.Printf("  err=%v  转账后: %s\n\n", err, balances(s1, "alice", "bob"))

	// 路径二：第二步入账失败 → Rollback，第一步的扣款被撤销
	s2 := newStore(map[string]int{"alice": 1000, "bob": 500})
	fmt.Printf("rollback 路径  转账前: %s\n", balances(s2, "alice", "bob"))
	err = transfer(s2, "alice", "carol", amount)
	fmt.Printf("  err=%v  转账后: %s\n\n", err, balances(s2, "alice", "bob"))

	// 拼接 vs 参数化：同一个输入，两种通道
	names := []string{"alice", "bob", "carol"}
	concatCond := fmt.Sprintf("name = '%s'", userInput)
	fmt.Printf("拼接版   : SELECT * FROM users WHERE %s\n", concatCond)
	concatMatched := matchRows(names, evalConcat(concatCond))
	fmt.Printf("  条件被当作语法解析 -> 匹配 %d/%d 行（数据变成了代码）\n\n",
		len(concatMatched), len(names))

	fmt.Printf("参数化版 : SELECT * FROM users WHERE name = $1\n")
	paramMatched := matchRows(names, evalParam(userInput))
	fmt.Printf("  参数 = %q -> 匹配 %d/%d 行（输入只是普通字符串，数据留在数据侧）\n",
		userInput, len(paramMatched), len(names))
}
