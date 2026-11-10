package multiagent

import (
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// toolSearchToolName Eino dynamictool 的 tool_search 元工具名称。
const toolSearchToolName = "tool_search"

// IsToolSearchTool 判断工具名是否为 tool_search 元工具。
func IsToolSearchTool(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), toolSearchToolName)
}

// normalizeSingleLeadingSystemMessage 保证消息形状对模型服务端友好：
// 存在 system 上下文时，只有索引 0 处的一条 system 消息。
// 部分严格的 OpenAI 兼容后端（如 qwen/vllm 模板）要求 system 消息只能出现在开头，
// 合并可避免 "System message must be at the beginning" 报错。
func normalizeSingleLeadingSystemMessage(msgs []adk.Message, extraSystem string) []adk.Message {
	extraSystem = strings.TrimSpace(extraSystem)
	if len(msgs) == 0 {
		if extraSystem == "" {
			return msgs
		}
		return []adk.Message{schema.SystemMessage(extraSystem)}
	}

	systemParts := make([]string, 0, 2)
	if extraSystem != "" {
		systemParts = append(systemParts, extraSystem)
	}
	nonSystem := make([]adk.Message, 0, len(msgs))
	var ledgerEntries []string
	ledgerPresent, canonicalLedgerPresent := false, false
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if msg.Role == schema.System {
			entries := trustedUserIntentLedgerEntries(msg)
			if _, hasProvenance := msg.Extra[userIntentLedgerExtraKey]; hasProvenance {
				ledgerEntries, ledgerPresent, canonicalLedgerPresent = entries, true, true
			} else if !canonicalLedgerPresent && len(entries) > 0 {
				ledgerEntries, ledgerPresent = entries, true
			}
			if s := strings.TrimSpace(msg.Content); s != "" {
				systemParts = append(systemParts, s)
			}
			continue
		}
		nonSystem = append(nonSystem, msg)
	}
	if len(systemParts) == 0 {
		return nonSystem
	}
	out := make([]adk.Message, 0, len(nonSystem)+1)
	system := schema.SystemMessage(strings.Join(systemParts, "\n\n"))
	if ledgerPresent {
		system.Extra = map[string]interface{}{userIntentLedgerExtraKey: append([]string(nil), ledgerEntries...)}
	}
	out = append(out, system)
	out = append(out, nonSystem...)
	return out
}
