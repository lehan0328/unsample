// Service B: Billing
//
// Processes payment charges. Simulates a slow DB query (300ms) and
// returns 500 for user ID "666" to demonstrate error tracing.
// Calls the notification service after processing.
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
	"time"

	unsample "github.com/unsample/unsample/sdk/go"

	"github.com/unsample/unsample/examples/demo-app/shared"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, err := shared.InitTracer(ctx, "billing-service")
	if err != nil {
		log.Fatalf("init tracer: %v", err)
	}
	defer shutdown()

	secret := os.Getenv("UNSAMPLE_SECRET")
	notificationURL := os.Getenv("NOTIFICATION_URL")
	if notificationURL == "" {
		notificationURL = "http://localhost:8082"
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/charge", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("billing-service")
		ctx, span := tracer.Start(r.Context(), "billing.charge")
		defer span.End()

		userID := r.URL.Query().Get("user")
		amount := r.URL.Query().Get("amount")
		span.SetAttributes(
			attribute.String("user.id", userID),
			attribute.String("charge.amount", amount),
		)

		// Simulate slow DB query.
		dbCtx, dbSpan := tracer.Start(ctx, "billing.db.query")
		dbSpan.SetAttributes(attribute.String("db.statement", "SELECT * FROM subscriptions WHERE user_id = ?"))
		time.Sleep(300 * time.Millisecond) // Simulated latency.
		dbSpan.End()

		// Simulated bug: user "666" triggers an error.
		if userID == "666" {
			span.SetStatus(codes.Error, "subscription_not_found")
			span.SetAttributes(attribute.String("error.type", "subscription_not_found"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":"subscription_not_found","user":"666"}`)
			return
		}

		// Call notification service.
		client := &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}

		notifReq, err := http.NewRequestWithContext(dbCtx, "POST",
			fmt.Sprintf("%s/send?user=%s&type=payment_confirmation", notificationURL, userID), nil)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
			return
		}

		notifResp, err := client.Do(notifReq)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			http.Error(w, fmt.Sprintf(`{"error":"calling notification: %v"}`, err), http.StatusBadGateway)
			return
		}
		defer notifResp.Body.Close()
		notifBody, _ := io.ReadAll(notifResp.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"charged":true,"amount":%s,"user":"%s","notification":%s}`,
			amount, userID, string(notifBody))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	handler := unsample.Middleware(unsample.Config{Secret: secret})(
		otelhttp.NewHandler(mux, "billing-service"),
	)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	srv := &http.Server{Addr: ":" + port, Handler: handler}
	log.Printf("billing-service starting on :%s (notification=%s)", port, notificationURL)

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("billing-service: %v", err)
		}
	}()

	<-ctx.Done()
	srv.Shutdown(context.Background())
	log.Println("billing-service stopped")
}
