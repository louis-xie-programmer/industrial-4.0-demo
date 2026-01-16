package types

// StationID 定义工站 ID
type StationID string

const (
	StationEntry    StationID = "ENTRY_STATION"    // 上料站
	StationAssemble StationID = "ASSEMBLE_STATION" // 组装站
	StationPaint    StationID = "PAINT_STATION"    // 喷涂站 (新增，用于并行演示)
	StationDry      StationID = "DRY_STATION"      // 烘干站 (新增，用于并行演示)
	StationQC       StationID = "QC_STATION"       // 质检站
	StationExit     StationID = "EXIT_STATION"     // 下料站
)

// WorkflowStep 定义工作流中的一个步骤，可以是单个工站，也可以是并行的一组工站
type WorkflowStep struct {
	StationIDs []StationID // 如果有多个 ID，则表示并行执行
}

// Product 表示生产线上的工件
type Product struct {
	ID       string
	Type     string      // 产品类型：STANDARD, SIMPLE, STRICT, PARALLEL_DEMO
	Priority int         // 优先级：0-普通, 1-加急, 2-紧急(Top Secret)
	Step     int         // 当前步骤索引
	History  []string    // 加工历史记录
	Status   string      // 由 FSM 管理，这里仅作快照或移除
	FSM      interface{} // 运行时绑定的 FSM 实例，使用 interface{} 避免循环依赖，实际使用时断言
}

// Result 任务执行结果
type Result struct {
	ProductID string
	Success   bool
	Error     error
}
