package main

import (
	"context"
	"encoding/json"
	"industrial-4.0-demo/internal/config"
	"industrial-4.0-demo/internal/engine"
	"industrial-4.0-demo/internal/event"
	"industrial-4.0-demo/internal/handlers"
	"industrial-4.0-demo/internal/persistence"
	"industrial-4.0-demo/internal/station"
	"industrial-4.0-demo/internal/types"
	"industrial-4.0-demo/internal/web"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const walPath = "tasks.wal"

// main 是应用程序的主入口
func main() {
	// 1. 初始化核心组件
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	hub := web.NewHub()
	go hub.Run()
	stateTracker := web.NewStateTracker(hub)

	eventBus := event.NewBus()

	wal, err := persistence.NewWAL(walPath)
	if err != nil {
		logger.Error("无法初始化 WAL", "error", err)
		os.Exit(1)
	}
	defer wal.Close()

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	// 2. 注册事件处理器，将系统各部分与事件总线连接起来
	handlers.RegisterEventHandlers(eventBus, stateTracker, logger)

	// 3. 初始化引擎和调度器
	wf := engine.NewWorkflowEngine(cfg.Workflows, cfg.ResourcePools, logger, eventBus)
	registerStations(wf, logger)

	scheduler := engine.NewScheduler(wf, cfg.MaxWorkers, wal, stateTracker, logger)

	// 4. 从 WAL 恢复未完成的任务
	if err := scheduler.RecoverTasks(); err != nil {
		logger.Warn("从 WAL 恢复任务失败", "error", err)
	}

	logger.Info("=== PCB 智能工厂调度系统启动 ===")

	// 5. 设置上下文用于优雅停机
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 6. 启动核心服务
	go scheduler.Start(ctx)
	go startAPIServer(scheduler, hub, stateTracker, logger)
	go simulateTasks(scheduler)

	// 7. 等待停机信号
	waitForShutdown(logger, cancel, scheduler)
}

// registerStations 注册所有可用的工站
func registerStations(wf *engine.WorkflowEngine, logger *slog.Logger) {
	// 注册本地工站
	wf.RegisterStation(station.NewStation(types.StationCAM, logger))
	wf.RegisterStation(station.NewStation(types.StationDrill, logger))
	wf.RegisterStation(station.NewStation(types.StationLami, logger))
	wf.RegisterStation(station.NewStation(types.StationEtch, logger))
	wf.RegisterStation(station.NewStation(types.StationMask, logger))
	wf.RegisterStation(station.NewStation(types.StationSilk, logger))
	wf.RegisterStation(station.NewStation(types.StationETest, logger))
	wf.RegisterStation(station.NewStation(types.StationPack, logger))

	// 注册远程工站 (AOI)
	remoteAddr := os.Getenv("REMOTE_STATION_ADDR")
	if remoteAddr == "" {
		remoteAddr = "http://localhost:9090"
	}
	wf.RegisterStation(station.NewRemoteStation(types.StationAOI, remoteAddr, logger))
}

// simulateTasks 模拟提交初始订单，仅在首次启动时执行
func simulateTasks(scheduler *engine.Scheduler) {
	info, _ := os.Stat(walPath)
	if info != nil && info.Size() > 100 {
		return
	}

	// 1. 标准双面板订单
	scheduler.SubmitTask(&types.Product{
		ID:       "PCB_Double_001",
		Type:     "PCB_DOUBLE_LAYER",
		Priority: 0,
		Attrs:    map[string]interface{}{"layers": 2},
	})

	// 2. 多层板订单 (4层，触发层压规则)
	scheduler.SubmitTask(&types.Product{
		ID:       "PCB_Multi_4L_001",
		Type:     "PCB_MULTILAYER",
		Priority: 1,
		Attrs:    map[string]interface{}{"layers": 4},
	})

	// 3. 极速打样订单 (跳过 AOI)
	scheduler.SubmitTask(&types.Product{
		ID:       "PCB_Proto_Fast",
		Type:     "PCB_PROTOTYPE",
		Priority: 2, // 最高优先级
		Attrs:    map[string]interface{}{"layers": 2},
	})
}

// startAPIServer 启动 API 和 Web 服务器
func startAPIServer(scheduler *engine.Scheduler, hub *web.Hub, st *web.StateTracker, logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler()) // Prometheus 指标端点
	mux.HandleFunc("/ws", hub.ServeWs)         // WebSocket 端点
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(st.GetStateSnapshot())
	})
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var p types.Product
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			logger.Warn("解析任务请求失败", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if p.ID == "" {
			p.ID = "API_ORDER_" + time.Now().Format("150405.000")
		}
		scheduler.SubmitTask(&p)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted", "id": p.ID})
	})

	// 托管前端静态文件
	fs := http.FileServer(http.Dir("./web/static"))
	mux.Handle("/", fs)

	logger.Info("API 和前端服务器启动在 :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		logger.Error("API 服务器启动失败", "error", err)
	}
}

// waitForShutdown 等待系统信号以实现优雅停机
func waitForShutdown(logger *slog.Logger, cancel context.CancelFunc, scheduler *engine.Scheduler) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	logger.Info("接收到停机信号，正在优雅关闭...")
	cancel()                      // 通知所有 goroutine 停止
	scheduler.WaitForCompletion() // 等待所有任务完成
	logger.Info("生产演示结束，系统已安全退出。")
}
