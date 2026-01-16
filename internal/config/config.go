package config

import (
	"industrial-4.0-demo/internal/types"
)

type Config struct {
	MaxWorkers int
	Sequence   []types.StationID
}

// LoadConfig 模拟加载配置，实际项目中可能从文件或环境变量加载
func LoadConfig() *Config {
	return &Config{
		MaxWorkers: 2,
		Sequence: []types.StationID{
			types.StationEntry,
			types.StationAssemble,
			types.StationQC,
			types.StationExit,
		},
	}
}
