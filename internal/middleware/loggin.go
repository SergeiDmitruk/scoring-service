package middleware

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/scoring-service/pkg/logger"
)

type respData struct {
	statusCode   int
	size         int
	responseBody string
}

type loggerResponseWriter struct {
	http.ResponseWriter
	respData *respData
}

func (r *loggerResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.respData.statusCode = statusCode
}

func (r *loggerResponseWriter) Write(b []byte) (int, error) {
	r.respData.responseBody = string(b)
	size, err := r.ResponseWriter.Write(b)
	r.respData.size += size
	return size, err
}

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		lw := loggerResponseWriter{
			ResponseWriter: w,
			respData:       &respData{},
		}

		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		next.ServeHTTP(&lw, r)

		logData := map[string]interface{}{
			"uri":           r.RequestURI,
			"method":        r.Method,
			"headers":       r.Header,
			"request_body":  string(bodyBytes),
			"status":        lw.respData.statusCode,
			"response_body": lw.respData.responseBody,
			"response_size": lw.respData.size,
			"duration":      time.Since(start),
		}

		logger.Log.Sugar().Infoln(logData)
	})
}
