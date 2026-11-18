package middleware

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

// tenantResponseWriter buffers an API response until the database transaction
// has committed. This prevents a successful HTTP response from escaping when
// the final database commit fails.
type tenantResponseWriter struct {
	gin.ResponseWriter
	header http.Header
	status int
	body   bytes.Buffer
}

func newTenantResponseWriter(dst gin.ResponseWriter) *tenantResponseWriter {
	return &tenantResponseWriter{ResponseWriter: dst, header: make(http.Header)}
}

func (w *tenantResponseWriter) Header() http.Header { return w.header }

func (w *tenantResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *tenantResponseWriter) WriteHeaderNow() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
}

func (w *tenantResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}

func (w *tenantResponseWriter) WriteString(s string) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.WriteString(s)
}

func (w *tenantResponseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *tenantResponseWriter) Size() int    { return w.body.Len() }
func (w *tenantResponseWriter) Written() bool { return w.status != 0 || w.body.Len() > 0 }
func (w *tenantResponseWriter) Flush()        {}

func (w *tenantResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("hijacking is not supported for tenant transactions")
}

func (w *tenantResponseWriter) CloseNotify() <-chan bool { return w.ResponseWriter.CloseNotify() }
func (w *tenantResponseWriter) Pusher() http.Pusher      { return w.ResponseWriter.Pusher() }
func (w *tenantResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *tenantResponseWriter) flushTo(dst gin.ResponseWriter) {
	for key, values := range w.header {
		for _, value := range values {
			dst.Header().Add(key, value)
		}
	}
	dst.WriteHeader(w.Status())
	if w.body.Len() > 0 {
		_, _ = dst.Write(w.body.Bytes())
	}
}
