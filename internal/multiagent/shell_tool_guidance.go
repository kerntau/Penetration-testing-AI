package multiagent

import (
	"strings"
)

// shellExecExecuteGuidanceSection exec 与 execute 的工具分工说明。
const shellExecExecuteGuidanceSection = `## exec 与 execute 的分工

- exec：在目标主机上执行一次性命令，适合短时、无交互、结果可一次性返回的操作。
- execute：在受控会话中执行命令，适合需要保持工作目录或环境状态、以及多步连续操作。
- 执行前先确认目标与参数，优先最小必要命令；不要执行会破坏目标可用性的操作，并保留输出作为证据。`

func shellToolsPresent(toolNames []string) bool {
	for _, n := range toolNames {
		switch strings.ToLower(strings.TrimSpace(n)) {
		case "exec", "execute":
			return true
		}
	}
	return false
}

// injectShellToolGuidance 在系统提示末尾追加 exec/execute 分工（仅当工具列表含 exec 或 execute）。
func injectShellToolGuidance(instruction string, toolNames []string) string {
	if !shellToolsPresent(toolNames) {
		return instruction
	}
	block := strings.TrimSpace(shellExecExecuteGuidanceSection)
	if block == "" {
		return instruction
	}
	s := strings.TrimSpace(instruction)
	if s == "" {
		return block
	}
	return s + "\n\n" + block
}
