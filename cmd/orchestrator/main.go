package main

import (
	"context"
	"fmt"
	"industrial-4.0-demo/internal/config"
	"industrial-4.0-demo/internal/engine"
	"industrial-4.0-demo/internal/station"
	"industrial-4.0-demo/internal/types"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 0. 加载配置
	cfg := config.LoadConfig()

	// 1. 初始化编排引擎与工站
	wf := engine.NewWorkflowEngine(cfg.Sequence)
	wf.RegisterStation(station.NewStation(types.StationEntry))
	wf.RegisterStation(station.NewStation(types.StationAssemble))
	wf.RegisterStation(station.NewStation(types.StationQC))
	wf.RegisterStation(station.NewStation(types.StationExit))

	// 2. 初始化调度器
	scheduler := engine.NewScheduler(wf, cfg.MaxWorkers)

	fmt.Println("=== 工业 4.0 智能调度系统启动 ===")

	// 创建上下文以控制生命周期
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. 在后台启动调度逻辑
	go scheduler.Start(ctx)

	// 4. 模拟不同时间到达的订单
	go func() {
		// 先来两个普通订单
		scheduler.SubmitTask(&types.Product{ID: "Normal_01", Priority: 0})
		scheduler.SubmitTask(&types.Product{ID: "Normal_02", Priority: 0})

		time.Sleep(200 * time.Millisecond)

		// 突然来了一个紧急订单 (VIP)
		scheduler.SubmitTask(&types.Product{ID: "URGENT_EV_BATTERY", Priority: 2})

		// 再来一个加急订单
		scheduler.SubmitTask(&types.Product{ID: "RUSH_ORDER_01", Priority: 1})
	}()

	// 5. 监听系统信号实现优雅停机
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号或超时（演示用）
	select {
	case <-sigChan:
		fmt.Println("\n接收到停机信号，正在优雅关闭...")
	case <-time.After(10 * time.Second):
		fmt.Println("\n演示时间结束，正在关闭...")
	}

	// 触发关闭流程
	cancel()
	scheduler.WaitForCompletion()
	fmt.Println("生产演示结束，系统已安全退出。")
}
