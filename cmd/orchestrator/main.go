package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"industrial-4.0-demo/internal/config"
	"industrial-4.0-demo/internal/engine"
	"industrial-4.0-demo/internal/persistence"
	"industrial-4.0-demo/internal/station"
	"industrial-4.0-demo/internal/types"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const walPath = "tasks.wal"

func main() {
	// 0. 初始化 WAL
	wal, err := persistence.NewWAL(walPath)
	if err != nil {
		log.Fatalf("无法初始化 WAL: %v", err)
	}
	defer wal.Close()

	// 1. 加载配置
	cfg := config.LoadConfig()

	// 2. 初始化编排引擎与工站
	wf := engine.NewWorkflowEngine(cfg.Workflows)
	wf.RegisterStation(station.NewStation(types.StationEntry))
	wf.RegisterStation(station.NewStation(types.StationAssemble))
	wf.RegisterStation(station.NewStation(types.StationPaint)) // 新增
	wf.RegisterStation(station.NewStation(types.StationDry))   // 新增
	wf.RegisterStation(station.NewStation(types.StationQC))
	wf.RegisterStation(station.NewStation(types.StationExit))

	// 3. 初始化调度器
	scheduler := engine.NewScheduler(wf, cfg.MaxWorkers, wal)

	// 4. 从 WAL 恢复任务
	if err := scheduler.RecoverTasks(); err != nil {
		log.Printf("警告: 从 WAL 恢复任务失败: %v", err)
	}

	fmt.Println("=== 工业 4.0 智能调度系统启动 ===")

	// 创建上下文以控制生命周期
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 5. 在后台启动调度逻辑
	go scheduler.Start(ctx)

	// 6. 启动 HTTP API 服务器
	go startAPIServer(scheduler)

	// 7. 模拟不同时间到达的订单 (仅在 WAL 为空时，避免重复提交)
	// 实际场景中，这里可能不需要
	go func() {
		// 简单的检查，看是否是首次启动
		info, _ := os.Stat(walPath)
		if info.Size() > 100 { // 假设有内容了就不再模拟提交
			return
		}

		// 先来两个普通订单
		scheduler.SubmitTask(&types.Product{ID: "Normal_01", Type: "STANDARD", Priority: 0})
		scheduler.SubmitTask(&types.Product{ID: "Simple_01", Type: "SIMPLE", Priority: 0})

		time.Sleep(200 * time.Millisecond)

		// 突然来了一个紧急订单 (VIP)
		scheduler.SubmitTask(&types.Product{ID: "URGENT_EV_BATTERY", Type: "STRICT", Priority: 2})

		// 再来一个加急订单
		scheduler.SubmitTask(&types.Product{ID: "RUSH_ORDER_01", Type: "STANDARD", Priority: 1})

		// 演示并行工序
		time.Sleep(500 * time.Millisecond)
		scheduler.SubmitTask(&types.Product{ID: "PARALLEL_DEMO_01", Type: "PARALLEL_DEMO", Priority: 1})
	}()

	// 8. 监听系统信号实现优雅停机
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	<-sigChan
	fmt.Println("\n接收到停机信号，正在优雅关闭...")

	// 触发关闭流程
	cancel()
	scheduler.WaitForCompletion()
	fmt.Println("生产演示结束，系统已安全退出。")
}

func startAPIServer(scheduler *engine.Scheduler) {
	// 注册 Prometheus metrics handler
	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var p types.Product
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// 简单的校验
		if p.ID == "" {
			p.ID = fmt.Sprintf("API_ORDER_%d", time.Now().UnixNano())
		}
		if p.Type == "" {
			p.Type = "STANDARD"
		}

		scheduler.SubmitTask(&p)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted", "id": p.ID})
	})

	fmt.Println("API 服务器启动在 :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("API 服务器启动失败: %v\n", err)
	}
}
