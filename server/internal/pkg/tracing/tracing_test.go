package tracing

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
)

func TestProviderWithoutEndpointIsNoopSafe(t *testing.T) {
	provider, err := Provider(context.Background(), "dlidli-test", "")
	if err != nil {
		t.Fatalf("empty endpoint should not fail: %v", err)
	}
	if provider == nil {
		t.Fatal("provider should not be nil")
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown noop provider: %v", err)
	}
}

func TestSetupDefaultsServiceAndPropagator(t *testing.T) {
	provider, err := Setup(context.Background(), "", "")
	if err != nil {
		t.Fatalf("setup without endpoint: %v", err)
	}
	defer provider.Shutdown(context.Background())
	if otel.GetTextMapPropagator() == nil {
		t.Fatal("setup should install a text-map propagator")
	}
}

func TestMiddlewareKeepsRequestWorking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/health", func(c *gin.Context) { c.String(200, "ok") })
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || w.Body.String() != "ok" {
		t.Fatalf("middleware changed response: status=%d body=%q", w.Code, w.Body.String())
	}
}
