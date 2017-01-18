/**
* @Author fansichi@qq.com
* @CreateData 2016/8/31
* @Desc 通用日志类
* log Level:
*    16 所有级别的日志都记录，即Debug\Trace\Notice\Warning\Fatal
*     8 记录 Trace\Notice\Warning\Fatal 级别的日志
      4 记录 Notice\Warning\Fatal 级别的日志
      2 记录 Warning\Fatal 级别的日志
      1 记录 Fatal 级别的日志
  log Type:
      1  日志都记录到单个文件
      2  日志根据日期生成文件
      3  日志根据小时生成文件
  log root dir:
      系统日志的根目录
  App:
      逻辑的概念，一般情况一个系统对应一个app
*  example：
   applog.SetApp("trade")
   applog.SetLevel(applog.LOG_LEVEL_DEBUG)
   applog.SetLogType(applog.LOG_TYPE_DATE)
   applog.SetLogRootDir("D:/project/go/log")

   applog.Debug("Debug", 1)
   applog.Trace("Trace", 2)
   applog.Notice("Notice", 3)
   applog.Warning("Warning", 4)
   applog.Fatal("Fatal", 5)

   result:
   write trace\notice log to the file  D:/project/go/log/trade/trade.log.20160831
   write warning fatal log to the file  D:/project/go/log/trade/trade.log.wf.20160831
*/
package applog

import (
	"fmt"
	"log"
	"os"
	"time"
)

const LOG_LEVEL_DEBUG = 16
const LOG_LEVEL_TRACE = 8
const LOG_LEVEL_NOTICE = 4
const LOG_LEVEL_WARNING = 2
const LOG_LEVEL_FATAL = 1

const LOG_TYPE_ALL = 1
const LOG_TYPE_DATE = 2
const LOG_TYPE_HOUR = 3

var logger *Logger

func SetLogRootDir(rootDir string) {
	Instance().LogRootDir = rootDir
}
func SetLogType(logType int) {
	Instance().LogType = logType
}
func SetApp(app string) {
	Instance().App = app
}
func SetLevel(level int) {
	Instance().Level = level
}

func DebugPrintln(v ...interface{}) {
	Instance().DebugPrintln(v...)
}
func DebugPrint(v ...interface{}) {
	Instance().DebugPrint(v...)
}
func DebugPrintf(format string, v ...interface{}) {
	Instance().DebugPrintf(format, v...)
}
func Debug(message string, code int) {
	Instance().Debug(message, code)
}
func Trace(message string, code int) {
	Instance().Trace(message, code)
}
func Notice(message string, code int) {
	Instance().Notice(message, code)
}
func Warning(message string, code int) {
	Instance().Warning(message, code)
}
func Fatal(message string, code int) {
	Instance().Fatal(message, code)
}

func Instance() *Logger {
	if logger == nil {
		logger = new(Logger)
	}
	return logger
}

type Logger struct {
	Level      int
	App        string
	LogRootDir string
	LogType    int
	isWf       bool
}

func (l *Logger) DebugPrintln(v ...interface{}) {
	if l.Level >= LOG_LEVEL_DEBUG {
		l.isWf = false
		log.Println(v...)
	}
}
func (l *Logger) DebugPrint(v ...interface{}) {
	if l.Level >= LOG_LEVEL_DEBUG {
		l.isWf = false
		log.Print(v...)
	}
}
func (l *Logger) DebugPrintf(format string, v ...interface{}) {
	if l.Level >= LOG_LEVEL_DEBUG {
		l.isWf = false
		log.Printf(format, v...)
	}
}

func (l *Logger) Debug(message string, code int) {
	if l.Level >= LOG_LEVEL_DEBUG {
		l.isWf = false
		//pc, _, _, _ := runtime.Caller(0)
		l.writeLog(message, code, "DEBUG")
	}
}
func (l *Logger) Trace(message string, code int) {
	if l.Level >= LOG_LEVEL_TRACE {
		l.isWf = false
		l.writeLog(message, code, "TRACE")
	}
}
func (l *Logger) Notice(message string, code int) {
	if l.Level >= LOG_LEVEL_NOTICE {
		l.isWf = false
		l.writeLog(message, code, "NOTICE")
	}
}

func (l *Logger) Warning(message string, code int) {
	if l.Level >= LOG_LEVEL_WARNING {
		l.isWf = true
		l.writeLog(message, code, "WARNING")
	}
}

func (l *Logger) Fatal(message string, code int) {
	if l.Level >= LOG_LEVEL_FATAL {
		l.isWf = true
		l.writeLog(message, code, "FATAL")
	}
}

/**
 * raw  write log
 */
func (l *Logger) writeLog(message string, code int, prefix string) {
	var logDir, logPath string
	var perm os.FileMode = 0666

	logDir = l.LogRootDir + "/" + l.App
	_, err := os.Stat(logDir)
	if err != nil && os.IsNotExist(err) {
		os.MkdirAll(logDir, perm)
	}

	logPath = logDir + "/" + l.App + ".log"
	if l.isWf {
		logPath += ".wf"
	}
	now := time.Now()
	if l.LogType == LOG_TYPE_DATE {
		logPath += now.Format(".20060102")
	} else if l.LogType == LOG_TYPE_HOUR {
		logPath += now.Format(".2006010215")
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND, perm)
	if err != nil {
		fmt.Printf("logfile[%s] create failed. err=%s \n", logPath, err.Error())
	}
	log.SetOutput(file)
	log.SetFlags(0)

	strNow := now.Format("2006-01-02 15:04:05")
	log.Printf("%s: %s code=%d %s\n", prefix, strNow, code, message)
}
