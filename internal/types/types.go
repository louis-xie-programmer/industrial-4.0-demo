package types

// StationID 定义工站 ID
// 使用字符串类型，方便在日志和配置中直接使用
type StationID string

const (
	// PCB 生产工站常量定义
	StationCAM   StationID = "STATION_CAM"    // CAM 工程站 (入口)：负责文件处理和工艺预审
	StationDrill StationID = "STATION_DRILL"  // 数控钻孔机：负责板材钻孔
	StationLami  StationID = "STATION_LAMI"   // 层压机：负责多层板的压合 (多层板专用)
	StationEtch  StationID = "STATION_ETCH"   // 蚀刻线：负责形成线路图形
	StationMask  StationID = "STATION_MASK"   // 阻焊涂布机：负责涂布绿油
	StationSilk  StationID = "STATION_SILK"   // 丝印机：负责打印字符
	StationAOI   StationID = "STATION_AOI"    // AOI 光学检测仪 (远程工站)：负责线路缺陷检测
	StationETest StationID = "STATION_E_TEST" // 飞针测试机 (资源瓶颈)：负责电气性能测试
	StationPack  StationID = "STATION_PACK"   // 包装机 (出口)：负责最终包装
)

// WorkflowStep 定义工作流中的一个步骤
// 一个步骤可以包含一个或多个工站（并行执行），也可以包含执行规则
type WorkflowStep struct {
	StationIDs []StationID `mapstructure:"station_ids"`    // 该步骤包含的工站 ID 列表，多个 ID 表示并行执行
	Rule       string      `mapstructure:"rule,omitempty"` // 执行该步骤的规则表达式 (expr 语法)，为空则默认执行
}

// Product 表示生产线上的工件 (PCB 板)
type Product struct {
	ID       string                 // 工件唯一标识
	Type     string                 // 产品类型: PCB_DOUBLE_LAYER, PCB_MULTILAYER, PCB_PROTOTYPE
	Priority int                    // 优先级：数值越大优先级越高
	Step     int                    // 当前步骤索引，用于流程控制
	History  []string               // 加工历史记录，存储经过的工站 ID
	Status   string                 // 当前状态，由 FSM 管理 (e.g., PROCESSING, COMPLETED)
	FSM      interface{}            `json:"-"`               // 运行时绑定的 FSM 实例，不参与 JSON 序列化
	Attrs    map[string]interface{} `json:"attrs,omitempty"` // 动态属性，用于规则引擎决策 (e.g., layers: 4, is_fragile: true)
}

// Result 表示工站任务执行的结果
type Result struct {
	ProductID string // 关联的工件 ID
	Success   bool   // 是否执行成功
	Error     error  // 如果失败，存储错误信息
}
