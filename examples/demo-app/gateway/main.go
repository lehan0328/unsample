// Service A: API Gateway
//
// Receives external requests and forwards them to the billing service.
// Demonstrates the entry point for Unsample debug tracing.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	unsample "github.com/unsample/unsample/sdk/go"

	"github.com/unsample/unsample/examples/demo-app/shared"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, err := shared.InitTracer(ctx, "gateway")
	if err != nil {
		log.Fatalf("init tracer: %v", err)
	}
	defer shutdown()

	secret := os.Getenv("UNSAMPLE_SECRET")
	billingURL := os.Getenv("BILLING_URL")
	if billingURL == "" {
		billingURL = "http://localhost:8081"
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/checkout", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("gateway")
		ctx, span := tracer.Start(r.Context(), "gateway.checkout")
		defer span.End()

		userID := r.URL.Query().Get("user")
		if userID == "" {
			userID = "anonymous"
		}

		// Forward to billing service.
		// otelhttp.NewTransport propagates TraceContext + Baggage automatically.
		client := &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}

		billReq, err := http.NewRequestWithContext(ctx, "POST",
			fmt.Sprintf("%s/charge?user=%s&amount=99.99", billingURL, userID), nil)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"building billing request: %v"}`, err), http.StatusInternalServerError)
			return
		}

		billResp, err := client.Do(billReq)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"calling billing service: %v"}`, err), http.StatusBadGateway)
			return
		}
		defer billResp.Body.Close()
		billBody, _ := io.ReadAll(billResp.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(billResp.StatusCode)
		fmt.Fprintf(w, `{"gateway":"processed","billing_status":%d,"billing_response":%s}`,
			billResp.StatusCode, string(billBody))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	// Wrap with Unsample middleware + OTel HTTP instrumentation.
	handler := unsample.Middleware(unsample.Config{Secret: secret})(
		otelhttp.NewHandler(mux, "gateway"),
	)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{Addr: ":" + port, Handler: handler}
	log.Printf("gateway starting on :%s (billing=%s)", port, billingURL)

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("gateway: %v", err)
		}
	}()

	<-ctx.Done()
	srv.Shutdown(context.Background())
	log.Println("gateway stopped")
}
