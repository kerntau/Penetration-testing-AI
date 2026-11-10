package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"penetration-testing-ai/internal/agent"
	"penetration-testing-ai/internal/config"
	"penetration-testing-ai/internal/database"
	"penetration-testing-ai/internal/multiagent"
	"penetration-testing-ai/internal/openai"
	"penetration-testing-ai/internal/reasoning"
	"penetration-testing-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// safeTruncateString 安全截断字符串，避免在 UTF-8 字符中间截断
func safeTruncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}

	// 将字符串转换为 rune 切片以正确计算字符数
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	// 截断到最大长度
	truncated := string(runes[:maxLen])

	// 尝试在标点符号或空格处截断，使截断更自然
	// 在截断点往前查找合适的断点（不超过20%的长度）
	searchRange := maxLen / 5
	if searchRange > maxLen {
		searchRange = maxLen
	}
	breakChars := []rune("，。、 ,.;:!?！？/\\-_")
	bestBreakPos := len(runes[:maxLen])

	for i := bestBreakPos - 1; i >= bestBreakPos-searchRange && i >= 0; i-- {
		for _, breakChar := range breakChars {
			if runes[i] == breakChar {
				bestBreakPos = i + 1 // 在标点符号后断开
				goto found
			}
		}
	}

found:
	truncated = string(runes[:bestBreakPos])
	return truncated + "..."
}

// responsePlanAgg buffers main-assistant response_stream chunks for one "planning" process_detail row.
type responsePlanAgg struct {
	meta            map[string]interface{}
	b               strings.Builder
	detailID        string
	lastPersistAt   time.Time
	lastPersistSize int
}

// thinkingBuf aggregates thinking_stream_* / reasoning_chain_stream_* before flush to process_details.
type thinkingBuf struct {
	b         strings.Builder
	meta      map[string]interface{}
	persistAs string // "thinking" | "reasoning_chain"
}

func normalizeProcessDetailText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}

// discardPlanningIfEchoesToolResult drops buffered planning text when it only repeats the
// upcoming tool_result body. Streaming models often echo tool stdout in chunk.Content; flushing
// that into "planning" before persisting tool_result duplicates the output after page refresh.
// sameResponseStreamMeta 判断是否为同一段主通道流（Eino ADK 可能对同一 MessageStream 重复发 response_start）。
func sameResponseStreamMeta(a, b map[string]interface{}) bool {
	if a == nil || b == nil {
		return false
	}
	agentA, _ := a["einoAgent"].(string)
	agentB, _ := b["einoAgent"].(string)
	agentA = strings.TrimSpace(agentA)
	agentB = strings.TrimSpace(agentB)
	if agentA == "" || !strings.EqualFold(agentA, agentB) {
		return false
	}
	orchA, _ := a["orchestration"].(string)
	orchB, _ := b["orchestration"].(string)
	if strings.TrimSpace(orchA) != strings.TrimSpace(orchB) {
		return false
	}
	iterA := responseStreamIterationFromMeta(a)
	iterB := responseStreamIterationFromMeta(b)
	if iterA != 0 && iterB != 0 && iterA != iterB {
		return false
	}
	streamA, _ := a["streamId"].(string)
	streamB, _ := b["streamId"].(string)
	streamA = strings.TrimSpace(streamA)
	streamB = strings.TrimSpace(streamB)
	if streamA != "" && streamB != "" && streamA != streamB {
		return false
	}
	return true
}

