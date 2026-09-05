package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	// 管理员名称会参与卡密表名和日志文件名的构造，因此必须使用严格白名单。
	管理员名称规则 = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)
	// 设备标识由客户端生成并持久化，允许常见 UUID、字母数字和少量分隔符。
	// 最短8位可以过滤明显的空值或占位字符串，最长128位避免把大字段带入索引。
	设备标识规则 = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{7,127}$`)
	// 卡密只允许能安全用于查询、日志和文本导出的 ASCII 字符。
	卡密格式规则 = regexp.MustCompile(`^[A-Za-z0-9_-]{7,63}$`)
)

// 验证管理员名称兼顾数据库表名、日志路径和前端展示的安全性。
func 验证管理员名称(name string) bool {
	return 管理员名称规则.MatchString(name)
}

// 卡密数据表名供新模块和敏感批量操作生成动态表名，拒绝把不可信名称带入 SQL 标识符。
func 卡密数据表名(name string) (string, error) {
	if !验证管理员名称(name) {
		return "", fmt.Errorf("管理员名称格式不正确")
	}
	return "card_" + name, nil
}

// 规范化设备标识统一大小写和首尾空格，保证同一设备不会因为输入格式
// 轻微不同而产生多个会话或多次扣费记录。
func 规范化设备标识(deviceID string) (string, bool) {
	deviceID = strings.ToLower(strings.TrimSpace(deviceID))
	return deviceID, 设备标识规则.MatchString(deviceID)
}

// 规范化可选设备标识用于允许省略 device_id 的卡端接口。省略和显式传入
// 空字符串具有完全相同的含义；非空值仍执行严格格式校验，不能绕过设备标识规则。
func 规范化可选设备标识(deviceID string) (string, bool) {
	if strings.TrimSpace(deviceID) == "" {
		return "", true
	}
	return 规范化设备标识(deviceID)
}

// 规范化可显示文本集中处理会出现在管理页面、用户页面或日志中的短文本。
// maxRunes 按字符数而非 UTF-8 字节数限制，和 MySQL varchar 的长度语义一致；
// 控制字符会破坏表格布局或伪造日志行，因此不允许写入。
func 规范化可显示文本(value string, maxRunes int) (string, bool) {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 || len([]rune(value)) > maxRunes {
		return "", false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return value, true
}

// 规范化多行文本用于软件公告等允许换行的公开内容。换行、回车和制表符
// 保留，其余控制字符一律拒绝；统一去掉首尾空白，避免公告页面出现空行噪声。
func 规范化多行文本(value string, maxRunes int) (string, bool) {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 || len([]rune(value)) > maxRunes {
		return "", false
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return "", false
		}
	}
	return value, true
}

// 规范化设备别名只做展示字段需要的清理，不要求别名唯一。
// 拒绝控制字符是为了避免流水页面换行伪造或破坏日志/表格布局。
func 规范化设备别名(alias string) (string, bool) {
	return 规范化可显示文本(alias, 64)
}
