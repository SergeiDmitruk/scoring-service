package middleware

import (
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
		lw := loggerResponseWriter{
			ResponseWriter: w,
			respData:       &respData{},
		}
		next.ServeHTTP(&lw, r)

		logger.Log.Sugar().Infoln(
			"uri", r.RequestURI,
			"method", r.Method,
			"status", lw.respData.statusCode,
			"size", lw.respData.size,
			"duration", time.Since(start),
		)
	})
}
