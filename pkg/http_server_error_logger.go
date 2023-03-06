package pkg

import (
	"log"
	"strings"

	"k8s.io/klog/v2"
)

type httpServerErrorLogger struct{}

func (*httpServerErrorLogger) Write(p []byte) (int, error) {
	m := string(p)
	if !strings.HasPrefix(m, "http: TLS handshake error") && !strings.HasSuffix(m, ": EOF\n") {
		klog.Error(m)
	}

	return len(p), nil
}

func newHttpServerErrorLogger() *log.Logger {
	return log.New(&httpServerErrorLogger{}, "", 0)
}
