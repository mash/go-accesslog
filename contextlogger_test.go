package accesslog_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/mash/go-accesslog"
)

type contextLogger struct {
	buf string
}

func (l *contextLogger) Log(record accesslog.LogRecord) {
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

func (l *contextLogger) LogContext(ctx context.Context, r accesslog.LogRecord) {
	if r.CustomRecords == nil {
		r.CustomRecords = map[string]string{}
	}
	r.CustomRecords["x-user-id"] = ctx.Value("x-user-id").(string)
	l.Log(r)
}

func TestContextLogger(t *testing.T) {
	logger := &contextLogger{}
	loggingHandler := accesslog.NewLoggingHandler(http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), "x-user-id", "1")
			*req = *req.WithContext(ctx)
			w.Write([]byte(`ok`))
		}), logger)
	writer := httptest.NewRecorder()
	loggingHandler.ServeHTTP(writer, httptest.NewRequest("GET", "/", nil))

	expected := "method:GET,uri:/,protocol:HTTP/1.1,username:-,host:example.com,status:200,customRecords:map[x-user-id:1]\n"
	output := logger.buf
	if output != expected {
		t.Errorf("expected %s but got %s", expected, output)
	}
}
