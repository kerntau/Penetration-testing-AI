package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"penetration-testing-ai/internal/agent"
	"penetration-testing-ai/internal/database"
	"penetration-testing-ai/internal/mcp/builtin"
	"penetration-testing-ai/internal/multiagent"

	"go.uber.org/zap"
)

// agentSessionPrepared 单代理请求在调用 Eino 前的会话与消息准备结果。
type agentSessionPrepared struct {
	ConversationID     string
	CreatedNew         bool
	History            []agent.ChatMessage
	FinalMessage       string
	RoleTools          []string
	AssistantMessageID string
	UserMessageID      string
}

// prepareAgentSession 为单代理对话准备会话、历史、角色提示词与消息占位。
func (h *AgentHandler) prepareAgentSession(req *ChatRequest, source string) (*agentSessionPrepared, error) {
	conversationID := strings.TrimSpace(req.ConversationID)
	createdNew := false
	if conversationID == "" {
		title := safeTruncateString(req.Message, 50)
		meta := database.ConversationCreateMeta{
			Source:    source,
			RoleName:  req.Role,
			AgentMode: "eino_single",
		}
		conv, err := h.db.CreateConversation(title, meta)
		if err != nil {
			return nil, fmt.Errorf("创建对话失败: %w", err)
		}
		conversationID = conv.ID
		createdNew = true
	} else if _, err := h.db.GetConversation(conversationID); err != nil {
		return nil, fmt.Errorf("对话不存在")
	}
	if h.tasks != nil && h.tasks.GetTask(conversationID) != nil {
		return nil, ErrTaskAlreadyRunning
	}
	if err := h.db.SetConversationRoleName(conversationID, req.Role); err != nil {
		h.logger.Warn("更新对话角色失败", zap.String("conversationId", conversationID), zap.String("role", req.Role), zap.Error(err))
	}
	if err := h.db.SetConversationAgentMode(conversationID, "eino_single"); err != nil {
		h.logger.Warn("更新对话模式失败", zap.String("conversationId", conversationID), zap.Error(err))
	}

	agentHistoryMessages, err := h.loadHistoryFromAgentTrace(conversationID)
	if err != nil {
		historyMessages, getErr := h.db.GetMessages(conversationID)
		if getErr != nil {
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			agentHistoryMessages = dbMessagesToAgentChatMessages(historyMessages)
		}
	}

	resolvedMessage, resolveErr := h.db.ResolveConversationPlaceholders(conversationID, req.Message)
	if resolveErr != nil {
		return nil, fmt.Errorf("解析对话占位符失败: %w", resolveErr)
	}
	req.Message = resolvedMessage
	finalMessage := req.Message
	var roleTools []string
	if req.Role != "" && req.Role != "默认" && h.config != nil && h.config.Roles != nil {
		if role, exists := h.config.Roles[req.Role]; exists && role.Enabled {
			if role.UserPrompt != "" {
				finalMessage = role.UserPrompt + "\n\n" + req.Message
			}
			roleTools = role.Tools
		}
	}

	retest, retestErr := h.db.GetVulnerabilityRetest(conversationID)
	if retestErr != nil {
		return nil, fmt.Errorf("读取复测上下文失败: %w", retestErr)
	}
	if retest != nil && len(roleTools) > 0 {
		// 复测需要这两个工具，即使继承的角色工具集较旧；MCP 侧仍会校验权限。
		roleTools = append(append([]string(nil), roleTools...), builtin.ToolGetVulnerability, builtin.ToolUpdateVulnerabilityStatus)
	}

	finalMessage, resolveErr = h.db.ResolveConversationPlaceholders(conversationID, finalMessage)
	if resolveErr != nil {
		return nil, fmt.Errorf("解析提示词占位符失败: %w", resolveErr)
	}

	userMsgRow, uerr := h.db.AddMessage(conversationID, "user", req.Message, nil)
	if uerr != nil {
		h.logger.Error("保存用户消息失败", zap.Error(uerr))
		return nil, fmt.Errorf("保存用户消息失败: %w", uerr)
	}
	if retest != nil {
		if err := h.db.SetVulnerabilityRetestPrompt(conversationID, ""); err != nil {
			h.logger.Warn("清除已发送复测提示词失败", zap.Error(err))
		}
	}
	userMessageID := ""
	if userMsgRow != nil {
		userMessageID = userMsgRow.ID
	}

	assistantMsg, aerr := h.db.AddMessage(conversationID, "assistant", "处理中...", nil)
	var assistantMessageID string
	if aerr != nil {
		h.logger.Warn("创建助手消息占位失败", zap.Error(aerr))
	} else if assistantMsg != nil {
		assistantMessageID = assistantMsg.ID
	}

	return &agentSessionPrepared{
		ConversationID:     conversationID,
		CreatedNew:         createdNew,
		History:            agentHistoryMessages,
		FinalMessage:       finalMessage,
		RoleTools:          roleTools,
		AssistantMessageID: assistantMessageID,
		UserMessageID:      userMessageID,
	}, nil
}

