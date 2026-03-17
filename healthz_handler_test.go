package servion

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler_Pattern(t *testing.T) {
	h := &implHealthHandler{HealthPattern: "/healthz"}
	if got := h.Pattern(); got != "/healthz" {
		t.Errorf("Pattern() = %q, want /healthz", got)
	}
}

func TestHealthHandler_Up(t *testing.T) {
	h := &implHealthHandler{
		Runtime:       newMockRuntime(true),
		HealthPattern: "/healthz",
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Status != "UP" {
		t.Errorf("status = %q, want UP", resp.Status)
	}
	if resp.Components != nil {
		t.Error("expected no components in non-detailed mode")
	}
}

func TestHealthHandler_Down(t *testing.T) {
	h := &implHealthHandler{
		Runtime:       newMockRuntime(false),
		HealthPattern: "/healthz",
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var resp healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Status != "DOWN" {
		t.Errorf("status = %q, want DOWN", resp.Status)
	}
}

func TestHealthHandler_Detailed(t *testing.T) {
	comp := &mockComponent{
		name:  "db",
		stats: map[string]string{"connections": "5", "latency": "2ms"},
	}

	h := &implHealthHandler{
		Runtime:       newMockRuntime(true),
		Components:    []Component{comp},
		HealthPattern: "/healthz",
		Detailed:      true,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Components == nil {
		t.Fatal("expected components in detailed mode")
	}
	dbStats, ok := resp.Components["db"]
	if !ok {
		t.Fatal("expected 'db' component")
	}
	if dbStats["connections"] != "5" {
		t.Errorf("connections = %q, want 5", dbStats["connections"])
	}
	if dbStats["latency"] != "2ms" {
		t.Errorf("latency = %q, want 2ms", dbStats["latency"])
	}
}

func TestHealthHandler_DetailedNoComponents(t *testing.T) {
	h := &implHealthHandler{
		Runtime:       newMockRuntime(true),
		HealthPattern: "/healthz",
		Detailed:      true,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(w, r)

	var resp healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Components != nil {
		t.Error("expected no components when none registered")
	}
}

func TestHealthHandler_MethodNotAllowed(t *testing.T) {
	h := &implHealthHandler{
		Runtime:       newMockRuntime(true),
		HealthPattern: "/healthz",
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(method, "/healthz", nil)
			h.ServeHTTP(w, r)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestHealthHandler_Head(t *testing.T) {
	h := &implHealthHandler{
		Runtime:       newMockRuntime(true),
		HealthPattern: "/healthz",
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/healthz", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHealthHandler_Factory(t *testing.T) {
	h := HealthHandler()
	if h == nil {
		t.Fatal("HealthHandler() returned nil")
	}
}

func TestHealthHandler_MultipleComponents(t *testing.T) {
	comp1 := &mockComponent{
		name:  "cache",
		stats: map[string]string{"hits": "100"},
	}
	comp2 := &mockComponent{
		name:  "db",
		stats: map[string]string{"pool": "10"},
	}

	h := &implHealthHandler{
		Runtime:       newMockRuntime(true),
		Components:    []Component{comp1, comp2},
		HealthPattern: "/healthz",
		Detailed:      true,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(w, r)

	var resp healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(resp.Components) != 2 {
		t.Errorf("expected 2 components, got %d", len(resp.Components))
	}
}

func TestHealthHandler_ContentType(t *testing.T) {
	h := &implHealthHandler{
		Runtime:       newMockRuntime(true),
		HealthPattern: "/healthz",
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
