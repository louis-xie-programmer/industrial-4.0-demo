package config

import (
	"industrial-4.0-demo/internal/types"
)

type Config struct {
	MaxWorkers int
	Workflows  map[string][]types.WorkflowStep
}

// LoadConfig 模拟加载配置，实际项目中可能从文件或环境变量加载
func LoadConfig() *Config {
	return &Config{
		MaxWorkers: 2,
		Workflows: map[string][]types.WorkflowStep{
			"STANDARD": {
				{StationIDs: []types.StationID{types.StationEntry}},
				{StationIDs: []types.StationID{types.StationAssemble}},
				{StationIDs: []types.StationID{types.StationQC}},
				{StationIDs: []types.StationID{types.StationExit}},
			},
			"SIMPLE": {
				{StationIDs: []types.StationID{types.StationEntry}},
				{StationIDs: []types.StationID{types.StationAssemble}},
				{StationIDs: []types.StationID{types.StationExit}},
			},
			"STRICT": {
				{StationIDs: []types.StationID{types.StationEntry}},
				{StationIDs: []types.StationID{types.StationAssemble}},
				{StationIDs: []types.StationID{types.StationQC}},
				{StationIDs: []types.StationID{types.StationQC}}, // 双重质检
				{StationIDs: []types.StationID{types.StationExit}},
			},
			"PARALLEL_DEMO": {
				{StationIDs: []types.StationID{types.StationEntry}},
				// 并行步骤：组装的同时进行喷涂
				{StationIDs: []types.StationID{types.StationAssemble, types.StationPaint}},
				{StationIDs: []types.StationID{types.StationDry}},
				{StationIDs: []types.StationID{types.StationQC}},
				{StationIDs: []types.StationID{types.StationExit}},
			},
		},
	}
}
