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
		Commands: []*cli.Command{
			{
				Name:    "init",
				Aliases: []string{"i"},
				Usage:   "Initialize new configuration",
				Action: func(c *cli.Context) error {
					return initConfig(c.String("config"))
				},
			},
			{
				Name:  "add-input",
				Usage: "Add new log input",
				Action: func(context *cli.Context) error {
					return addInput(context)
				},
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:     "paths",
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
				Name:   "remove-input",
				Usage:  "Remove log input",
				Flags:  []cli.Flag{},
				Action: func(c *cli.Context) error { return removeInput(c) },
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
			{
				Name:  "install",
				Usage: "Install Filebeat, initialize config, and start it",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:        "start",
						Aliases:     []string{"s"},
						DefaultText: "true",
						Value:       true,
						Usage:       "Whether to start Filebeat after installation",
					},
				},
				Action: func(c *cli.Context) error {
					err := installFilebeat()
					if err != nil {
						logger.Fatal("Failed to install Filebeat", err)
						return err
					}

					err = initConfig(c.String("config"))
					if err != nil {
						logger.Fatal("Failed to initialize config", zap.Error(err))
						return err
					}

					if c.Bool("start") {
						err = startFilebeatWithSystemctl()
						if err != nil {
							logger.Error("Failed to start Filebeat with systemctl", err)
							return err
						}
					}

					return nil
				},
			},
			{
				Name:  "start-filebeat",
				Usage: "Start Filebeat with systemctl",
				Action: func(c *cli.Context) error {
					return startFilebeatWithSystemctl()
				},
			},
		},
	}

	logger.Info("Filebeat Manager started")

	err := app.Run(os.Args)
	if err != nil {
		logger.Fatal("Application failed", zap.Error(err))
	}
}

func initConfig(path string) error {
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
					AddHostMetadata: struct{}{},
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
		logger.Fatal("Failed to write config", zap.Error(err))
		return err
	}

	logger.Info("Configuration initialized successfully", zap.String("config_path", path))
	return nil
}

func removeInput(c *cli.Context) error {
	configPath := c.String("config")
	paths := c.StringSlice("paths")

	cfg, err := config.ReadConfig(configPath)
	if err != nil {
		logger.Fatal("Failed to read config", err)
	}

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

	cfg, err := config.ReadConfig(configPath)
	if err != nil {
		logger.Fatal("Failed to read config", err)
	}

	for i, input := range cfg.Filebeat.Inputs {
		for _, oldPath := range oldPaths {
			if contains(input.Paths, oldPath) {
				cfg.Filebeat.Inputs[i].Paths = newPaths
				break
			}
		}
	}

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

func startFilebeatWithSystemctl() error {
	cmd := exec.Command("systemctl", "restart", "filebeat")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start Filebeat with systemctl: %w", err)
	}
	return nil
}