// agentSessionHTTPErrorStatus 将会话准备错误映射为 HTTP 状态码与提示文案。
func agentSessionHTTPErrorStatus(err error) (int, string) {
	msg := err.Error()
	switch {
	case errors.Is(err, ErrTaskAlreadyRunning):
		return http.StatusConflict, msg
	case strings.Contains(msg, "对话不存在"):
		return http.StatusNotFound, msg
	case strings.Contains(msg, "创建对话失败"), strings.Contains(msg, "保存用户消息失败"):
		return http.StatusInternalServerError, msg
	default:
		return http.StatusBadRequest, msg
	}
}

// persistEinoAgentTraceForResume 在 Eino 运行异常结束时写入代理轨迹（库列 last_react_*），供下一请求 loadHistoryFromAgentTrace 软续跑。
func (h *AgentHandler) persistEinoAgentTraceForResume(conversationID string, result *multiagent.RunResult) {
	if h == nil || result == nil {
		return
	}
	if result.LastAgentTraceInput == "" && result.LastAgentTraceOutput == "" {
		return
	}
	if err := h.db.SaveAgentTrace(conversationID, result.LastAgentTraceInput, result.LastAgentTraceOutput); err != nil {
		h.logger.Warn("保存 Eino 续跑上下文失败", zap.String("conversationId", conversationID), zap.Error(err))
	}
}

// mergeMCPExecutionIDLists 去重合并多段 Run 的 MCP execution id（顺序：先 dst 后 more）。
func mergeMCPExecutionIDLists(dst []string, more []string) []string {
	seen := make(map[string]struct{}, len(dst)+len(more))
	out := make([]string, 0, len(dst)+len(more))
	add := func(ids []string) {
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	add(dst)
	add(more)
	return out
}

// interruptContinueTimelineSummary 时间线 / process_details 中展示的简短正文。
func interruptContinueTimelineSummary(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return "用户选择「中断并继续」，未填写说明；已按默认模板合并上下文并续跑。"
	}
	return "用户中断说明（原文）：\n\n" + note
}

// formatInterruptContinueUserMessage 将「中断并继续」弹窗中的说明格式化为新一轮 user 消息。
func formatInterruptContinueUserMessage(note string) string {
	var b strings.Builder
	b.WriteString("【用户补充 / 中断后继续】\n")
	if s := strings.TrimSpace(note); s != "" {
		b.WriteString(s)
		b.WriteString("\n\n")
	}
	b.WriteString("【请在本轮落实】\n")
	b.WriteString("- 将用户提供的接口路径、参数、业务变化纳入后续测试与推理。\n")
	b.WriteString("- 若资产或目标信息有更新，请对目标重新执行端口/服务探测，再基于新结果规划下一步。\n")
	b.WriteString("- 在已有轨迹基础上推进，避免无意义重复已完成的步骤。\n")
	return strings.TrimSpace(b.String())
}
