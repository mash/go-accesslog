package accesslog

import (
	"net/http"
	"strings"
	"time"
)

type LogRecord struct {
	Time                                time.Time
	Ip, Method, Uri, Protocol, Username string
	Status                              int
	Size                                int64
	ElapsedTime                         time.Duration
	CustomRecords                       map[string]string
}

type LoggingWriter struct {
	http.ResponseWriter
	logRecord LogRecord
}

func (r *LoggingWriter) Write(p []byte) (int, error) {
	if r.logRecord.Status == 0 {
		// The status will be StatusOK if WriteHeader has not been called yet
		r.logRecord.Status = http.StatusOK
	}
	written, err := r.ResponseWriter.Write(p)
	r.logRecord.Size += int64(written)
	return written, err
}

func (r *LoggingWriter) WriteHeader(status int) {
	r.logRecord.Status = status
	r.ResponseWriter.WriteHeader(status)
}

// w.(accesslogger.LoggingWriter).SetCustomLogRecord("X-User-Id", "3")
func (r *LoggingWriter) SetCustomLogRecord(key, value string) {
	if r.logRecord.CustomRecords == nil {
		r.logRecord.CustomRecords = map[string]string{}
	}
	r.logRecord.CustomRecords[key] = value
}

func (r *LoggingWriter) CloseNotify() <-chan bool {
	return r.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

type Logger interface {
	Log(record LogRecord)
}

type LoggingHandler struct {
	handler http.Handler
	logger  Logger
}

func NewLoggingHandler(handler http.Handler, logger Logger) http.Handler {
	return &LoggingHandler{
		handler: handler,
		logger:  logger,
	}
}

func NewLoggingMiddleware(logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		handler := NewLoggingHandler(next, logger)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
		})
	}
}

func (h *LoggingHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ip := strings.Split(r.RemoteAddr, ":")[0]

	username := "-"
	if r.URL.User != nil {
		if name := r.URL.User.Username(); name != "" {
			username = name
		}
	}

	writer := &LoggingWriter{
		ResponseWriter: rw,
		logRecord: LogRecord{
			Time:        time.Time{},
			Ip:          ip,
			Method:      r.Method,
			Uri:         r.RequestURI,
			Username:    username,
			Protocol:    r.Proto,
			Status:      0,
			Size:        0,
			ElapsedTime: time.Duration(0),
		},
	}

	startTime := time.Now()
	h.handler.ServeHTTP(writer, r)
	finishTime := time.Now()

	writer.logRecord.Time = finishTime.UTC()
	writer.logRecord.ElapsedTime = finishTime.Sub(startTime)

	h.logger.Log(writer.logRecord)
}
