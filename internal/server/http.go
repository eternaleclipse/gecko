package server

import (
	"crypto/subtle"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/gecko-term/gecko/internal/hub"
	"github.com/gecko-term/gecko/web"
)

const cookieName = "gecko_token"

// HTTP serves the web client and its WebSocket.
type HTTP struct {
	Hub    *hub.Hub
	Token  string
	Listen string
}

// Handler returns the http.Handler.
func (s *HTTP) Handler() http.Handler {
	mux := http.NewServeMux()
	dist, _ := fs.Sub(web.Dist, "dist")
	if dir := os.Getenv("GECKO_WEB_DIR"); dir != "" {
		dist = os.DirFS(dir) // development: serve the client from disk
	}
	files := http.FileServer(http.FS(dist))
	mux.HandleFunc("/ws", s.ws)
	mux.HandleFunc("/api/info", s.info)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if t := r.URL.Query().Get("token"); t != "" {
			if !s.tokenOK(t) {
				http.Error(w, "bad token", http.StatusForbidden)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: cookieName, Value: t, Path: "/", HttpOnly: true,
				SameSite: http.SameSiteStrictMode, Expires: time.Now().Add(400 * 24 * time.Hour)})
			q := r.URL.Query()
			q.Del("token")
			r.URL.RawQuery = q.Encode()
			http.Redirect(w, r, r.URL.String(), http.StatusFound)
			return
		}
		// Static assets are public; the app shell is too, it just can't
		// connect without the token.
		if r.URL.Path == "/" {
			if _, err := fs.Stat(dist, "index.html"); err != nil {
				http.Error(w, "web client not built: run `make web` and rebuild gecko", http.StatusNotFound)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
	return securityHeaders(mux)
}

func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		h.ServeHTTP(w, r)
	})
}

func (s *HTTP) tokenOK(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.Token)) == 1
}

func (s *HTTP) authed(r *http.Request) bool {
	if c, err := r.Cookie(cookieName); err == nil && s.tokenOK(c.Value) {
		return true
	}
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") && s.tokenOK(a[7:]) {
		return true
	}
	return false
}

// sameOrigin rejects cross-site WebSocket hijacking.
func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return true // non-browser client
	}
	u, err := url.Parse(o)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize: 32 << 10, WriteBufferSize: 64 << 10,
	EnableCompression: true,
	CheckOrigin:       sameOrigin,
}

func (s *HTTP) ws(w http.ResponseWriter, r *http.Request) {
	if !s.authed(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c.SetReadLimit(16 << 20)
	Serve(s.Hub, newWSConn(c))
}

func (s *HTTP) info(w http.ResponseWriter, r *http.Request) {
	if !s.authed(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"host": s.Hub.Name(),
		"urls": ShareURLs(s.Listen, s.Token),
	})
}

// ShareURLs lists URLs (with token) other devices can open, e.g. a phone on
// the same network or tailnet. Empty when listening on loopback only.
func ShareURLs(listen, token string) []string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() || host == "localhost" {
		return nil
	}
	var hosts []string
	if host != "" && host != "0.0.0.0" && host != "::" {
		hosts = []string{host}
	} else if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() && n.IP.To4() != nil {
				hosts = append(hosts, n.IP.String())
			}
		}
	}
	var out []string
	for _, h := range hosts {
		out = append(out, "http://"+net.JoinHostPort(h, port)+"/?token="+token)
	}
	return out
}

// wsConn adapts a gorilla WebSocket to proto.Conn.
type wsConn struct {
	c  *websocket.Conn
	mu sync.Mutex
}

func newWSConn(c *websocket.Conn) *wsConn {
	w := &wsConn{c: c}
	// Keep NATs and mobile networks from silently dropping idle sockets.
	go func() {
		t := time.NewTicker(25 * time.Second)
		defer t.Stop()
		for range t.C {
			w.mu.Lock()
			err := c.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			w.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return w
}

func (w *wsConn) Read() (bool, []byte, error) {
	t, b, err := w.c.ReadMessage()
	return t == websocket.BinaryMessage, b, err
}

func (w *wsConn) Write(bin bool, data []byte) error {
	t := websocket.TextMessage
	if bin {
		t = websocket.BinaryMessage
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.c.SetWriteDeadline(time.Now().Add(30 * time.Second))
	return w.c.WriteMessage(t, data)
}

func (w *wsConn) Close() error { return w.c.Close() }
