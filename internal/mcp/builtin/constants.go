package builtin

// 内置工具名称常量。
// 所有代码中使用内置工具名称的地方都应该使用这些常量，而不是硬编码字符串。
const (
	// 漏洞管理工具
	ToolRecordVulnerability       = "record_vulnerability"
	ToolListVulnerabilities       = "list_vulnerabilities"
	ToolGetVulnerability          = "get_vulnerability"
	ToolUpdateVulnerabilityStatus = "update_vulnerability_status"

	// 长耗时工具执行控制（后台 execution 查询/等待/取消）
	ToolGetToolExecution    = "get_tool_execution"
	ToolWaitToolExecution   = "wait_tool_execution"
	ToolCancelToolExecution = "cancel_tool_execution"
)

// GetAllBuiltinTools 返回所有内置工具名称列表。
func GetAllBuiltinTools() []string {
	return []string{
		ToolRecordVulnerability,
		ToolListVulnerabilities,
		ToolGetVulnerability,
		ToolUpdateVulnerabilityStatus,
		ToolGetToolExecution,
		ToolWaitToolExecution,
		ToolCancelToolExecution,
	}
}
