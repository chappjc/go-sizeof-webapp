package log

import (
	"fmt"
	"os"

	"github.com/chappjc/go-sizeof-webapp/internal/log/filelog"

	l4g "github.com/alecthomas/log4go"
)

// ApplicationLogFile is the relative path (from application root) to file where
// application log is stored.
const ApplicationLogFile = "logs/application.log"

// Description of filelog.Writer creation error.
const errCreateLogFile = "failed to create '%s' log file"

// Logger represents a logger with different levels of logs.
type Logger interface {
	Debug(interface{}, ...interface{})
	Trace(interface{}, ...interface{})
	Info(interface{}, ...interface{})
	Warn(interface{}, ...interface{}) error
	Error(interface{}, ...interface{}) error
	Critical(interface{}, ...interface{}) error
	Close()
}

// NewApplicationLogger creates and returns new application logger, ready for
// use.
func NewApplicationLogger() (Logger, error) {
	lgr := make(l4g.Logger)
	if flw := filelog.NewWriter(ApplicationLogFile, false); flw == nil {
		return nil, fmt.Errorf(errCreateLogFile, ApplicationLogFile)
	} else {
		flw.SetFormat("[%D %T][%L] %M")
		flw.SetWaitOnClose(true)
		lgr.AddFilter("s", l4g.INFO, flw)
	}
	return lgr, nil
}

// StdErr performs printf() of given pattern with given arguments to OS standard
// error output stream (stderr).
func StdErr(pattern string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, pattern, args...)
}
