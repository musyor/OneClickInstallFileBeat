package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

type FilebeatConfig struct {
	Filebeat struct {
		Inputs     []InputConfig `yaml:"inputs"`
		Processors []Processor   `yaml:"processors"`
	} `yaml:"filebeat"`

	Output struct {
		Kafka KafkaConfig `yaml:"kafka"`
	} `yaml:"output"`

	Logging LoggingConfig `yaml:"logging"`

	Setup struct {
		TemplateEnabled bool `yaml:"template.enabled"`
		ILMEnabled      bool `yaml:"ilm.enabled"`
	} `yaml:"setup"`

	Fields GlobalFields `yaml:"fields"`
}

type InputConfig struct {
	Type          string        `yaml:"type"`
	Enabled       bool          `yaml:"enabled"`
	RecursiveGlob RecursiveGlob `yaml:"recursive_glob"`
	Paths         []string      `yaml:"paths"`
	Fields        InputFields   `yaml:"fields"`
	Multiline     *Multiline    `yaml:"multiline,omitempty"`
}

// Processor 表示一个处理器
type Processor struct {
	AddHostMetadata struct{} `yaml:"add_host_metadata"`
}

type KafkaConfig struct {
	Enabled         bool     `yaml:"enabled"`
	Hosts           []string `yaml:"hosts"`
	Topic           string   `yaml:"topic"`
	RequiredAcks    int      `yaml:"required_acks"`
	BulkMaxSize     int      `yaml:"bulk_max_size"`
	MaxMessageBytes int      `yaml:"max_message_bytes"`
}

type LoggingConfig struct {
	Level   string   `yaml:"level"`
	ToFiles bool     `yaml:"to_files"`
	Files   LogFiles `yaml:"files"`
}

type GlobalFields struct {
	LogType string `yaml:"log_type"`
}

type RecursiveGlob struct {
	Enabled bool `yaml:"enabled"`
}

type InputFields struct {
	ProjectName string `yaml:"projectname"`
	FileType    string `yaml:"filetype"`
}

type Multiline struct {
	Pattern  string `yaml:"pattern"`
	Negate   bool   `yaml:"negate"`
	Match    string `yaml:"match"`
	MaxLines int    `yaml:"max_lines"`
	Timeout  string `yaml:"timeout"`
}

type LogFiles struct {
	Path        string `yaml:"path"`
	Name        string `yaml:"name"`
	KeepFiles   int    `yaml:"keepfiles"`
	Permissions string `yaml:"permissions"`
}

// ReadConfig 读取配置文件
func ReadConfig(path string) (*FilebeatConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg FilebeatConfig
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

//保存配置至yaml文件中

func WriteConfig(cfg *FilebeatConfig, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)

}

// ValidateConfig 验证配置文件
func ValidateConfig(cfg *FilebeatConfig) error {
	//// 验证 Filebeat 配置
	//if cfg.Filebeat == nil {
	//	return fmt.Errorf("filebeat configuration is missing")
	//}

	// 验证 Inputs
	if len(cfg.Filebeat.Inputs) == 0 {
		return fmt.Errorf("no inputs configured")
	}

	for i, input := range cfg.Filebeat.Inputs {
		if input.Type == "" {
			return fmt.Errorf("input type is missing for input %d", i)
		}
		if input.Enabled && len(input.Paths) == 0 {
			return fmt.Errorf("no paths configured for input %d", i)
		}
	}

	//// 验证 Output
	//if cfg.Output == nil {
	//	return fmt.Errorf("output configuration is missing")
	//}

	// 验证 Kafka 配置
	if cfg.Output.Kafka.Enabled {
		if len(cfg.Output.Kafka.Hosts) == 0 {
			return fmt.Errorf("no Kafka hosts configured")
		}
		if cfg.Output.Kafka.Topic == "" {
			return fmt.Errorf("Kafka topic is missing")
		}
	}
	//
	//// 验证 Logging 配置
	//if cfg.Logging == nil {
	//	return fmt.Errorf("logging configuration is missing")
	//}

	return nil
}
