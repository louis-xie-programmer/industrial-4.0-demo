package station

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"industrial-4.0-demo/internal/types"
	"industrial-4.0-demo/internal/util"
	"log/slog"
	"net/http"
	"time"
)

// RemoteStation 代表一个通过 HTTP 调用的远程工站客户端
// 它实现了 Station 接口，使得引擎层可以像对待本地工站一样对待它
type RemoteStation struct {
	ID       types.StationID // 工站 ID
	Endpoint string          // 远程服务的地址 (e.g., http://localhost:9090)
	Client   *http.Client    // HTTP 客户端
	logger   *slog.Logger    // 日志记录器
}

// NewRemoteStation 创建一个新的远程工站实例
func NewRemoteStation(id types.StationID, endpoint string, logger *slog.Logger) Station {
	return &RemoteStation{
		ID:       id,
		Endpoint: endpoint,
		Client:   &http.Client{Timeout: 5 * time.Second}, // 设置 5 秒超时
		logger:   logger.With("station_id", id, "remote", true),
	}
}

func (s *RemoteStation) GetID() types.StationID {
	return s.ID
}

// remoteRequest 定义了发送到远程服务的请求体
type remoteRequest struct {
	ID string `json:"id"`
}

// remoteResponse 定义了从远程服务接收的响应体
type remoteResponse struct {
	ProductID string `json:"product_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// Execute 通过 HTTP POST 请求调用远程工站的 /execute 端点
func (s *RemoteStation) Execute(ctx context.Context, p *types.Product) types.Result {
	logger := s.logger
	if traceID, ok := util.TraceIDFromContext(ctx); ok {
		logger = logger.With("trace_id", traceID)
	}
	logger.Info("请求处理工件", "product_id", p.ID)

	reqBody, _ := json.Marshal(remoteRequest{ID: p.ID})
	httpReq, err := http.NewRequestWithContext(ctx, "POST", s.Endpoint+"/execute", bytes.NewBuffer(reqBody))
	if err != nil {
		logger.Error("创建远程请求失败", "error", err, "product_id", p.ID)
		return types.Result{ProductID: p.ID, Success: false, Error: err}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// 将 Trace ID 放入 HTTP Header 中，实现跨服务追踪
	if traceID, ok := util.TraceIDFromContext(ctx); ok {
		httpReq.Header.Set("X-Trace-ID", traceID)
	}

	resp, err := s.Client.Do(httpReq)
	if err != nil {
		logger.Error("远程调用失败", "error", err, "product_id", p.ID)
		return types.Result{ProductID: p.ID, Success: false, Error: fmt.Errorf("远程调用失败: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("远程服务返回错误状态", "status", resp.Status, "product_id", p.ID)
		return types.Result{ProductID: p.ID, Success: false, Error: fmt.Errorf("远程服务错误: %s", resp.Status)}
	}

	var rResp remoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&rResp); err != nil {
		logger.Error("解析远程响应失败", "error", err, "product_id", p.ID)
		return types.Result{ProductID: p.ID, Success: false, Error: fmt.Errorf("解析响应失败: %v", err)}
	}

	if !rResp.Success {
		logger.Warn("远程工件处理失败", "remote_error", rResp.Error, "product_id", p.ID)
		return types.Result{ProductID: p.ID, Success: false, Error: fmt.Errorf(rResp.Error)}
	}

	p.History = append(p.History, string(s.ID)+"(Remote)")
	logger.Info("远程工件处理成功", "product_id", p.ID)
	return types.Result{ProductID: p.ID, Success: true}
}

// Compensate 通过 HTTP POST 请求调用远程工站的 /compensate 端点
func (s *RemoteStation) Compensate(ctx context.Context, p *types.Product) {
	logger := s.logger
	if traceID, ok := util.TraceIDFromContext(ctx); ok {
		logger = logger.With("trace_id", traceID)
	}
	logger.Warn("请求补偿", "product_id", p.ID)

	reqBody, _ := json.Marshal(remoteRequest{ID: p.ID})
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", s.Endpoint+"/compensate", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	if traceID, ok := util.TraceIDFromContext(ctx); ok {
		httpReq.Header.Set("X-Trace-ID", traceID)
	}
	_, _ = s.Client.Do(httpReq)
}
