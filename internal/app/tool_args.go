package app

import "penetration-testing-ai/internal/mcp"

// textResult 构造一个文本型 MCP 工具结果。
func textResult(msg string, isErr bool) *mcp.ToolResult {
	return &mcp.ToolResult{
		Content: []mcp.Content{{Type: "text", Text: msg}},
		IsError: isErr,
	}
}

// strArg 从 MCP 工具入参中读取字符串字段，缺失或类型不符时返回空串。
func strArg(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

// boolArg 从 MCP 工具入参中读取布尔字段，缺失或类型不符时返回 false。
func boolArg(args map[string]interface{}, key string) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return false
}

// intArg 从 MCP 工具入参中读取整数，缺失或类型不符时返回默认值。
func intArg(args map[string]interface{}, key string, def int) int {
	switch v := args[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return def
}
