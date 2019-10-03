package accesslog

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

type customLogger struct {
	buf string
}

func (l *customLogger) Log(record LogRecord) {
	fields := make([]string, 0)
	fields = append(fields, "method:"+record.Method)
	fields = append(fields, "uri:"+record.Uri)
	fields = append(fields, "protocol:"+record.Protocol)
	fields = append(fields, "username:"+record.Username)
	fields = append(fields, "host:"+record.Host)
	fields = append(fields, "status:"+fmt.Sprintf("%d", record.Status))

	// Sort the custom records to get a deterministic test result
	keys := []string{}
	for k, _ := range record.CustomRecords {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	customRecords := []string{}
	for _, key := range keys {
		customRecords = append(customRecords, fmt.Sprintf("%s:%s", key, record.CustomRecords[key]))
	}
	fields = append(fields, "customRecords:"+fmt.Sprintf("map%v", customRecords))

	l.buf += strings.Join(fields, ",")
	l.buf += "\n"
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
	req.Host = "example.com"
	return req
}

func TestOutput(t *testing.T) {
	logger := customLogger{}
	loggingHandler := NewLoggingHandler(http.HandlerFunc(okHandler), &logger)
	writer := httptest.NewRecorder()
	loggingHandler.ServeHTTP(writer, newRequest("GET", "/"))

	expected := "method:GET,uri:,protocol:HTTP/1.1,username:-,host:example.com,status:200,customRecords:map[x-user-id:1]\n"
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

	expected := "method:GET,uri:,protocol:HTTP/1.1,username:-,host:example.com,status:0,customRecords:map[at:before]\n" +
		"method:GET,uri:,protocol:HTTP/1.1,username:-,host:example.com,status:200,customRecords:map[at:after x-user-id:1]\n"

	output := logger.buf
	if output != expected {
		t.Errorf("expected\n%s\nbut got\n%s", expected, output)
	}
}
