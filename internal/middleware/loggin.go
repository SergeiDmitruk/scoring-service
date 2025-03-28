package middleware

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/scoring-service/pkg/logger"
)

type respData struct {
	statusCode int
	size       int
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
	size, err := r.ResponseWriter.Write(b)
	r.respData.size += size
	return size, err
}

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()

		logger.Log.Sugar().Infoln(
			"uri", r.RequestURI,
			"method", r.Method,
			"headers", r.Header,
		)

		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		if len(bodyBytes) > 0 {
			logger.Log.Sugar().Infoln("request body", string(bodyBytes))
		}

		lw := loggerResponseWriter{
			ResponseWriter: w,
			respData:       &respData{},
		}

		next.ServeHTTP(&lw, r)
		logger.Log.Sugar().Infoln(
			"status", lw.respData.statusCode,
			"duration", time.Since(start),
		)
	})
}
