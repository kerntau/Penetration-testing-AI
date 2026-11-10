package multiagent

import (
	"context"

	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/schema"
)

// fixedTokenCounter 让 tool 消息按 tokensPerToolMessage 计，其它消息按 1 计。
// 用于验证 tool-round 超预算时整体被跳过的分支。
func fixedTokenCounter(tokensPerToolMessage int) summarization.TokenCounterFunc {
	return func(_ context.Context, in *summarization.TokenCounterInput) (int, error) {
		total := 0
		for _, msg := range in.Messages {
			if msg == nil {
				continue
			}
			switch msg.Role {
			case schema.Tool:
				total += tokensPerToolMessage
			default:
				total++
			}
		}
		return total, nil
	}
}
