package cmd

import "fmt"

// printColor 使用 ANSI 颜色码打印格式化文本。
func printColor(color, format string, args ...any) {
	fmt.Printf("\033[%sm%s\033[0m", color, fmt.Sprintf(format, args...))
}

// printGreen 绿色输出，用于成功提示。
func printGreen(format string, args ...any) { printColor("32", format, args...) }

// printBlue 蓝色输出，用于普通强调或信息标题。
func printBlue(format string, args ...any) { printColor("34", format, args...) }

// printRed 红色输出，用于错误提示。
func printRed(format string, args ...any) { printColor("31", format, args...) }

// printYellow 黄色输出，用于警告或取消提示。
func printYellow(format string, args ...any) { printColor("33", format, args...) }

// printGray 灰色输出，用于不重要或辅助信息。
func printGray(format string, args ...any) { printColor("90", format, args...) }

// printLightGray 亮灰色输出，比 printGray 更淡，用于已完成但不强调的辅助步骤。
func printLightGray(format string, args ...any) { printColor("37", format, args...) }
