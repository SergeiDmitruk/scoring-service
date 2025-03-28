import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
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

func decompressGzip(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	gzipReader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	_, err = io.Copy(&buf, gzipReader)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
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
		var responseBody string
		if lw.Header().Get("Content-Encoding") == "gzip" {
			decompressed, err := decompressGzip(lw.respData.responseBody)
			if err == nil {
				responseBody = string(decompressed)
			} else {
				logger.Log.Error("Failed to decompress gzip data: " + err.Error())
			}
		} else {
			responseBody = string(lw.respData.responseBody)
		}
		logData := map[string]interface{}{
			"uri":           r.RequestURI,
			"method":        r.Method,
			"headers":       r.Header,
			"request_body":  string(bodyBytes),
			"response_body": responseBody,
			"response_size": lw.respData.size,
			"status":        lw.respData.statusCode,
			"duration":      time.Since(start),
		}

		logger.Log.Sugar().Infoln(logData)
	})
}