func responseStreamIterationFromMeta(m map[string]interface{}) int {
	if m == nil {
		return 0
	}
	switch v := m["iteration"].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func discardPlanningIfEchoesToolResult(respPlan *responsePlanAgg, toolData interface{}) string {
	if respPlan == nil {
		return ""
	}
	plan := normalizeProcessDetailText(respPlan.b.String())
	if plan == "" {
		return ""
	}
	dataMap, ok := toolData.(map[string]interface{})
	if !ok {
		return ""
	}
	res, ok := dataMap["result"].(string)
	if !ok {
		return ""
	}
	r := normalizeProcessDetailText(res)
	if r == "" {
		return ""
	}
	if plan == r || strings.HasSuffix(plan, r) {
		detailID := respPlan.detailID
		respPlan.meta = nil
		respPlan.b.Reset()
		respPlan.detailID = ""
		respPlan.lastPersistAt = time.Time{}
		respPlan.lastPersistSize = 0
		return detailID
	}
	return ""
}

// AgentHandler Agent处理器
type AgentHandler struct {
	asyncWorkers  sync.WaitGroup
	asyncSubmitMu sync.Mutex
	agent         *agent.Agent
	db            *database.DB
	logger        *zap.Logger
	tasks         *AgentTaskManager
	taskEventBus  *TaskEventBus  // 镜像 SSE 事件，供刷新后订阅同一运行中任务
	config        *config.Config // 配置引用，用于获取角色信息
}

// TaskManager 返回 Agent 任务管理器（供 MCP 监控页终止 Eino execute 等）。
func (h *AgentHandler) TaskManager() *AgentTaskManager {
	if h == nil {
		return nil
	}
	return h.tasks
}

// CancelRunningTaskForConversation stops any in-flight agent work for the conversation (idempotent).
func (h *AgentHandler) CancelRunningTaskForConversation(conversationID string) {
	if h == nil || conversationID == "" || h.tasks == nil {
		return
	}
	ok, err := h.tasks.CancelTask(conversationID, ErrTaskCancelled)
	if !ok {
		h.cancelRunningMCPToolsForConversation(conversationID)
		h.tasks.AbortActiveEinoExecute(conversationID, "")
	}
	if h.logger != nil {
		if err != nil {
			h.logger.Warn("取消会话运行中任务失败", zap.String("conversationId", conversationID), zap.Error(err))
		} else if ok {
			h.logger.Info("已取消会话运行中任务", zap.String("conversationId", conversationID))
		}
	}

}

// ConversationTaskRuntimeState exposes the authoritative live state and start
// time used to scope persisted TaskCreate files to the current run. A task
// already entering cancellation must stop driving progress UI immediately.
func (h *AgentHandler) ConversationTaskRuntimeState(conversationID string) (bool, time.Time) {
	if h == nil || h.tasks == nil || strings.TrimSpace(conversationID) == "" {
		return false, time.Time{}
	}
	task := h.tasks.GetTaskSnapshot(strings.TrimSpace(conversationID))
	if task == nil || !strings.EqualFold(strings.TrimSpace(task.Status), "running") {
		return false, time.Time{}
	}
	return true, task.StartedAt
}

func (h *AgentHandler) cancelRunningMCPToolsForConversation(conversationID string) {
	if h == nil || h.agent == nil {
		return
	}
	n := h.agent.CancelRunningMCPToolsForConversation(conversationID, "会话已结束，自动终止仍在运行的工具")
	if n > 0 && h.logger != nil {
		h.logger.Info("已终止会话仍在运行的 MCP 工具", zap.String("conversationId", conversationID), zap.Int("count", n))
	}
}

// NewAgentHandler 创建新的Agent处理器
func NewAgentHandler(agent *agent.Agent, db *database.DB, cfg *config.Config, logger *zap.Logger) *AgentHandler {
	bus := NewTaskEventBus()
	tm := NewAgentTaskManager()
	tm.SetTaskEventBus(bus)
	handler := &AgentHandler{
		agent:        agent,
		db:           db,
		logger:       logger,
		tasks:        tm,
		taskEventBus: bus,
		config:       cfg,
	}
	tm.SetToolCanceler(handler.cancelRunningMCPToolsForConversation)
	return handler
}

// ChatReasoningRequest 对话页「模型推理」意图（Eino 单代理路径消费）。
type ChatReasoningRequest struct {
	// Mode: default（跟随系统）| off | on | auto
	Mode string `json:"mode,omitempty"`
	// Effort: low | medium | high | max | xhigh（原样下发；不同网关最高档命名不同）。空表示不指定。
	Effort string `json:"effort,omitempty"`
}

// ChatFinalizationRequest is a caller-provided delivery policy. The server does
// not infer execution intent from natural-language user text.
type ChatFinalizationRequest struct {
	RequireExecutionEvidence *bool `json:"requireExecutionEvidence,omitempty"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Message        string                  `json:"message" binding:"required"`
	ConversationID string                  `json:"conversationId,omitempty"`
	Role           string                  `json:"role,omitempty"`        // 角色名称
	AIChannelID    string                  `json:"aiChannelId,omitempty"` // 会话级 AI 通道；空则使用 ai.default_channel
	Reasoning      *ChatReasoningRequest   `json:"reasoning,omitempty"`
	Finalization   ChatFinalizationRequest `json:"finalization,omitempty"`
}

func (h *AgentHandler) configForAIChannel(channelID string) (*config.Config, string, error) {
	if h == nil || h.config == nil {
		return nil, "", fmt.Errorf("服务器配置未加载")
	}
	oa, resolvedID, ok := h.config.ResolveAIChannel(channelID)
	if !ok {
		return nil, resolvedID, fmt.Errorf("AI 通道不存在: %s", resolvedID)
	}
	cfgCopy := *h.config
	cfgCopy.OpenAI = oa
	return &cfgCopy, resolvedID, nil
}

func chatReasoningToClientIntent(r *ChatReasoningRequest) *reasoning.ClientIntent {
	if r == nil {
		return nil
	}
	return &reasoning.ClientIntent{Mode: r.Mode, Effort: r.Effort}
}

// appendAssistantMessageNotice 在助手消息末尾追加提示，避免覆盖已生成内容。
// 若消息为空则直接写入提示；若已包含相同提示则保持不变。
func (h *AgentHandler) appendAssistantMessageNotice(messageID, notice string) error {
	trimmedNotice := strings.TrimSpace(notice)
	if strings.TrimSpace(messageID) == "" || trimmedNotice == "" {
		return nil
	}
	_, err := h.db.Exec(
		`UPDATE messages
		 SET content = CASE
			WHEN content IS NULL OR TRIM(content) = '' THEN ?
			WHEN INSTR(content, ?) > 0 THEN content
			ELSE content || '\n\n' || ?
		 END,
		     updated_at = ?
		 WHERE id = ?`,
		trimmedNotice,
		trimmedNotice,
		trimmedNotice,
		time.Now(),
		messageID,
	)
	return err
}

// mergeAssistantMessagePartialOnCancel 将取消前已生成的部分回复尽量合并进消息：
// - content 为空或仅占位（处理中...）时，直接替换为 partial；
// - 已有正文时，仅在尚未包含 partial 时追加，避免丢失与重复。
func (h *AgentHandler) mergeAssistantMessagePartialOnCancel(messageID, partial string) error {
	trimmedPartial := strings.TrimSpace(partial)
	if strings.TrimSpace(messageID) == "" || trimmedPartial == "" {
		return nil
	}
	_, err := h.db.Exec(
		`UPDATE messages
		 SET content = CASE
			WHEN content IS NULL OR TRIM(content) = '' OR TRIM(content) = '处理中...' THEN ?
			WHEN INSTR(content, ?) > 0 THEN content
			ELSE content || '\n\n' || ?
		 END,
		     updated_at = ?
		 WHERE id = ?`,
		trimmedPartial,
		trimmedPartial,
		trimmedPartial,
		time.Now(),
		messageID,
	)
	return err
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type    string      `json:"type"`    // conversation, progress, tool_call, tool_result, response, error, cancelled, done
	Message string      `json:"message"` // 显示消息
	Data    interface{} `json:"data,omitempty"`
}

// publishProgressToTaskEventBus 将进度事件镜像到 taskEventBus（机器人/无 HTTP SSE 客户端时供 Web task-events 订阅）。
func (h *AgentHandler) publishProgressToTaskEventBus(conversationID, eventType, message string, data interface{}) {
	if h == nil || h.taskEventBus == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	event := StreamEvent{Type: eventType, Message: message, Data: data}
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return
	}
	sseLine := make([]byte, 0, len(eventJSON)+8)
	sseLine = append(sseLine, []byte("data: ")...)
	sseLine = append(sseLine, eventJSON...)
	sseLine = append(sseLine, '\n', '\n')
	h.taskEventBus.Publish(conversationID, sseLine)
}

func isInternalEinoDiagnosticProgress(eventType, message string, data interface{}) bool {
	switch eventType {
	case "model_output_rejected":
		return true
	case "progress":
		msg := strings.TrimSpace(message)
		if msg == "Eino TurnLoop 常驻多轮 runtime 已接管本轮会话。" ||
			msg == "Eino TurnLoop 已在安全点切换到用户补充后的下一轮。" ||
			msg == "已将用户补充推入 Eino TurnLoop，正在等待安全点切换…" {
			return true
		}
		m, ok := data.(map[string]interface{})
		if !ok {
			return false
		}
		switch strings.TrimSpace(fmt.Sprint(m["kind"])) {
		case "turn_loop_takeover", "turn_loop_preempted":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// enrichProgressEventData 为 SSE / taskEventBus 事件补齐 conversationId、messageId，便于前端懒加载过程详情。
func enrichProgressEventData(data interface{}, conversationID, assistantMessageID string) interface{} {
	if strings.TrimSpace(conversationID) == "" && strings.TrimSpace(assistantMessageID) == "" {
		return data
	}
	var m map[string]interface{}
	switch v := data.(type) {
	case map[string]interface{}:
		m = make(map[string]interface{}, len(v)+2)
		for k, val := range v {
			m[k] = val
		}
	case nil:
		m = make(map[string]interface{}, 2)
	default:
		m = map[string]interface{}{"payload": data}
	}
	if id := strings.TrimSpace(assistantMessageID); id != "" {
		if existing, ok := m["messageId"]; !ok || strings.TrimSpace(fmt.Sprint(existing)) == "" {
			m["messageId"] = id
		}
	}
	if id := strings.TrimSpace(conversationID); id != "" {
		if existing, ok := m["conversationId"]; !ok || strings.TrimSpace(fmt.Sprint(existing)) == "" {
			m["conversationId"] = id
		}
	}
	return m
}

// createProgressCallback 创建进度回调函数，用于保存processDetails
// sendEventFunc: 可选的流式事件发送函数，如果为nil则不发送流式事件
func (h *AgentHandler) createProgressCallback(runCtx context.Context, cancelRun context.CancelCauseFunc, conversationID, assistantMessageID string, sendEventFunc func(eventType, message string, data interface{})) agent.ProgressCallback {
	skillCallCache := make(map[string]string) // toolCallId -> skillName
	skillToolName := "skill"
	if h.config != nil {
		if customName := strings.TrimSpace(h.config.MultiAgent.EinoSkills.SkillToolName); customName != "" {
			skillToolName = customName
		}
	}

	extractSkillName := func(args map[string]interface{}) string {
		if len(args) == 0 {
			return ""
		}
		for _, key := range []string{"skill_name", "skillName", "name", "skill", "id", "skill_id", "skillId"} {
			if v, ok := args[key]; ok {
				switch vv := v.(type) {
				case string:
					if s := strings.TrimSpace(vv); s != "" {
						return s
					}
				case map[string]interface{}:
					for _, nestedKey := range []string{"name", "id", "skill_name", "skillId"} {
						if nestedV, nestedOK := vv[nestedKey].(string); nestedOK {
							if s := strings.TrimSpace(nestedV); s != "" {
								return s
							}
						}
					}
				}
			}
		}
		return ""
	}

	// thinking_stream_*（ReAct 等助手正文流）与 reasoning_chain_stream_*（Eino ReasoningContent）：
	// 不逐条落库，按 streamId 聚合，flush 时分别落 thinking / reasoning_chain。
	thinkingStreams := make(map[string]*thinkingBuf) // streamId -> buf
	flushedThinking := make(map[string]bool)         // streamId -> flushed
	seenToolCallSigs := make(map[string]string)      // toolCallId -> payload signature
	seenToolResultSigs := make(map[string]string)    // toolCallId -> payload signature

	// progressMu 保护闭包内 map 与聚合状态。Eino parallelRunToolCall 会在多 goroutine 中并发回调
	// progress（ToolInvokeNotifyHolder.Fire → createProgressCallback），未加锁的 map 会触发 fatal panic。
	var progressMu sync.Mutex

	// response_start + response_delta：前端时间线显示为「📝 规划中」（monitor.js），不落逐条 delta；
	// 聚合为一条 planning 写入 process_details，刷新后与线上一致。
	var respPlan responsePlanAgg
	persistResponsePlan := func(reset bool) {
		if assistantMessageID == "" {
			return
		}
		content := strings.TrimSpace(respPlan.b.String())
		if content == "" {
			if reset {
				respPlan = responsePlanAgg{}
			}
			return
		}
		data := map[string]interface{}{
			"source": "response_stream",
		}
		for k, v := range respPlan.meta {
			data[k] = v
		}
		var err error
		if respPlan.detailID == "" {
			respPlan.detailID, err = h.db.AddProcessDetailWithID(
				assistantMessageID, conversationID, "planning", content, data,
			)
		} else {
			err = h.db.UpdateProcessDetailContent(respPlan.detailID, content, data)
		}
		if err != nil {
			h.logger.Warn("保存过程详情失败", zap.Error(err), zap.String("eventType", "planning"))
		} else {
			respPlan.lastPersistAt = time.Now()
			respPlan.lastPersistSize = respPlan.b.Len()
		}
		if reset {
			respPlan = responsePlanAgg{}
		}
	}
	flushResponsePlan := func() { persistResponsePlan(true) }

	flushThinkingStreams := func() {
		if assistantMessageID == "" {
			return
		}
		for sid, tb := range thinkingStreams {
			if sid == "" || flushedThinking[sid] || tb == nil {
				continue
			}
			content := strings.TrimSpace(tb.b.String())
			if content == "" {
				flushedThinking[sid] = true
				continue
			}
			data := map[string]interface{}{
				"streamId": sid,
			}
			for k, v := range tb.meta {
				// 避免覆盖 streamId
				if k == "streamId" {
					continue
				}
				data[k] = v
			}
			persist := tb.persistAs
			if persist != "reasoning_chain" {
				persist = "thinking"
			}
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, persist, content, data); err != nil {
				h.logger.Warn("保存过程详情失败", zap.Error(err), zap.String("eventType", persist))
			}
			flushedThinking[sid] = true
		}
	}

	return func(eventType, message string, data interface{}) {
		progressMu.Lock()
		defer progressMu.Unlock()

		if isInternalEinoDiagnosticProgress(eventType, message, data) {
			return
		}

		// 上游在重试/补偿时可能重复回调相同 tool_call/tool_result。
		// 这里做幂等过滤，保证前端展示和 process_details 都以唯一事件为准。
		if (eventType == "tool_call" || eventType == "tool_result") && data != nil {
			if dataMap, ok := data.(map[string]interface{}); ok {
				toolCallID := strings.TrimSpace(fmt.Sprint(dataMap["toolCallId"]))
				if toolCallID != "" && toolCallID != "<nil>" {
					payloadJSON, _ := json.Marshal(dataMap)
					sig := eventType + "|" + message + "|" + string(payloadJSON)
					seen := seenToolCallSigs
					if eventType == "tool_result" {
						seen = seenToolResultSigs
					}
					if prev, exists := seen[toolCallID]; exists && prev == sig {
						h.logger.Debug("跳过重复工具进度事件",
							zap.String("eventType", eventType),
							zap.String("toolCallId", toolCallID))
						return
					}
					seen[toolCallID] = sig
				}
			}
		}

		// 工具输出片段不在详情区实时展示；完整结果由 tool_result 落库后按需拉取。
		if eventType == "tool_result_delta" {
			return
		}

		deferToolProgressSend := eventType == "tool_call" || eventType == "tool_result"
		// 主 HTTP SSE 与 taskEventBus 必须同时写入：页面刷新会切断原连接，刷新后的
		// GET task-events 订阅依赖 eventBus 才能继续收到后续迭代。机器人等无主 SSE
		// 的来源同样只写 eventBus。工具事件需先落库拿 processDetailId，再发送摘要。
		if !deferToolProgressSend {
			clientData := enrichProgressEventData(data, conversationID, assistantMessageID)
			if sendEventFunc != nil {
				sendEventFunc(eventType, message, clientData)
			}
			h.publishProgressToTaskEventBus(conversationID, eventType, message, clientData)
		}

		// 缓存 skills 调用的参数，供 tool_result 阶段统计调用结果
		if eventType == "tool_call" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				toolName, _ := dataMap["toolName"].(string)
				if strings.EqualFold(strings.TrimSpace(toolName), skillToolName) {
					toolCallID, _ := dataMap["toolCallId"].(string)
					if toolCallID != "" {
						if argumentsObj, ok := dataMap["argumentsObj"].(map[string]interface{}); ok {
							if skillName := extractSkillName(argumentsObj); skillName != "" {
								skillCallCache[toolCallID] = skillName
							}
						}
					}
				}
			}
		}

		// 记录 skills 调用统计（tool_call + tool_result 关联）
		if eventType == "tool_result" && h.db != nil {
			if dataMap, ok := data.(map[string]interface{}); ok {
				toolName, _ := dataMap["toolName"].(string)
				if strings.EqualFold(strings.TrimSpace(toolName), skillToolName) {
					toolCallID, _ := dataMap["toolCallId"].(string)
					skillName := ""
					if toolCallID != "" {
						skillName = strings.TrimSpace(skillCallCache[toolCallID])
						delete(skillCallCache, toolCallID)
					}
					if skillName == "" {
						if argumentsObj, ok := dataMap["argumentsObj"].(map[string]interface{}); ok {
							skillName = strings.TrimSpace(extractSkillName(argumentsObj))
						}
					}
					if skillName != "" {
						success, ok := dataMap["success"].(bool)
						if !ok {
							if isError, okErr := dataMap["isError"].(bool); okErr {
								success = !isError
							}
						}
						successCalls := 0
						failedCalls := 0
						if success {
							successCalls = 1
						} else {
							failedCalls = 1
						}
						now := time.Now()
						if err := h.db.UpdateSkillStats(skillName, 1, successCalls, failedCalls, &now); err != nil {
							h.logger.Warn("更新Skills调用统计失败", zap.Error(err), zap.String("skill", skillName))
						}
					}
				}
			}
		}

		// 子代理回复流式增量不落库；结束时合并为一条 eino_agent_reply
		if assistantMessageID != "" && eventType == "eino_agent_reply_stream_end" {
			flushResponsePlan()
			// 确保思考流在子代理回复前能持久化（刷新后可读）
			flushThinkingStreams()
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "eino_agent_reply", message, data); err != nil {
				h.logger.Warn("保存过程详情失败", zap.Error(err), zap.String("eventType", eventType))
			}
			return
		}

		// 多代理主代理「规划中」：response_start / response_delta 仅用于 SSE，聚合落一条 planning
		if eventType == "response_start" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if sameResponseStreamMeta(respPlan.meta, dataMap) {
					if respPlan.meta == nil {
						respPlan.meta = make(map[string]interface{}, len(dataMap))
					}
					for k, v := range dataMap {
						respPlan.meta[k] = v
					}
					return
				}
			}
			flushResponsePlan()
			// 助手正文开始前，推理流通常已结束；落库以便刷新后「渗透测试详情」可回放
			flushThinkingStreams()
			respPlan.meta = nil
			if dataMap, ok := data.(map[string]interface{}); ok {
				respPlan.meta = make(map[string]interface{}, len(dataMap))
				for k, v := range dataMap {
					respPlan.meta[k] = v
				}
			}
			respPlan.b.Reset()
			return
		}
		if eventType == "response_delta" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if acc, okAcc := dataMap[openai.SSEAccumulatedKey].(string); okAcc {
					respPlan.b.Reset()
					respPlan.b.WriteString(acc)
				} else {
					respPlan.b.WriteString(message)
				}
			} else {
				respPlan.b.WriteString(message)
			}
			if dataMap, ok := data.(map[string]interface{}); ok && respPlan.meta == nil {
				respPlan.meta = make(map[string]interface{}, len(dataMap))
				for k, v := range dataMap {
					respPlan.meta[k] = v
				}
			} else if dataMap, ok := data.(map[string]interface{}); ok {
				for k, v := range dataMap {
					respPlan.meta[k] = v
				}
			}
			// 运行中的主回复不能只保存在内存：刷新会销毁旧页面，新的 task-events
			// 订阅只能收到未来增量。按时间或增量大小节流更新同一条 planning 记录，
			// 这样刷新时能从数据库恢复刷新前已经展示的全部文本。
			if respPlan.lastPersistAt.IsZero() ||
				time.Since(respPlan.lastPersistAt) >= 300*time.Millisecond ||
				respPlan.b.Len()-respPlan.lastPersistSize >= 1024 {
				persistResponsePlan(false)
			}
			return
		}
		if eventType == "response" {
			flushResponsePlan()
			flushThinkingStreams()
			return
		}
		if eventType == "done" {
			flushResponsePlan()
			flushThinkingStreams()
			return
		}

		// 流式思考/推理结束：聚合落库（与 eino_agent_reply_stream_end 同理）
		if eventType == "thinking_stream_end" || eventType == "reasoning_chain_stream_end" {
			flushResponsePlan()
			flushThinkingStreams()
			return
		}

		// 聚合 thinking_stream_* / reasoning_chain_stream_*，不逐条落库
		if eventType == "thinking_stream_start" || eventType == "reasoning_chain_stream_start" {
			persistAs := "thinking"
			if eventType == "reasoning_chain_stream_start" {
				persistAs = "reasoning_chain"
			}
			if dataMap, ok := data.(map[string]interface{}); ok {
				if sid, ok2 := dataMap["streamId"].(string); ok2 && sid != "" {
					tb := thinkingStreams[sid]
					if tb == nil {
						tb = &thinkingBuf{meta: map[string]interface{}{}, persistAs: persistAs}
						thinkingStreams[sid] = tb
					} else {
						tb.persistAs = persistAs
					}
					// 记录元信息（source/einoAgent/einoRole/iteration 等）
					for k, v := range dataMap {
						tb.meta[k] = v
					}
				}
			}
			return
		}
		if eventType == "thinking_stream_delta" || eventType == "reasoning_chain_stream_delta" {
			persistAs := "thinking"
			if eventType == "reasoning_chain_stream_delta" {
				persistAs = "reasoning_chain"
			}
			if dataMap, ok := data.(map[string]interface{}); ok {
				if sid, ok2 := dataMap["streamId"].(string); ok2 && sid != "" {
					tb := thinkingStreams[sid]
					if tb == nil {
						tb = &thinkingBuf{meta: map[string]interface{}{}, persistAs: persistAs}
						thinkingStreams[sid] = tb
					} else if tb.persistAs == "" {
						tb.persistAs = persistAs
					}
					if acc, okAcc := dataMap[openai.SSEAccumulatedKey].(string); okAcc {
						tb.b.Reset()
						tb.b.WriteString(acc)
					} else {
						tb.b.WriteString(message)
					}
					// 有时 delta 先到 start 未到，补充元信息
					for k, v := range dataMap {
						tb.meta[k] = v
					}
				}
			}
			return
		}

		// 当 Agent 同时发送 *_stream_* 与同名 streamId 的 thinking/reasoning_chain 时，
		// 流式聚合已会在 flushThinkingStreams() 落库；此处跳过逐条重复。
		if eventType == "thinking" || eventType == "reasoning_chain" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if sid, ok2 := dataMap["streamId"].(string); ok2 && sid != "" {
					if tb, exists := thinkingStreams[sid]; exists && tb != nil {
						if strings.TrimSpace(tb.b.String()) != "" {
							return
						}
					}
					if flushedThinking[sid] {
						return
					}
				}
			}
		}

		// 保存过程详情到数据库（排除 response/done；response 正文已在 messages 表）
		// response_start/response_delta 已聚合为 planning，不落逐条。
		// [Eino] agent 心跳 progress 仅用于实时进度标题，不落库以免时间线刷屏。
		skipEinoAgentHeartbeat := eventType == "progress" && strings.HasPrefix(strings.TrimSpace(message), "[Eino] ")
		if assistantMessageID != "" &&
			!skipEinoAgentHeartbeat &&
			eventType != "response" &&
			eventType != "done" &&
			eventType != "response_start" &&
			eventType != "response_delta" &&
			eventType != "tool_result_delta" &&
			eventType != "eino_trace_run" &&
			eventType != "eino_trace_start" &&
			eventType != "eino_trace_end" &&
			eventType != "eino_trace_error" &&
			eventType != "eino_agent_reply_stream_start" &&
			eventType != "eino_agent_reply_stream_delta" &&
			eventType != "eino_agent_reply_stream_end" {
			if eventType == "tool_result" {
				if detailID := discardPlanningIfEchoesToolResult(&respPlan, data); detailID != "" {
					if err := h.db.DeleteProcessDetail(detailID); err != nil {
						h.logger.Warn("删除工具结果回显规划失败", zap.Error(err), zap.String("processDetailId", detailID))
					}
				}
			}
			// 在关键过程事件落库前，先把「规划中」与聚合中的 thinking / reasoning_chain 流落库
			flushResponsePlan()
			flushThinkingStreams()
			processDetailID, err := h.db.AddProcessDetailWithID(assistantMessageID, conversationID, eventType, message, data)
			if err != nil {
				h.logger.Warn("保存过程详情失败", zap.Error(err), zap.String("eventType", eventType))
			}
			if deferToolProgressSend {
				clientData := enrichProgressEventData(summarizeProcessDetailData(eventType, data), conversationID, assistantMessageID)
				if m, ok := clientData.(map[string]interface{}); ok {
					m["processDetailId"] = processDetailID
				}
				if sendEventFunc != nil {
					sendEventFunc(eventType, message, clientData)
				}
				h.publishProgressToTaskEventBus(conversationID, eventType, message, clientData)
			}
		} else if deferToolProgressSend {
			clientData := enrichProgressEventData(summarizeProcessDetailData(eventType, data), conversationID, assistantMessageID)
			if sendEventFunc != nil {
				sendEventFunc(eventType, message, clientData)
			}
			h.publishProgressToTaskEventBus(conversationID, eventType, message, clientData)
		}
	}
}

// cancelToolContinueAfter 仅终止当前工具调用，不停止整条 Agent 任务（对话「中断并继续」与 MCP 监控终止共用）。
func (h *AgentHandler) cancelToolContinueAfter(conversationID, preferredExecID, note string) (bool, gin.H) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || h.tasks.GetTask(conversationID) == nil {
		return false, nil
	}
	note = strings.TrimSpace(note)
	execID := strings.TrimSpace(preferredExecID)
	if execID == "" {
		execID = h.tasks.ActiveMCPExecutionID(conversationID)
	}
	if execID != "" {
		if h.agent.CancelMCPToolExecutionWithNote(execID, note) {
			return true, gin.H{
				"status":              "tool_abort_requested",
				"conversationId":      conversationID,
				"executionId":         execID,
				"message":             "已请求终止当前工具调用；工具返回后本轮推理将继续（与 MCP 监控页终止一致）。",
				"continueAfter":       true,
				"interruptWithNote":   note != "",
				"continueWithoutTool": false,
			}
		}
		if h.tasks.AbortActiveEinoExecute(conversationID, note) {
			return true, gin.H{
				"status":              "tool_abort_requested",
				"conversationId":      conversationID,
				"executionId":         execID,
				"message":             "已请求终止当前 execute 命令；命令返回后本轮推理将继续。",
				"continueAfter":       true,
				"interruptWithNote":   note != "",
				"continueWithoutTool": false,
			}
		}
		return false, nil
	}
	if h.tasks.AbortActiveEinoExecute(conversationID, note) {
		return true, gin.H{
			"status":              "tool_abort_requested",
			"conversationId":      conversationID,
			"message":             "已请求终止当前 execute 命令；命令返回后本轮推理将继续。",
			"continueAfter":       true,
			"interruptWithNote":   note != "",
			"continueWithoutTool": false,
		}
	}
	return false, nil
}

// CancelAgentLoop 取消正在执行的任务
func (h *AgentHandler) CancelAgentLoop(c *gin.Context) {
	var req struct {
		ConversationID string `json:"conversationId" binding:"required"`
		ExecutionID    string `json:"executionId,omitempty"`
		Reason         string `json:"reason,omitempty"`
		ContinueAfter  bool   `json:"continueAfter,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.agentConversationAllowed(c, req.ConversationID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该资源"})
		return
	}

	if req.ContinueAfter {
		if h.tasks.GetTask(req.ConversationID) == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到正在执行的任务"})
			return
		}
		note := strings.TrimSpace(req.Reason)
		activeExec := strings.TrimSpace(h.tasks.ActiveMCPExecutionID(req.ConversationID))
		if ok, payload := h.cancelToolContinueAfter(req.ConversationID, strings.TrimSpace(req.ExecutionID), note); ok {
			execID, _ := payload["executionId"].(string)
			h.logger.Info("对话页仅终止当前工具",
				zap.String("conversationId", req.ConversationID),
				zap.String("executionId", execID),
				zap.Bool("hasNote", note != ""),
			)
			c.JSON(http.StatusOK, payload)
			return
		}
		if activeExec != "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到进行中的工具执行或该调用已结束"})
			return
		}
		// 无进行中的 MCP 工具（模型纯推理/流式输出阶段）：取消当前上下文并由 Eino 流式处理器合并用户补充后自动续跑。
		h.tasks.SetInterruptContinueNote(req.ConversationID, note)
		ok, err := h.tasks.CancelTask(req.ConversationID, multiagent.ErrInterruptContinue)
		if err != nil {
			h.logger.Error("中断并继续（无工具）失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到正在执行的任务"})
			return
		}
		h.logger.Info("对话页中断并继续（无 MCP 工具，将自动续跑）",
			zap.String("conversationId", req.ConversationID),
			zap.Bool("hasNote", note != ""),
		)
		c.JSON(http.StatusOK, gin.H{
			"status":              "interrupt_continue_scheduled",
			"conversationId":      req.ConversationID,
			"message":             "已请求暂停当前推理；用户补充将合并到上下文并自动继续执行（无需整轮停止）。",
			"continueAfter":       true,
			"interruptWithNote":   note != "",
			"continueWithoutTool": true,
		})
		return
	}

	var cause error = ErrTaskCancelled
	msg := "已提交取消请求，任务将在当前步骤完成后停止。"
	h.cancelRunningMCPToolsForConversation(req.ConversationID)
	h.tasks.AbortActiveEinoExecute(req.ConversationID, "")
	ok, err := h.tasks.CancelTask(req.ConversationID, cause)
	if err != nil {
		h.logger.Error("取消任务失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到正在执行的任务"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":            "cancelling",
		"conversationId":    req.ConversationID,
		"message":           msg,
		"continueAfter":     false,
		"interruptWithNote": false,
	})
}

