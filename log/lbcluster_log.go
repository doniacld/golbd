package log

import (
	"fmt"
	"log/syslog"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	LevelInfo    = "INFO"
	LevelDebug   = "DEBUG"
	LevelWarning = "WARNING"
	LevelError   = "ERROR"
)

// Log struct for the log
type Log struct {
	SyslogWriter *syslog.Writer
	Stdout       bool
	DebugFlag    bool
	ToFilePath   string
	logMu        sync.Mutex
}

// Info writes as Info
func (l *Log) Info(s string) error {
	var err error
	if l.SyslogWriter != nil {
		err = l.SyslogWriter.Info(s)
	}
	if l.Stdout || (l.ToFilePath != "") {
		err = l.writeFileStd("INFO: " + s)
	}

	return err
}

// Warning writes as Warning
func (l *Log) Warning(s string) error {
	var err error
	if l.SyslogWriter != nil {
		err = l.SyslogWriter.Warning(s)
	}
	if l.Stdout || (l.ToFilePath != "") {
		err = l.writeFileStd("WARNING: " + s)
	}

	return err
}

// Debug writes as Debug
func (l *Log) Debug(s string) error {
	var err error
	if l.DebugFlag {
		if l.SyslogWriter != nil {
			err = l.SyslogWriter.Debug(s)
		}
		if l.Stdout || (l.ToFilePath != "") {
			err = l.writeFileStd("DEBUG: " + s)
		}
	}

	return err
}

// Error writes as Error
func (l *Log) Error(s string) error {
	var err error
	if l.SyslogWriter != nil {
		err = l.SyslogWriter.Err(s)
	}
	if l.Stdout || (l.ToFilePath != "") {
		err = l.writeFileStd("ERROR: " + s)
	}
	return err

}

// writeFileStd actually writes on a file a string and locks the file when writing.
func (l *Log) writeFileStd(s string) error {
	tag := "lbd"
	nl := ""
	if !strings.HasSuffix(s, "\n") {
		nl = "\n"
	}

	timestamp := time.Now().Format(time.StampMilli)
	msg := fmt.Sprintf("%s %s[%d]: %s%s", timestamp, tag, os.Getpid(), s, nl)

	l.logMu.Lock()
	defer l.logMu.Unlock()

	// write on the standard output
	if l.Stdout {
		_, err := fmt.Printf(msg)
		if err != nil {
			return err
		}
	}

	// write on the file
	if l.ToFilePath != "" {
		f, err := os.OpenFile(l.ToFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0640)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = fmt.Fprintf(f, msg)
		if err != nil {
			return err
		}
	}

	return nil
}
