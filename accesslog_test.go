package accesslog

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type customLogger struct {
	buf string
}

func (l *customLogger) Log(record LogRecord) {
	// l.buf += "time:" + record.Time.Format("02/Jan/2006:15:04:05 -0700")
	// l.buf += "ip:" + record.Ip
	l.buf += "method:" + record.Method
	l.buf += "uri:" + record.Uri
	l.buf += "protocol:" + record.Protocol
	l.buf += "username:" + record.Username
	l.buf += "status:" + fmt.Sprintf("%d", record.Status)
	// l.buf += "elapsedTime:" + fmt.Sprintf("%d", record.ElapsedTime)
	l.buf += "customRecords:" + fmt.Sprintf("%v", record.CustomRecords)
}

func okHandler(w http.ResponseWriter, req *http.Request) {
	w.(*LoggingWriter).SetCustomLogRecord("x-user-id", "1")
	w.Write([]byte(`ok`))
}

func newRequest(method, url string) *http.Request {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		panic(err)
	}
	return req
}

func TestOutput(t *testing.T) {
	logger := customLogger{}
	loggingHandler := NewLoggingHandler(http.HandlerFunc(okHandler), &logger)
	writer := httptest.NewRecorder()
	loggingHandler.ServeHTTP(writer, newRequest("GET", "/"))

	expected := "method:GETuri:protocol:HTTP/1.1username:-status:200customRecords:map[x-user-id:1]"
	output := logger.buf
	if output != expected {
		t.Errorf("expected %s but got %s", expected, output)
	}
}

func TestAroundOutput(t *testing.T) {
	logger := customLogger{}
	loggingHandler := NewAroundLoggingHandler(http.HandlerFunc(okHandler), &logger)
	writer := httptest.NewRecorder()
	loggingHandler.ServeHTTP(writer, newRequest("GET", "/"))

	expected := "method:GETuri:protocol:HTTP/1.1username:-status:0customRecords:map[at:before]" +
		"method:GETuri:protocol:HTTP/1.1username:-status:200customRecords:map[at:after x-user-id:1]"

	output := logger.buf
	if output != expected {
		t.Errorf("expected %s but got %s", expected, output)
	}
}