// SubscribeAgentTaskEvents GET SSE：订阅指定会话当前运行中任务的事件镜像（帧格式与 POST .../stream 一致），用于刷新页面或断线后接续 UI。
func (h *AgentHandler) SubscribeAgentTaskEvents(c *gin.Context) {
	conversationID := strings.TrimSpace(c.Query("conversationId"))
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversationId is required"})
		return
	}
	if !h.agentConversationAllowed(c, conversationID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该资源"})
		return
	}
	if h.tasks.GetTask(conversationID) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active task for this conversation"})
		return
	}
	if h.taskEventBus == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "task event bus unavailable"})
		return
	}

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	sub, ch := h.taskEventBus.Subscribe(conversationID)
	defer h.taskEventBus.Unsubscribe(conversationID, sub)

	flusher, _ := c.Writer.(http.Flusher)
	ctx := c.Request.Context()
	var writeMu sync.Mutex
	stopKeepalive := runSSEKeepalive(c, &writeMu)
	defer stopKeepalive()

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-ch:
			if !ok {
				return
			}
			writeMu.Lock()
			if _, err := c.Writer.Write(chunk); err != nil {
				writeMu.Unlock()
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
			writeMu.Unlock()
		}
	}
}

// enrichAgentTasksWithConversationTitles 为任务列表附加当前会话标题（供顶栏/任务页展示，重命名后自动同步）
func (h *AgentHandler) enrichAgentTasksWithConversationTitles(tasks []*AgentTask) {
	if h == nil || h.db == nil {
		return
	}
	for _, task := range tasks {
		if task == nil || strings.TrimSpace(task.ConversationID) == "" {
			continue
		}
		if title, err := h.db.GetConversationTitle(task.ConversationID); err == nil {
			task.Title = strings.TrimSpace(title)
		}
	}
}

