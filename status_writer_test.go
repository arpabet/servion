package servion

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestStatusWriter_RecordsFinalStatus(t *testing.T) {
	sw := &statusWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}

	sw.WriteHeader(http.StatusEarlyHints)
	sw.WriteHeader(http.StatusNotFound)
	sw.WriteHeader(http.StatusInternalServerError) // superfluous: net/http ignores it too

	if sw.status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", sw.status)
	}
}

func TestStatusWriter_ImplicitOK(t *testing.T) {
	sw := &statusWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}

	sw.Write([]byte("body"))
	sw.WriteHeader(http.StatusNotFound) // too late: Write already sent 200

	if sw.status != http.StatusOK || sw.written != 4 {
		t.Errorf("status = %d, written = %d; want 200, 4", sw.status, sw.written)
	}
}

func TestStatusWriter_Flush(t *testing.T) {
	rec := httptest.NewRecorder()
	var w http.ResponseWriter = &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	w.(http.Flusher).Flush()

	if !rec.Flushed {
		t.Error("Flush did not reach the underlying writer")
	}
}

// TestStatusWriter_ResponseControllerReachesConnection: handlers that extend
// deadlines for slow clients use http.ResponseController, which needs Unwrap.
func TestStatusWriter_ResponseControllerReachesConnection(t *testing.T) {
	errc := make(chan error, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		errc <- http.NewResponseController(sw).SetWriteDeadline(time.Now().Add(time.Minute))
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if err := <-errc; err != nil {
		t.Errorf("SetWriteDeadline through statusWriter: %v", err)
	}
}

// TestStatusWriter_WebSocketThroughFail2Ban upgrades a WebSocket through the
// fail2ban router middleware: gorilla/websocket asserts the writer is an
// http.Hijacker, so the wrapper must not hide it.
func TestStatusWriter_WebSocketThroughFail2Ban(t *testing.T) {
	upgrader := websocket.Upgrader{}
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return // Upgrade already wrote the error response
		}
		defer conn.Close()
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(mt, msg)
	})

	srv := httptest.NewServer(newTestFail2Ban(newFakeClock()).RouterMiddleware(echo))
	defer srv.Close()

	header := http.Header{"CF-Connecting-IP": {"203.0.113.7"}}
	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), header)
	if err != nil {
		status := "no response"
		if resp != nil {
			status = resp.Status
		}
		t.Fatalf("dial: %v (%s)", err, status)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, msg, err := conn.ReadMessage(); err != nil || string(msg) != "ping" {
		t.Fatalf("echo = %q, %v; want \"ping\"", msg, err)
	}
}
