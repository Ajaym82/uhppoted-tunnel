package httpd

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/uhppoted/uhppoted-tunnel/protocol"
	"github.com/uhppoted/uhppoted-tunnel/router"
	"github.com/uhppoted/uhppoted-tunnel/tunnel/conn"
)

type httpd struct {
	conn.Conn
	addr    *net.TCPAddr
	retry   conn.Backoff
	timeout time.Duration
	ch      chan protocol.Message
	closing chan struct{}
	closed  chan struct{}
	fs      filesystem
}

type slice []byte

func (s slice) MarshalJSON() ([]byte, error) {
	a := make([]uint16, len(s))
	for i, v := range s {
		a[i] = uint16(v)
	}

	return json.Marshal(a)
}

const GZIP_MINIMUM = 16384

func NewHTTPD(spec string, html string, retry conn.Backoff) (*httpd, error) {
	addr, err := net.ResolveTCPAddr("tcp", spec)
	if err != nil {
		return nil, err
	}

	fs := filesystem{
		FileSystem: http.FS(os.DirFS(html)),
	}

	if addr == nil {
		return nil, fmt.Errorf("unable to resolve HTTP base address '%v'", spec)
	}

	h := httpd{
		Conn: conn.Conn{
			Tag: "HTTP",
		},
		addr:    addr,
		retry:   retry,
		timeout: 5 * time.Second,
		ch:      make(chan protocol.Message, 16),
		closing: make(chan struct{}),
		closed:  make(chan struct{}),
		fs:      fs,
	}

	return &h, nil
}

func (h *httpd) Close() {
	h.Infof("closing")

	close(h.closing)

	timeout := time.NewTimer(5 * time.Second)
	select {
	case <-h.closed:
		h.Infof("closed")

	case <-timeout.C:
		h.Infof("close timeout")
	}
}

func (h *httpd) Run(router *router.Switch) error {
	h.listen(router)
	h.closed <- struct{}{}

	return nil
}

func (h *httpd) Send(id uint32, msg []byte) {
}

func (h *httpd) listen(router *router.Switch) {
	h.Infof("listening on %v", h.addr)

	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(h.fs))
	mux.HandleFunc("/udp", func(w http.ResponseWriter, r *http.Request) {
		h.dispatch(w, r, router)
	})

	// ... listen and serve
	srv := &http.Server{
		Addr:    fmt.Sprintf("%v", h.addr),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			h.Fatalf("%v", err)
		}

		h.closed <- struct{}{}
	}()

	<-h.closing

	srv.Close()
}

func (h *httpd) dispatch(w http.ResponseWriter, r *http.Request, router *router.Switch) {
	switch strings.ToUpper(r.Method) {
	case http.MethodPost:
		h.post(w, r, router)

	default:
		http.Error(w, "Invalid request", http.StatusMethodNotAllowed)
	}
}

func (h *httpd) post(w http.ResponseWriter, r *http.Request, router *router.Switch) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	acceptsGzip := false
	contentType := ""
	body := struct {
		ID      int    `json:"ID"`
		Request []byte `json:"request"`
	}{}

	defer cancel()

	for k, h := range r.Header {
		if strings.TrimSpace(strings.ToLower(k)) == "content-type" {
			for _, v := range h {
				contentType = strings.TrimSpace(strings.ToLower(v))
			}
		}

		if strings.TrimSpace(strings.ToLower(k)) == "accept-encoding" {
			for _, v := range h {
				if strings.Contains(strings.TrimSpace(strings.ToLower(v)), "gzip") {
					acceptsGzip = true
				}
			}
		}
	}

	switch contentType {
	case "application/json":
		blob, err := io.ReadAll(r.Body)
		if err != nil {
			h.Warnf("%v", err)
			http.Error(w, "Error reading request", http.StatusInternalServerError)
			return
		}

		if err := json.Unmarshal(blob, &body); err != nil {
			h.Warnf("%v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

	default:
		h.Warnf("%v", fmt.Errorf("Invalid request content-type (%v)", contentType))
		http.Error(w, fmt.Sprintf("Invalid request content-type (%v)", contentType), http.StatusBadRequest)
		return
	}

	id := protocol.NextID()
	received := make(chan []byte)

	h.Dumpf(body.Request, "request %v  %v bytes from %v", id, len(body.Request), r.RemoteAddr)

	router.Received(id, body.Request, func(reply []byte) { received <- reply })

	select {
	case <-ctx.Done():
		h.Warnf("%v", ctx.Err())
		http.Error(w, "No response from controller", http.StatusInternalServerError)
		return

	case reply := <-received:
		h.Dumpf(reply, "reply %v  %v bytes for %v", id, len(reply), r.RemoteAddr)

		response := struct {
			ID    int   `json:"ID"`
			Reply slice `json:"reply"`
		}{
			ID:    body.ID,
			Reply: reply,
		}

		if b, err := json.Marshal(response); err != nil {
			h.Warnf("%v", err)
			http.Error(w, "Internal error generating response", http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")

			if acceptsGzip && len(b) > GZIP_MINIMUM {
				w.Header().Set("Content-Encoding", "gzip")

				gz := gzip.NewWriter(w)
				gz.Write(b)
				gz.Close()
			} else {
				w.Write(b)
			}
		}
	}
}