// enrichCompletedTasksWithConversationTitles 为已完成任务附加当前会话标题
func (h *AgentHandler) enrichCompletedTasksWithConversationTitles(tasks []*CompletedTask) {
	if h == nil || h.db == nil {
		return
	}
	for _, task := range tasks {
		if task == nil || strings.TrimSpace(task.ConversationID) == "" {
			continue
		}
		if title, err := h.db.GetConversationTitle(task.ConversationID); err == nil {
			task.Title = strings.TrimSpace(title)
		}
	}
}

// ListAgentTasks 列出所有运行中的任务
func (h *AgentHandler) ListAgentTasks(c *gin.Context) {
	tasks := h.tasks.GetActiveTasks()
	tasks = filterSlice(tasks, func(task *AgentTask) bool {
		return task != nil && h.agentConversationAllowed(c, task.ConversationID)
	})
	h.enrichAgentTasksWithConversationTitles(tasks)
	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
	})
}

// ListCompletedTasks 列出最近完成的任务历史
func (h *AgentHandler) ListCompletedTasks(c *gin.Context) {
	tasks := h.tasks.GetCompletedTasks()
	tasks = filterSlice(tasks, func(task *CompletedTask) bool {
		return task != nil && h.agentConversationAllowed(c, task.ConversationID)
	})
	h.enrichCompletedTasksWithConversationTitles(tasks)
	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
	})
}

