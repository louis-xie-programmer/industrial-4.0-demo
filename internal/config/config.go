package config

import (
	"fmt"
	"industrial-4.0-demo/internal/types"

	"github.com/spf13/viper"
)

// Config 定义应用程序的配置结构
// 使用 mapstructure 标签来映射配置文件中的字段
type Config struct {
	MaxWorkers    int                             `mapstructure:"max_workers"`    // 全局最大并发工人数
	StepDelayMs   int                             `mapstructure:"step_delay_ms"`  // 步骤之间的移动延时
	Workflows     map[string][]types.WorkflowStep `mapstructure:"workflows"`      // 工艺流程定义，Key 为产品类型
	ResourcePools map[types.StationID]int         `mapstructure:"resource_pools"` // 资源池配置，Key 为工站 ID，Value 为资源数量
}

// LoadConfig 从 config.yaml 文件加载配置
// 使用 Viper 库来读取和解析配置文件
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config") // 配置文件名称 (不带扩展名)
	viper.SetConfigType("yaml")   // 配置文件类型
	viper.AddConfigPath(".")      // 查找配置文件的路径 (当前目录)

	// 设置默认值
	viper.SetDefault("step_delay_ms", 500)

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 将配置解析到结构体中
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}
