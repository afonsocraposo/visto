package main

import (
	"log"
	"net/http"
	"strings"
	"time"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (writer *loggingResponseWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *loggingResponseWriter) Write(data []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	n, err := writer.ResponseWriter.Write(data)
	writer.bytes += n
	return n, err
}

func (writer *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		writer := &loggingResponseWriter{ResponseWriter: w}
		defer func() {
			status := writer.status
			if status == 0 {
				status = http.StatusOK
			}
			log.Printf("http request method=%s path=%s status=%d bytes=%d duration=%s", r.Method, logPath(r.URL.EscapedPath()), status, writer.bytes, time.Since(started).Round(time.Millisecond))
		}()
		next.ServeHTTP(writer, r)
	})
}

func logPath(path string) string {
	const plexWebhookPrefix = "/api/v1/webhooks/plex/"
	if index := strings.Index(path, plexWebhookPrefix); index >= 0 {
		return path[:index+len(plexWebhookPrefix)] + "[redacted]"
	}
	return path
}