func (h *AgentHandler) agentConversationAllowed(c *gin.Context, conversationID string) bool {
	session, ok := security.CurrentSession(c)
	return ok && h.db != nil && h.db.UserCanAccessResource(session.UserID, session.Scope, "conversation", strings.TrimSpace(conversationID))
}

func filterSlice[T any](items []T, keep func(T) bool) []T {
	out := make([]T, 0, len(items))
	for _, item := range items {
		if keep(item) {
			out = append(out, item)
		}
	}
	return out
}

// loadHistoryFromAgentTrace 从库中保存的代理消息轨迹恢复历史（列 last_react_*；含单代理与 Eino）。
// 逻辑与攻击链一致：优先用已保存的 JSON 消息带 + 最后一轮助手摘要，否则回退消息表。
func (h *AgentHandler) loadHistoryFromAgentTrace(conversationID string) ([]agent.ChatMessage, error) {
	traceInputJSON, assistantOut, err := h.db.GetAgentTrace(conversationID)
	if err != nil {
		return nil, fmt.Errorf("获取代理轨迹失败: %w", err)
	}

	if traceInputJSON == "" {
		return nil, fmt.Errorf("代理轨迹为空，将使用消息表")
	}

	dataSource := "database_last_agent_trace"

	parsed, err := agent.ParseTraceMessages(traceInputJSON)
	if err != nil {
		return nil, fmt.Errorf("解析代理轨迹 JSON 失败: %w", err)
	}

	messageCount := len(parsed)

	h.logger.Info("使用保存的代理轨迹恢复历史上下文",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("traceInputSize", len(traceInputJSON)),
		zap.Int("messageCount", messageCount),
		zap.Int("assistantOutSize", len(assistantOut)),
	)
	agentMessages := agent.TraceMessagesForResume(parsed)

	// 若存在 last_react_output（助手摘要），合并为最后一条 assistant（与保存格式一致）
	if assistantOut != "" {
		if len(agentMessages) > 0 {
			lastMsg := &agentMessages[len(agentMessages)-1]
			if strings.EqualFold(lastMsg.Role, "assistant") && len(lastMsg.ToolCalls) == 0 {
				lastMsg.Content = assistantOut
			} else {
				agentMessages = append(agentMessages, agent.ChatMessage{
					Role:    "assistant",
					Content: assistantOut,
				})
			}
		} else {
			agentMessages = append(agentMessages, agent.ChatMessage{
				Role:    "assistant",
				Content: assistantOut,
			})
		}
	}

	if len(agentMessages) == 0 {
		return nil, fmt.Errorf("从代理轨迹解析的消息为空")
	}

	if h.agent != nil {
		if fixed := h.agent.RepairOrphanToolMessages(&agentMessages); fixed {
			h.logger.Info("修复了从代理轨迹恢复的历史消息中的失配 tool 消息",
				zap.String("conversationId", conversationID),
			)
		}
	}

	h.logger.Info("从代理轨迹恢复历史消息完成",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("originalMessageCount", messageCount),
		zap.Int("finalMessageCount", len(agentMessages)),
		zap.Bool("hasAssistantOut", assistantOut != ""),
	)
	return agentMessages, nil
}

// dbMessagesToAgentChatMessages maps DB rows to agent ChatMessage for history fallback
// (includes reasoning_content for DeepSeek thinking + tool replay).
func dbMessagesToAgentChatMessages(msgs []database.Message) []agent.ChatMessage {
	out := make([]agent.ChatMessage, 0, len(msgs))
	for i := range msgs {
		m := msgs[i]
		out = append(out, agent.ChatMessage{
			Role:             m.Role,
			Content:          m.Content,
			ReasoningContent: m.ReasoningContent,
		})
	}
	return out
}
