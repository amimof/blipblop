package logger

import (
	"log"
	"runtime"
	"strings"
)

type Logger interface {
	Debug(string, ...interface{})
	Info(string, ...interface{})
	Warn(string, ...interface{})
	Error(string, ...interface{})
}

type ConsoleLogger struct{}

func (c ConsoleLogger) Debug(msg string, fields ...interface{}) {}
func (c ConsoleLogger) Info(msg string, fields ...interface{})  {}
func (c ConsoleLogger) Warn(msg string, fields ...interface{})  {}
func (c ConsoleLogger) Error(msg string, fields ...interface{}) {}

func (c *ConsoleLogger) log(level, msg string, fields ...interface{}) {
	file, line, funcName := getCallerInfo(2)
	log.Printf("[%s] %s:%d %s() - "+msg, append([]interface{}{level, file, line, funcName}, fields...)...)
}

// getCallerInfo gets the file, line, and function name of the caller
func getCallerInfo(skip int) (string, int, string) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown_file", 0, "unknown_func"
	}

	// Get function name
	funcName := runtime.FuncForPC(pc).Name()
	funcName = trimFunctionName(funcName)

	// Trim the file path to only the base name
	fileParts := strings.Split(file, "/")
	file = fileParts[len(fileParts)-1]

	return file, line, funcName
}

func trimFunctionName(funcName string) string {
	funcParts := strings.Split(funcName, "/")
	return funcParts[len(funcParts)-1]
}
