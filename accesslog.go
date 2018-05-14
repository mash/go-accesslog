package accesslog

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type LogRecord struct {
	Time                                                 time.Time
	Ip, Method, Uri, Protocol, Username, Host, UserAgent string
	Status                                               int
	ResponseSize, RequestSize                            int64
	ElapsedTime                                          time.Duration
	RequestHeader                                        http.Header
	CustomRecords                                        map[string]string
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
	r.logRecord.ResponseSize += int64(written)
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

// http.CloseNotifier interface
func (r *LoggingWriter) CloseNotify() <-chan bool {
	if w, ok := r.ResponseWriter.(http.CloseNotifier); ok {
		return w.CloseNotify()
	}
	return make(chan bool)
}

func (r *LoggingWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := r.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("ResponseWriter doesn't support Hijacker interface")
}

// http.Flusher
func (r *LoggingWriter) Flush() {
	flusher, ok := r.ResponseWriter.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

// http.Pusher
func (r *LoggingWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := r.ResponseWriter.(http.Pusher)
	if ok {
		return pusher.Push(target, opts)
	}
	return fmt.Errorf("ResponseWriter doesn't support Pusher interface")
}

type Logger interface {
	Log(record LogRecord)
}

type LoggingHandler struct {
	handler   http.Handler
	logger    Logger
	logBefore bool
}

func NewLoggingHandler(handler http.Handler, logger Logger) http.Handler {
	return &LoggingHandler{
		handler:   handler,
		logger:    logger,
		logBefore: false,
	}
}

func NewAroundLoggingHandler(handler http.Handler, logger Logger) http.Handler {
	return &LoggingHandler{
		handler:   handler,
		logger:    logger,
		logBefore: true,
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

func NewAroundLoggingMiddleware(logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		handler := NewAroundLoggingHandler(next, logger)
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

	startTime := time.Now()
	writer := &LoggingWriter{
		ResponseWriter: rw,
		logRecord: LogRecord{
			Time:          startTime.UTC(),
			Ip:            ip,
			Method:        r.Method,
			Uri:           r.RequestURI,
			UserAgent:     r.UserAgent(),
			Username:      username,
			Protocol:      r.Proto,
			Host:          r.Host,
			Status:        0,
			ResponseSize:  0,
			RequestSize:   r.ContentLength,
			ElapsedTime:   time.Duration(0),
			RequestHeader: r.Header,
		},
	}

	if h.logBefore {
		writer.SetCustomLogRecord("at", "before")
		h.logger.Log(writer.logRecord)
	}
	h.handler.ServeHTTP(writer, r)
	finishTime := time.Now()

	writer.logRecord.Time = finishTime.UTC()
	writer.logRecord.ElapsedTime = finishTime.Sub(startTime)

	if h.logBefore {
		writer.SetCustomLogRecord("at", "after")
	}
	h.logger.Log(writer.logRecord)
}
