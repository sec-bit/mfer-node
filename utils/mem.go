package utils

import (
	"runtime"

	"github.com/kataras/golog"
)

func PrintMemUsage(note string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	golog.Infof("%s Alloc = %v MiB\tTotalAlloc = %v MiB\tSys = %v MiB\tNumGC = %v", note, bToMb(m.Alloc), bToMb(m.TotalAlloc), bToMb(m.Sys), m.NumGC)
}
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
