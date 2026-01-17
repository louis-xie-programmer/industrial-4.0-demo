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

	// 2. 注册事件处理器
	handlers.RegisterEventHandlers(eventBus, stateTracker, logger)

	// 3. 初始化引擎和调度器
	wf := engine.NewWorkflowEngine(cfg.Workflows, cfg.ResourcePools, logger, eventBus, cfg.StepDelayMs)
	registerStations(wf, logger)

	scheduler := engine.NewScheduler(wf, cfg.MaxWorkers, wal, stateTracker, logger)

	// 4. 恢复和启动
	if err := scheduler.RecoverTasks(); err != nil {
		logger.Warn("从 WAL 恢复任务失败", "error", err)
	}

	logger.Info("=== PCB 智能工厂调度系统启动 ===")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go scheduler.Start(ctx)
	go startAPIServer(scheduler, hub, stateTracker, logger)
	go simulateTasks(ctx, scheduler)

	// 5. 优雅停机
	waitForShutdown(logger, cancel, scheduler)
}

// registerStations 注册所有可用的工站
func registerStations(wf *engine.WorkflowEngine, logger *slog.Logger) {
	wf.RegisterStation(station.NewStation(types.StationCAM, logger))
	wf.RegisterStation(station.NewStation(types.StationDrill, logger))
	wf.RegisterStation(station.NewStation(types.StationLami, logger))
	wf.RegisterStation(station.NewStation(types.StationEtch, logger))
	wf.RegisterStation(station.NewStation(types.StationMask, logger))
	wf.RegisterStation(station.NewStation(types.StationSilk, logger))
	wf.RegisterStation(station.NewStation(types.StationETest, logger))
	wf.RegisterStation(station.NewStation(types.StationPack, logger))

	remoteAddr := os.Getenv("REMOTE_STATION_ADDR")
	if remoteAddr == "" {
		remoteAddr = "http://localhost:9090"
	}
	wf.RegisterStation(station.NewRemoteStation(types.StationAOI, remoteAddr, logger))
}

// simulateTasks 模拟提交初始订单
func simulateTasks(ctx context.Context, scheduler *engine.Scheduler) {
	info, _ := os.Stat(walPath)
	if info != nil && info.Size() > 100 {
		return
	}

	tasks := []types.Product{
		{ID: "PCB_Double_001", Type: "PCB_DOUBLE_LAYER", Priority: 0, Attrs: map[string]interface{}{"layers": 2}},
		{ID: "PCB_Multi_4L_001", Type: "PCB_MULTILAYER", Priority: 1, Attrs: map[string]interface{}{"layers": 4}},
		{ID: "PCB_Proto_Fast", Type: "PCB_PROTOTYPE", Priority: 2, Attrs: map[string]interface{}{"layers": 2}},
		{ID: "PCB_Double_002", Type: "PCB_DOUBLE_LAYER", Priority: 0, Attrs: map[string]interface{}{"layers": 2}},
		{ID: "PCB_Multi_8L_001", Type: "PCB_MULTILAYER", Priority: 1, Attrs: map[string]interface{}{"layers": 8}},
		{ID: "PCB_Double_003", Type: "PCB_DOUBLE_LAYER", Priority: 0, Attrs: map[string]interface{}{"layers": 2}},
	}

	for _, task := range tasks {
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
			scheduler.SubmitTask(&task)
		}
	}
}

// startAPIServer 启动 API 和 Web 服务器
func startAPIServer(scheduler *engine.Scheduler, hub *web.Hub, st *web.StateTracker, logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/ws", hub.ServeWs)
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
	cancel()
	scheduler.WaitForCompletion()
	logger.Info("生产演示结束，系统已安全退出。")
}
