package servion

import (
	"testing"
)

func TestHttpServerScanner_ScannerBeans(t *testing.T) {
	scanner := HttpServerScanner("my-server")
	beans := scanner.ScannerBeans()

	if len(beans) < 2 {
		t.Fatalf("expected at least 2 beans, got %d", len(beans))
	}

	// First bean should be HttpServerFactory
	if _, ok := beans[0].(*implHttpServerFactory); !ok {
		t.Errorf("beans[0] type = %T, want *implHttpServerFactory", beans[0])
	}
}

func TestHttpServerScanner_WithExtraBeans(t *testing.T) {
	extra1 := &testHandler{pattern: "/api/test"}
	extra2 := &testHandler{pattern: "/api/hello"}

	scanner := HttpServerScanner("my-server", extra1, extra2)
	beans := scanner.ScannerBeans()

	// 2 base beans + 2 extra
	if len(beans) != 4 {
		t.Fatalf("expected 4 beans, got %d", len(beans))
	}

	if beans[2] != extra1 {
		t.Error("beans[2] should be extra1")
	}
	if beans[3] != extra2 {
		t.Error("beans[3] should be extra2")
	}
}

func TestHttpServerScanner_FactoryBeanName(t *testing.T) {
	scanner := HttpServerScanner("api-server")
	beans := scanner.ScannerBeans()

	factory := beans[0].(*implHttpServerFactory)
	if factory.beanName != "api-server" {
		t.Errorf("beanName = %q, want api-server", factory.beanName)
	}
}
