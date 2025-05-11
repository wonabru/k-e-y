package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	logFile *os.File
	mw      *MultiWriter
	logger  *log.Logger
	once    sync.Once
)

type MultiWriter struct {
	writers []io.Writer
}

func (t *MultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(p), nil
}
func InitLogger() {
	once.Do(func() {
		homePath, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}

		logsDir := filepath.Join(homePath, ".okura", "logs")
		if err := os.MkdirAll(logsDir, 0755); err != nil {
			log.Fatal(err)
		}
		logFile, err = os.OpenFile(filepath.Join(logsDir, "mining.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatal(err)
		}
		mw = &MultiWriter{
			writers: []io.Writer{
				os.Stdout,
				logFile,
			},
		}
		logger = log.New(mw, "", log.LstdFlags)
		log.SetOutput(mw)
		log.SetFlags(log.LstdFlags)
	})
}
func GetLogger() *log.Logger {
	InitLogger()
	return logger
}

func CloseLogger() {
	if logFile != nil {
		logFile.Close()
	}
}
