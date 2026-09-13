package middleware

import (
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

type loggingResponseWriter struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach optional interfaces implemented by
// the original writer without hiding them behind the logging wrapper.
func (w *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// RequestLogger logs one completion event for every backend request. The
// healthcheck endpoint is intentionally omitted to avoid routine log noise.
func RequestLogger(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		started := time.Now()
		requestID := requestID(r)
		w.Header().Set(requestIDHeader, requestID)
		response := &loggingResponseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		next.ServeHTTP(response, r)

		logger.Printf(
			"http_request method=%q path=%q route=%q status=%d bytes=%d duration_ms=%.3f client_ip=%q request_id=%q user_agent=%q",
			r.Method,
			r.URL.EscapedPath(),
			r.Pattern,
			response.status,
			response.bytes,
			float64(time.Since(started).Microseconds())/1000,
			clientIP(r),
			requestID,
			r.UserAgent(),
		)
	})
}

func requestID(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get(requestIDHeader)); value != "" && len(value) <= 128 {
		return value
	}
	return uuid.NewString()
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); net.ParseIP(forwarded) != nil {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
