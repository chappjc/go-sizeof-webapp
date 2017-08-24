// Package filelog implements advanced writer to log files for "log4go" package
// with improved algorithm of log rotation.
package filelog

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/alecthomas/log4go"
)

// Day format for comparing files changed time during daily log rotation.
const dayFormat = "2006-01-02"

// Writer represents log writer which writes logs into files. It can rotate
// files and delete previously rotated but expired now logs.
type Writer struct {
	// Channels to receive commands
	rec chan *log.LogRecord
	rot chan bool

	// The opened file
	filename string
	file     *os.File
	writer   io.Writer

	// The logging format
	format string
	// File header/trailer
	header, trailer string

	// How long keep already rotated files (0 value means always)
	keepRotatedSeconds time.Duration
	// Rotate at linecount
	maxlines         uint64
	maxlinesCurlines uint64
	// Rotate at size
	maxsize        uint64
	maxsizeCursize uint64
	// Rotate daily
	daily         bool
	dailyOpenDate string
	// Keep old log files (.001, .002, etc)
	rotate bool

	// Makes closing synchronized if true
	waitOnClose bool
	waiter      *sync.WaitGroup
}

// NewWriter initializes new log writer.
func NewWriter(fName string, rotate bool) *Writer {
	w := &Writer{
		rec:      make(chan *log.LogRecord, log.LogBufferLength),
		rot:      make(chan bool),
		filename: fName,
		format:   "[%D %T] [%L] (%S) %M",
		rotate:   rotate,
		waiter:   &sync.WaitGroup{},
	}

	w.waiter.Add(1)
	go func() {
		defer w.waiter.Done()
		defer w.closeCurrentFile()
		printErr := func(e error) {
			fmt.Fprintf(os.Stderr,
				"imaginator/filelog.NewWriter(%q): %s\n", w.filename, e,
			)
		}
		for {
			select {
			case <-w.rot:
				if err := w.doRotation(); err != nil {
					printErr(err)
					return
				}
			case rec, ok := <-w.rec:
				if !ok {
					return
				}
				if w.file == nil {
					if err := w.openNewFile(); err != nil {
						printErr(err)
						return
					}
				}
				if (w.maxlines > 0 && w.maxlinesCurlines >= w.maxlines) ||
					(w.maxsize > 0 && w.maxsizeCursize >= w.maxsize) ||
					(w.daily &&
						(time.Now().Format(dayFormat) != w.dailyOpenDate)) {
					if err := w.doRotation(); err != nil {
						printErr(err)
						return
					}
				}
				if err := w.write(rec); err != nil {
					printErr(err)
					return
				}
			}
		}
	}()

	return w
}

// Helper function to rotate logs files.
func (w *Writer) doRotation() (e error) {
	w.closeCurrentFile()
	if w.rotate {
		err := os.Rename(w.filename, w.processAlreadyRotatedFiles())
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rotation failed: %s", err)
		}
	}
	if w.file != nil {
		e = w.openNewFile()
	}
	return
}

// Helper function to process already rotated files. It removes expired log
// files if any and returns name of next file to rotate into.
func (w *Writer) processAlreadyRotatedFiles() (fileNameForRotation string) {
	dir := filepath.Dir(w.filename)
	lastNum := 0
	if files, err := ioutil.ReadDir(dir); err == nil {
		base := filepath.Base(w.filename)
		now := time.Now()
		for _, file := range files {
			fileName := file.Name()
			if file.IsDir() || !strings.HasPrefix(fileName, base) {
				continue
			}
			suffix := strings.TrimPrefix(fileName, base)
			if suffix == "" {
				continue
			}
			num, _ := strconv.Atoi(strings.TrimPrefix(suffix, "."))
			if num > lastNum {
				lastNum = num
			}
			if w.keepRotatedSeconds > 0 &&
				(now.Sub(file.ModTime()) > w.keepRotatedSeconds) {
				if err := os.Remove(filepath.Join(dir, fileName)); err != nil {
					fmt.Fprintf(os.Stderr,
						"filelog.processAlreadyRotatedFiles(%q): %s\n",
						fileName, err,
					)
				}
			}
		}
	}
	if lastNum < 1 {
		lastNum = 0
	}
	return w.filename + fmt.Sprintf(".%03d", lastNum+1)
}

