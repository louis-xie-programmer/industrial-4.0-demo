package main

import (
	"encoding/json"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"
)

// Request 定义了远程服务接收的请求体
type Request struct {
	ID string `json:"id"`
}

// Response 定义了远程服务返回的响应体
type Response struct {
	ProductID string `json:"product_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// main 是远程工站服务的入口
func main() {
	port := ":9090"
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "remote-station")
	slog.SetDefault(logger)

	logger.Info("=== 远程工站服务 (AOI) 启动 ===", "port", port)

	// 注册 HTTP 处理函数
	http.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Warn("解析请求失败", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// 从 HTTP Header 中提取 Trace ID，用于链路追踪
		traceID := r.Header.Get("X-Trace-ID")
		taskLogger := logger.With("product_id", req.ID)
		if traceID != "" {
			taskLogger = taskLogger.With("trace_id", traceID)
		}

		taskLogger.Info("接收到任务")

		// 模拟远程处理耗时
		processTime := time.Duration(rand.Intn(2000)+3000) * time.Millisecond
		time.Sleep(processTime)

		// 模拟随机失败
		success := true
		errMsg := ""
		if rand.Float32() < 0.1 { // 10% 概率失败
			success = false
			errMsg = "远程设备故障 (AOI 检测发现缺陷)"
			taskLogger.Warn("任务失败", "error", errMsg)
		} else {
			taskLogger.Info("任务完成", "duration", processTime.Seconds())
		}

		resp := Response{ProductID: req.ID, Success: success, Error: errMsg}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	http.HandleFunc("/compensate", func(w http.ResponseWriter, r *http.Request) {
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return
		}

		traceID := r.Header.Get("X-Trace-ID")
		compLogger := logger.With("product_id", req.ID)
		if traceID != "" {
			compLogger = compLogger.With("trace_id", traceID)
		}

		compLogger.Warn("执行补偿")
		time.Sleep(1000 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	if err := http.ListenAndServe(port, nil); err != nil {
		logger.Error("服务启动失败", "error", err)
	}
}
