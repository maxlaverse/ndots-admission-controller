package log

// Logger is the interface used to plug logging backends
type Logger interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Warning(args ...interface{})
	Warningf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
}

var l Logger = noopLogger{}

// SetLogger sets a logger
func SetLogger(lg Logger) { l = lg }

// Error logs in error level
func Error(args ...interface{}) { l.Error(args...) }

// Errorf logs in error level with formatting
func Errorf(format string, args ...interface{}) { l.Errorf(format, args...) }

// Warning logs in error level
func Warning(args ...interface{}) { l.Warning(args...) }

// Warningf logs in error level with formatting
func Warningf(format string, args ...interface{}) { l.Warningf(format, args...) }

// Info logs in error level
func Info(args ...interface{}) { l.Info(args...) }

// Infof logs in error level with formatting
func Infof(format string, args ...interface{}) { l.Infof(format, args...) }

// Debug logs in error level
func Debug(args ...interface{}) { l.Debug(args...) }

// Debugf logs in error level with formatting
func Debugf(format string, args ...interface{}) { l.Debugf(format, args...) }

type noopLogger struct{}

func (n noopLogger) Error(args ...interface{})                   {}
func (n noopLogger) Errorf(format string, args ...interface{})   {}
func (n noopLogger) Warning(args ...interface{})                 {}
func (n noopLogger) Warningf(format string, args ...interface{}) {}
func (n noopLogger) Info(args ...interface{})                    {}
func (n noopLogger) Infof(format string, args ...interface{})    {}
func (n noopLogger) Debug(args ...interface{})                   {}
func (n noopLogger) Debugf(format string, args ...interface{})   {}
