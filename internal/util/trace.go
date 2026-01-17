package util

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// contextKey 是一个私有类型，用于避免 context key 的冲突
type contextKey string

const traceIDKey contextKey = "traceID"

// NewTraceID 生成一个随机的、唯一的 Trace ID
// 用于在分布式系统中追踪单个请求的完整生命周期
func NewTraceID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// 在极少数情况下，如果随机数生成失败，返回一个固定的错误字符串
		return "failed-to-generate-trace-id"
	}
	return hex.EncodeToString(bytes)
}

// ContextWithTraceID 将 Trace ID 注入到 Context 中，并返回一个新的 Context
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// TraceIDFromContext 从 Context 中提取 Trace ID
func TraceIDFromContext(ctx context.Context) (string, bool) {
	traceID, ok := ctx.Value(traceIDKey).(string)
	return traceID, ok
}
