package tuihub

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/go-kratos/kratos/v2/log"
)

const DefaultCallerKey = "logger"

func getCaller() string {
	pc, _, _, _ := runtime.Caller(2) //nolint:gomnd //get external caller
	return runtime.FuncForPC(pc).Name()
}

func NewWriter(level log.Level) io.Writer {
	return log.NewWriter(log.GetLogger(), log.WithWriterLevel(level))
}

// Debug logs a message at debug level.
func Debug(a ...interface{}) {
	log.Log(log.LevelDebug, DefaultCallerKey, getCaller(), log.DefaultMessageKey, fmt.Sprint(a...))
}

// Debugf logs a message at debug level.
func Debugf(format string, a ...interface{}) {
	log.Log(log.LevelDebug, DefaultCallerKey, getCaller(), log.DefaultMessageKey, fmt.Sprintf(format, a...))
}

// Info logs a message at info level.
func Info(a ...interface{}) {
	log.Log(log.LevelInfo, DefaultCallerKey, getCaller(), log.DefaultMessageKey, fmt.Sprint(a...))
}

// Infof logs a message at info level.
func Infof(format string, a ...interface{}) {
	log.Log(log.LevelInfo, DefaultCallerKey, getCaller(), log.DefaultMessageKey, fmt.Sprintf(format, a...))
}

// Warn logs a message at warn level.
func Warn(a ...interface{}) {
	log.Log(log.LevelWarn, DefaultCallerKey, getCaller(), log.DefaultMessageKey, fmt.Sprint(a...))
}

// Warnf logs a message at warnf level.
func Warnf(format string, a ...interface{}) {
	log.Log(log.LevelWarn, DefaultCallerKey, getCaller(), log.DefaultMessageKey, fmt.Sprintf(format, a...))
}

// Error logs a message at error level.
func Error(a ...interface{}) {
	log.Log(log.LevelError, DefaultCallerKey, getCaller(), log.DefaultMessageKey, fmt.Sprint(a...))
}

// Errorf logs a message at error level.
func Errorf(format string, a ...interface{}) {
	log.Log(log.LevelError, DefaultCallerKey, getCaller(), log.DefaultMessageKey, fmt.Sprintf(format, a...))
}

// Fatal logs a message at fatal level.
func Fatal(a ...interface{}) {
	log.Log(log.LevelFatal, DefaultCallerKey, getCaller(), log.DefaultMessageKey, fmt.Sprint(a...))
	os.Exit(1)
}

// Fatalf logs a message at fatal level.
func Fatalf(format string, a ...interface{}) {
	log.Log(log.LevelFatal, DefaultCallerKey, getCaller(), log.DefaultMessageKey, fmt.Sprintf(format, a...))
	os.Exit(1)
}
