package sensitiveword

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// NewLogger 构造一个仅供敏感词命中事件使用的独立 zap logger，输出到 path 指定的
// 文件，按 lumberjack 默认策略轮转。path 为空时返回 (nil, nil) —— 调用方应据此
// 视为禁用记录。返回错误仅在创建目录失败时出现。
func NewLogger(path string) (*zap.Logger, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir sensitive word log dir: %w", err)
	}
	lj := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    50,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
		LocalTime:  true,
	}
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encCfg), zapcore.AddSync(lj), zapcore.InfoLevel)
	return zap.New(core), nil
}
