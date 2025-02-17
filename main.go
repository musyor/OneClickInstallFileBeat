package main

import (
	"OneClickInstallFileBeat/internal/config"
	_ "embed"
	"fmt"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"os"
	"os/exec"
	"runtime"
)

//go:embed embed/filebeat-8.15.3-x86_64.rpm
var filebeatRPM []byte

//go:embed embed/filebeat-8.15.3-amd64.deb
var filebeatDEB []byte

var (
	logger     *zap.SugaredLogger
	configPath string
)

func init() {
	// 初始化日志记录器
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	cfg.DisableStacktrace = true
	l, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	logger = l.Sugar()
}

func addInput(c *cli.Context) error {

	configPath := c.String("config")
	paths := c.StringSlice("paths")
	project := c.String("project")
	filetype := c.String("type")

	//读取现有配置
	cfg, err := config.ReadConfig(configPath)
	if err != nil {
		logger.Fatal("Failed to read config", err)
	}

	//创建新的Input配置
	newInput := config.InputConfig{
		Type:          "log",
		Enabled:       true,
		RecursiveGlob: config.RecursiveGlob{Enabled: true},
		Paths:         paths,
		Fields: config.InputFields{
			ProjectName: project,
			FileType:    filetype,
		},
	}

	//添加新配置
	cfg.Filebeat.Inputs = append(cfg.Filebeat.Inputs, newInput)

	err = config.ValidateConfig(cfg)
	if err != nil {
		logger.Fatal("invalid config", err)
	}
	//保存新配置
	err = config.WriteConfig(cfg, configPath)
	if err != nil {
		logger.Fatal("Failed to write config", err)
	}
	logger.Info("New input added", "project", project, "filetype", filetype, "paths", paths)

	return nil
}

func main() {
	app := &cli.App{
		Name:  "fbctl",
		Usage: "Filebeat Configuration management Tool",

		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "/etc/filebeat/filebeat.yml",
			},
		},
		//定义命令
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "Initialize new configuration",
				Action: func(c *cli.Context) error {
					return initConfig(c.String("config"))
				},
				Aliases: []string{"i"},
			},
			{
				Name:  "add-input",
				Usage: "Add new log input",
				Action: func(context *cli.Context) error {
					return addInput(context)
				},
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:     "path",
						Usage:    "Log file path",
						Required: true,
					},
					&cli.StringSliceFlag{
						Name:     "project",
						Usage:    "Project Name",
						Required: true,
					},
					&cli.StringSliceFlag{
						Name:     "type",
						Usage:    "Input Type",
						Required: true,
					},
				},
			},
			{
				Name:  "remove-input",
				Usage: "Remove log input",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:     "paths",
						Usage:    "Log file paths (comma separated)",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					return removeInput(c)
				},
			},
			{
				Name:  "update-input",
				Usage: "Update log input paths",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:     "old-paths",
						Usage:    "Old log file paths (comma separated)",
						Required: true,
					},
					&cli.StringSliceFlag{
						Name:     "new-paths",
						Usage:    "New log file paths (comma separated)",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					return updateInput(c)
				},
			},
		},
	}

	//初始化日志记录器
	logger.Info("Filebeat Manager stated")

	// 安装 Filebeat
	if os.Getenv("INSTALL_FILEBEAT") == "true" {
		err := installFilebeat()
		if err != nil {
			logger.Fatal("Failed to install Filebeat", zap.Error(err))
		}
	}

	// 启动 Filebeat
	if os.Getenv("START_FILEBEAT") == "true" {
		cmd := exec.Command("filebeat", "-e")
		err := cmd.Run()
		if err != nil {
			logger.Fatal("Failed to start Filebeat", zap.Error(err))
		}
	}

	// 验证配置文件
	configPath := "./filebeat.yml"
	cfg, err := config.ReadConfig(configPath)
	if err != nil {
		logger.Fatal("Failed to read config", zap.Error(err))
	}

	err = config.ValidateConfig(cfg)
	if err != nil {
		logger.Fatal("Invalid config", zap.Error(err))
	}

	err = app.Run(os.Args)
	if err != nil {
		logger.Fatal("Application failed", err)
	}

}

