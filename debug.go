package main

import (
	"log"
	"os"
	"runtime"
	"sync"
	"time"
)

var debugLog *log.Logger
var lastActivityTime time.Time
var activityMutex sync.Mutex

func initDebugLog() {
	f, err := os.OpenFile("lancache_spy_debug.log", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return
	}
	debugLog = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
	lastActivityTime = time.Now()

	// Start watchdog - dump stack if no Update activity for 3 seconds
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			activityMutex.Lock()
			timeSinceActivity := time.Since(lastActivityTime)
			activityMutex.Unlock()

			if timeSinceActivity > 3*time.Second {
				debugLog.Printf("!!! WATCHDOG: No activity for %v, dumping stack", timeSinceActivity)
				dumpStackTrace()
			}
		}
	}()
}

func touchActivity() {
	activityMutex.Lock()
	lastActivityTime = time.Now()
	activityMutex.Unlock()
}

func debugf(format string, args ...interface{}) {
	if debugLog != nil {
		debugLog.Printf(format, args...)
		touchActivity()
	}
}

func debugPanic() {
	if r := recover(); r != nil {
		if debugLog != nil {
			debugLog.Printf("PANIC: %v", r)
			dumpStackTrace()
		}
		panic(r)
	}
}

func dumpStackTrace() {
	if debugLog == nil {
		return
	}

	buf := make([]byte, 1024*64)
	n := runtime.Stack(buf, true)
	debugLog.Printf("\n=== STACK TRACE (all goroutines) ===\n%s\n=== END STACK TRACE ===\n", buf[:n])
	debugLog.Printf("Number of goroutines: %d\n", runtime.NumGoroutine())
}