// Helper function for opening new file to write logs into.
func (w *Writer) openNewFile() (e error) {
	fd, e := os.OpenFile(w.filename, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)
	if e != nil {
		return
	}
	w.file = fd
	w.writer = io.MultiWriter(fd, os.Stdout)
	fi, e := fd.Stat()
	if e != nil {
		return
	}
	w.dailyOpenDate = fi.ModTime().Format(dayFormat)
	w.maxsizeCursize = uint64(fi.Size())
	if w.maxlinesCurlines, e = func() (num uint64, _ error) {
		scanner := bufio.NewScanner(w.file)
		for scanner.Scan() {
			num++
		}
		return num, scanner.Err()
	}(); e != nil {
		return
	}
	fmt.Fprint(w.writer,
		log.FormatLogRecord(w.header, &log.LogRecord{Created: time.Now()}),
	)
	return
}

// Helper function for closing current opened file if any.
func (w *Writer) closeCurrentFile() {
	if w.file == nil {
		return
	}
	fmt.Fprint(w.writer,
		log.FormatLogRecord(w.trailer, &log.LogRecord{Created: time.Now()}),
	)
	if err := w.file.Close(); err != nil {
		log.Stderrf("Failed to close file: %v", err)
	}
}

// Helper function to write given log record into current opened file.
//
// Attention: File must be opened to avoid nil pointer failure!
func (w *Writer) write(rec *log.LogRecord) (e error) {
	n, e := fmt.Fprint(w.writer, log.FormatLogRecord(w.format, rec))
	if e != nil {
		return
	}
	w.maxlinesCurlines++
	w.maxsizeCursize += uint64(n)
	return
}

// LogWrite writes given log record into file. Implementation of
// log4go.LogWriter interface.
func (w *Writer) LogWrite(rec *log.LogRecord) {
	w.rec <- rec
}

// Close closes current log writer and resources connected with it. By default
// acts asynchronous, which means that method doesn't wait log writer to be
// closed. To change this behaviour you must use .SetWaitOnClose() method.
// Implementation of log4go.LogWriter interface.
func (w *Writer) Close() {
	close(w.rec)
	if w.waitOnClose {
		w.waiter.Wait()
	}
}

// Rotate requests current log rotation.
func (w *Writer) Rotate() {
	w.rot <- true
}

// SetFormat sets the logging format (chainable). Must be called before the
// first log message is written.
func (w *Writer) SetFormat(format string) *Writer {
	w.format = format
	return w
}

// SetHeadFoot sets the log file header and footer (chainable). Must be called
// before the first log message is written. These are formatted similar to the
// log4go.FormatLogRecord (e.g. you can use %D and %T in your header/footer for
// date and time).
func (w *Writer) SetHeadFoot(head, foot string) *Writer {
	w.header, w.trailer = head, foot
	return w
}

// SetRotateLines sets rotate at linecount (chainable). Must be called before
// the first log message is written.
func (w *Writer) SetRotateLines(maxlines int) *Writer {
	w.maxlines = uint64(maxlines)
	return w
}

// SetRotateSize sets rotate at size (chainable). Must be called before the
// first log message is written.
func (w *Writer) SetRotateSize(maxsize int) *Writer {
	w.maxsize = uint64(maxsize)
	return w
}

// SetRotateDaily sets rotate daily (chainable). Must be called before the first
// log message is written.
func (w *Writer) SetRotateDaily(daily bool) *Writer {
	w.daily = daily
	return w
}

// SetRotate changes whether or not the old logs are kept (chainable). Must be
// called before the first log message is written. If rotate is false, the files
// are overwritten; otherwise, they are rotated to another file before the new
// log is opened.
func (w *Writer) SetRotate(rotate bool) *Writer {
	w.rotate = rotate
	return w
}

// SetRotatedFilesExpiration sets duration (in seconds) of how long already
// rotated files must be kept (chainable). If is not set, then files will be
// kept always.
func (w *Writer) SetRotatedFilesExpiration(seconds uint64) *Writer {
	w.keepRotatedSeconds = time.Duration(seconds) * time.Second
	return w
}

// SetWaitOnClose makes .Close() method to wait until Writer will be totally
// closed. If is not set, by default is false, which means .Close() method to
// act asynchronous.
func (w *Writer) SetWaitOnClose(yes bool) *Writer {
	w.waitOnClose = yes
	return w
}
