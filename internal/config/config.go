package config

import (
	"fmt"
	"industrial-4.0-demo/internal/types"

	"github.com/spf13/viper"
)

// Config 定义应用程序的配置结构
// 使用 mapstructure 标签来映射配置文件中的字段
type Config struct {
	MaxWorkers     int                             `mapstructure:"max_workers"`
	StepDelayMs    int                             `mapstructure:"step_delay_ms"`
	StationDelayMs int                             `mapstructure:"station_delay_ms"` // 新增：工站处理延时
	Workflows      map[string][]types.WorkflowStep `mapstructure:"workflows"`
	ResourcePools  map[types.StationID]int         `mapstructure:"resource_pools"`
}

// LoadConfig 从 config.yaml 文件加载配置
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// 设置默认值
	viper.SetDefault("step_delay_ms", 500)
	viper.SetDefault("station_delay_ms", 10000) // 默认为 10 秒

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}
