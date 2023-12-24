package youyoproxy

import (
	"fmt"
	"log"
	"time"
)

const (
	colorInfo     = "\033[32m"
	colorWarn     = "\033[33m"
	colorError    = "\033[31m"
	colorDebugger = "\033[37m"
	colorReset    = "\033[0m"
)

func init() {
	log.SetFlags(0)
}

func (proxy *HttpProxy) Info(format string, v ...any) {
	format = prefix("info", colorInfo) + format + "\n"
	log.Printf(format, v...)
}

func (proxy *HttpProxy) Warn(format string, v ...any) {
	format = prefix("warn", colorWarn) + format + "\n"
	log.Printf(format, v...)
}

func (proxy *HttpProxy) Error(format string, v ...any) {
	format = prefix("error", colorError) + format + "\n"
	log.Printf(format, v...)
}

func (proxy *HttpProxy) Debugger(format string, v ...any) {

	if !proxy.IsDebugger {
		return
	}

	format = prefix("debugger", colorDebugger) + format + "\n"
	log.Printf(format, v...)
}

func prefix(t string, color string) string {
	format := time.Now().Format("2006-01-02 15:04:05")
	return fmt.Sprintf("%s[%-8s] %s %s ", color, t, format, colorReset)
}
