package logging

import (
	"encoding/json"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

type Level int8

// iota works similar to auto-increment - starting at 0, each successive const is assigned a sequential int. In other words, LevelDebug is a Level set as 0, LevelError is a Level set to 3 etc.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
	LevelOff
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARNING"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return ""
	}
}

type Log struct {
	ID        int            `json:"id,omitempty"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Timestamp time.Time      `json:"timestamp"`
	Service   string         `json:"service,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	Trace     string         `json:"trace,omitempty"`
	UserID    int            `json:"user_id,omitempty"`
	File      string         `json:"file,omitempty"`
	Line      int            `json:"line,omitempty"`
}

// Agnostic LogRepo interface - allows logging package to be imported and implemented by any application, and for logs to optionally be written to a database as long as the db model provides an Insert function with this fingerprint
type LogRepo interface {
	Insert(log *Log) error
}

// The mutex (mutual exclusion lock) prevents two log triggers from trying to write at the same time (which would lead to jumbled log messages). logModel is optional, and provides the ability to simultaneously write logs to the database if logModel exists.
type Logger struct {
	out      io.Writer
	logRepo  LogRepo
	minLevel Level
	mu       sync.Mutex
	service  string
}

// A function wrapper that takes a parameter and sets it as the property of an instantiated Logger
type LoggerOption func(*Logger)

// Logger will send logs to provided database as well as writing to io.Writer
func WithDatabase(repo LogRepo) LoggerOption {
	return func(l *Logger) {
		l.logRepo = repo
	}
}

// Attach service name to logs
func WithService(service string) LoggerOption {
	return func(l *Logger) {
		l.service = service
	}
}

// Set minimum logging level
func WithMinLevel(level Level) LoggerOption {
	return func(l *Logger) {
		l.minLevel = level
	}
}

// Creates a new Logger instance, with optional LoggerOption functions to set Logger properties. Usage: `logger := logging.New(os.Stdout, logging.WithDatabase(logRepo), logging.WithService("api"), logging.WithMinLevel(logging.LevelInfo))`
func New(out io.Writer, opts ...LoggerOption) *Logger {
	logger := &Logger{
		out:      out,
		minLevel: LevelInfo,
		logRepo:  nil,
	}

	for _, opt := range opts {
		opt(logger)
	}

	return logger
}

func (l *Logger) Debug(message string, details map[string]any) {
	l.print(LevelDebug, message, details)
}

func (l *Logger) Info(message string, details map[string]any) {
	l.print(LevelInfo, message, details)
}

func (l *Logger) Warn(message string, details map[string]any) {
	l.print(LevelWarn, message, details)
}

func (l *Logger) Error(err error, details map[string]any) {
	l.print(LevelError, err.Error(), details)
}

func (l *Logger) Fatal(err error, details map[string]any) {
	l.print(LevelFatal, err.Error(), details)
	// As it is a fatal error, terminate the application
	os.Exit(1)
}

// As print is an internal function only (Info/Debug/Error/Fatal etc. are the only ones that will be called from outside this package), it is not capitalised.
func (l *Logger) print(level Level, message string, details map[string]any) (int, error) {
	// Lock mutex, and defer unlock until function returns with the result of the log write operation.
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.minLevel {
		return 0, nil
	}

	// for i := range 5 {
	// 	if _, file, line, ok := runtime.Caller(i); ok {
	// 		fmt.Printf("Call Depth: %d\nFile: %v\nLine: %v\n\n", i, file, line)
	// 	}
	// }

	aux := Log{
		Level:     level.String(),
		Message:   message,
		Timestamp: time.Now().UTC(),
		Service:   l.service,
		Details:   details,
	}

	// Get information about where the log was created - stack depth of 4 works well with app.logAppError, but may produce useless information in other cases
	if _, file, line, ok := runtime.Caller(4); ok {
		aux.File = file
		aux.Line = line
	}

	// If log is at least error level, include a stacktrace in the log
	if level >= LevelError {
		aux.Trace = string(debug.Stack())
	}

	// Write log to database if logModel exists
	if l.logRepo != nil {
		if err := l.logRepo.Insert(&aux); err != nil {
			mes := []byte(LevelError.String() + ": failed to write log to database: " + err.Error())
			l.out.Write(mes)
		}
	}

	// Create the line constituting the log and populate it with all info in aux marshalled to JSON. If that fails, create a log line recording that error instead.
	var output []byte

	output, err := json.Marshal(aux)
	if err != nil {
		output = []byte(LevelError.String() + ": unable to marshal log message: " + err.Error())
	}

	return l.out.Write(append(output, '\n'))
}

// TODO: What is the reason for LevelError specifically here? Check Let's Go
func (l *Logger) Write(message []byte) (n int, err error) {
	return l.print(LevelError, string(message), nil)
}