func initConfig(path string) error {
	//创建默认配置
	defaultCfg := &config.FilebeatConfig{
		Filebeat: struct {
			Inputs     []config.InputConfig `yaml:"inputs"`
			Processors []config.Processor   `yaml:"processors"`
		}{
			Inputs: []config.InputConfig{
				{
					Type:    "log",
					Enabled: true,
					RecursiveGlob: config.RecursiveGlob{
						Enabled: true,
					},
					Paths: []string{"/var/log/secure"},
					Fields: config.InputFields{
						ProjectName: "centos-logs",
						FileType:    "secure",
					},
				},
				{
					Type:    "log",
					Enabled: true,
					RecursiveGlob: config.RecursiveGlob{
						Enabled: true,
					},
					Paths: []string{"/var/log/audit/audit.log"},
					Fields: config.InputFields{
						ProjectName: "centos-logs",
						FileType:    "audit",
					},
				},
			},
			Processors: []config.Processor{
				{
					AddHostMetadata: map[string]interface{}{},
				},
			},
		},
		Output: struct {
			Kafka config.KafkaConfig `yaml:"kafka"`
		}{
			Kafka: config.KafkaConfig{
				Enabled:         true,
				Hosts:           []string{"172.1.200.75:9092", "172.1.200.76:9092", "172.1.200.77:9092"},
				Topic:           "centosin_log_topic",
				RequiredAcks:    1,
				BulkMaxSize:     40960,
				MaxMessageBytes: 1000000,
			},
		},
		Logging: config.LoggingConfig{
			Level:   "info",
			ToFiles: true,
			Files: config.LogFiles{
				Path:        "/var/log/filebeat",
				Name:        "filebeat.log",
				KeepFiles:   7,
				Permissions: "0644",
			},
		},
		Setup: struct {
			TemplateEnabled bool `yaml:"template.enabled"`
			ILMEnabled      bool `yaml:"ilm.enabled"`
		}{
			TemplateEnabled: false,
			ILMEnabled:      false,
		},
		Fields: config.GlobalFields{
			LogType: "windows",
		},
	}

	err := config.ValidateConfig(defaultCfg)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	err = config.WriteConfig(defaultCfg, path)
	if err != nil {
		logger.Error("failed to write config", err)
		return err
	}

	logger.Info("Configuration initialized successfully", err)
	return nil
}

func removeInput(c *cli.Context) error {
	configPath := c.String("config")
	paths := c.StringSlice("paths")

	// 读取现有配置
	cfg, err := config.ReadConfig(configPath)
	if err != nil {
		logger.Fatal("Failed to read config", err)
	}

	// 删除指定路径的输入
	newInputs := []config.InputConfig{}
	for _, input := range cfg.Filebeat.Inputs {
		match := false
		for _, path := range paths {
			if contains(input.Paths, path) {
				match = true
				break
			}
		}
		if !match {
			newInputs = append(newInputs, input)
		}
	}

	cfg.Filebeat.Inputs = newInputs

	// 保存配置文件
	err = config.WriteConfig(cfg, configPath)
	if err != nil {
		logger.Fatal("Failed to write config", err)
	}

	logger.Info("Inputs removed", "paths", paths)
	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func updateInput(c *cli.Context) error {
	configPath := c.String("config")
	oldPaths := c.StringSlice("old-paths")
	newPaths := c.StringSlice("new-paths")

	// 读取现有配置
	cfg, err := config.ReadConfig(configPath)
	if err != nil {
		logger.Fatal("Failed to read config", err)
	}

	// 更新指定路径的输入
	for i, input := range cfg.Filebeat.Inputs {
		for _, oldPath := range oldPaths {
			if contains(input.Paths, oldPath) {
				cfg.Filebeat.Inputs[i].Paths = newPaths
				break
			}
		}
	}

	// 保存配置文件
	err = config.WriteConfig(cfg, configPath)
	if err != nil {
		logger.Fatal("Failed to write config", err)
	}

	logger.Info("Inputs updated", "old_paths", oldPaths, "new_paths", newPaths)
	return nil
}

func installFilebeat() error {
	osType := runtime.GOOS
	switch osType {
	case "linux":
		// 检测是否是基于 RPM 的系统
		if isRPMSystem() {
			return installRPM()
		} else {
			return installDEB()
		}
	default:
		return fmt.Errorf("unsupported OS: %s", osType)
	}
}

func isRPMSystem() bool {
	// 简单检测是否存在 RPM 包管理器
	_, err := exec.LookPath("rpm")
	return err == nil
}

func installRPM() error {
	rpmFile := "filebeat.rpm"
	defer os.Remove(rpmFile)

	err := os.WriteFile(rpmFile, filebeatRPM, 0644)
	if err != nil {
		return fmt.Errorf("failed to write RPM file: %w", err)
	}

	cmd := exec.Command("rpm", "-i", rpmFile)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install RPM package: %w", err)
	}

	return nil
}

func installDEB() error {
	debFile := "filebeat.deb"
	defer os.Remove(debFile)

	err := os.WriteFile(debFile, filebeatDEB, 0644)
	if err != nil {
		return fmt.Errorf("failed to write DEB file: %w", err)
	}

	cmd := exec.Command("dpkg", "-i", debFile)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install DEB package: %w", err)
	}

	return nil
}
