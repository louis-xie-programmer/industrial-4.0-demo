package types

// StationID 定义工站 ID
type StationID string

const (
	StationEntry    StationID = "ENTRY_STATION"    // 上料站
	StationAssemble StationID = "ASSEMBLE_STATION" // 组装站
	StationQC       StationID = "QC_STATION"       // 质检站
	StationExit     StationID = "EXIT_STATION"     // 下料站
)

// Product 表示生产线上的工件
type Product struct {
	ID       string
	Priority int      // 优先级：0-普通, 1-加急, 2-紧急(Top Secret)
	Step     int      // 当前步骤索引
	History  []string // 加工历史记录
	Status   string   // "OK" 或 "NG" (Not Good)
}

// Result 任务执行结果
type Result struct {
	ProductID string
	Success   bool
	Error     error
}
