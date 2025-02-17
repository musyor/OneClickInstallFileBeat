package logger

import "go.uber.org/zap"

var sugar *zap.SugaredLogger

func init() {

	//配置日志等级
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	cfg.DisableStacktrace = true
	//构建日志记录器
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}

	//创建SugarLogger 以便更好记录日志
	sugar = logger.Sugar()
}

func Info(msg string, fields ...interface{}) {
	sugar.Infow(msg, fields)
}

func Error(msg string, fields ...interface{}) {
	sugar.Errorw(msg, fields)
}

func Fatal(msg string, fields ...interface{}) {
	sugar.Fatalw(msg, fields)
}
