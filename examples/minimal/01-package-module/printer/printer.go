package printer

import "fmt"

// Print 是 printer 唯一的导出入口：外部只能通过它使用本包的能力。
func Print(word string, edge int) string {
	return decorate(word, "=", edge)
}

// decorate 未导出：包外的 main 无法直接调用它，只能经 Print 间接使用。
func decorate(word, deco string, edge int) string {
	frame := ""
	for i := 0; i < edge; i++ {
		frame += deco
	}
	return fmt.Sprintf("%s %s %s", frame, word, frame)
}
