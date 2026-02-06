package tui

import (
	"fmt"
	"os"
	"time"
)

var debugFile *os.File

func init() {
	f, err := os.OpenFile("/tmp/c64u-debug.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	debugFile = f
	DebugLog("=== Session Started ===")
}

func DebugLog(format string, args ...interface{}) {
	if debugFile == nil {
		return
	}
	t := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(debugFile, "[%s] %s\n", t, msg)
	// Force synchronization to ensure content is written immediately
	debugFile.Sync()
}
