package gun

import (
	"io"
	"net"
	"strings"
	"sync"

	"github.com/metacubex/mihomo/common/buf"
	"github.com/metacubex/mihomo/common/httputils"
	N "github.com/metacubex/mihomo/common/net"

	"github.com/metacubex/http"
)

type ServerOption struct {
	ServiceName string
	ConnHandler func(conn net.Conn)
	HttpHandler http.Handler
}

func NewServerHandler(options ServerOption) http.Handler {
	path := ServiceNameToPath(options.ServiceName)
	connHandler := options.ConnHandler
	httpHandler := options.HttpHandler
	if httpHandler == nil {
		httpHandler = http.NewServeMux()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == path &&
			request.Method == http.MethodPost &&
			strings.HasPrefix(request.Header.Get("Content-Type"), "application/grpc") {

			writer.Header().Set("Content-Type", "application/grpc")
			writer.Header().Set("TE", "trailers")
			writer.WriteHeader(http.StatusOK)

			conn := &Conn{
				initFn: func(addr *httputils.NetAddr) (io.ReadCloser, error) {
					httputils.SetAddrFromRequest(addr, request)
					return request.Body, nil
				},
				writer: writer,
			}
			_ = conn.Init()

			wrapper := &h2ConnWrapper{
				// gun.Conn can't correct handle ReadDeadline
				// so call N.NewDeadlineConn to add a safe wrapper
				ExtendedConn: N.NewDeadlineConn(conn),
			}
			connHandler(wrapper)
			wrapper.CloseWrapper()

			return
		}

		httpHandler.ServeHTTP(writer, request)
	})
}

// h2ConnWrapper used to avoid "panic: Write called after Handler finished" for gun.Conn
type h2ConnWrapper struct {
	N.ExtendedConn
	access sync.Mutex
	closed bool
}

func (w *h2ConnWrapper) Write(p []byte) (n int, err error) {
	w.access.Lock()
	defer w.access.Unlock()
	if w.closed {
		return 0, net.ErrClosed
	}
	return w.ExtendedConn.Write(p)
}

func (w *h2ConnWrapper) WriteBuffer(buffer *buf.Buffer) error {
	w.access.Lock()
	defer w.access.Unlock()
	if w.closed {
		return net.ErrClosed
	}
	return w.ExtendedConn.WriteBuffer(buffer)
}

func (w *h2ConnWrapper) CloseWrapper() {
	w.access.Lock()
	defer w.access.Unlock()
	w.closed = true
}

func (w *h2ConnWrapper) Upstream() any {
	return w.ExtendedConn
}
