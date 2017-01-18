package applog

import (
	"testing"
)

//单测
func TestLog(t *testing.T) {
	SetApp("trade")
	SetLevel(LOG_LEVEL_TRACE)
	SetLogType(LOG_TYPE_DATE)
	SetLogRootDir("D:/project/go/log")

	Debug("Debug", 1)
	Trace("Trace", 2)
	Notice("Notice", 3)
	Warning("Warning", 4)
	Fatal("Fatal", 5)

	SetLevel(LOG_LEVEL_DEBUG)
	SetLogType(LOG_TYPE_HOUR)

	Debug("Debug2", 1)
	Trace("Trace2", 2)
	Notice("Notice2", 3)
	Warning("Warning2", 4)
	Fatal("Fatal2", 5)

	SetLevel(LOG_LEVEL_NOTICE)
	SetLogType(LOG_TYPE_HOUR)

	Debug("Debug3", 1)
	Trace("Trace3", 2)
	Notice("Notice3", 3)
	Warning("Warning3", 4)
	Fatal("Fatal3", 5)

}
