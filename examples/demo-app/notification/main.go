// Service C: Notification
//
// Sends fake notifications (email, SMS, push). This is the leaf service
// in the call chain — it doesn't call any other services.
package main

import (
	"context"
	"fmt"
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
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, err := shared.InitTracer(ctx, "notification")
	if err != nil {
		log.Fatalf("init tracer: %v", err)
	}
	defer shutdown()

	secret := os.Getenv("UNSAMPLE_SECRET")

	mux := http.NewServeMux()

	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("notification")
		_, span := tracer.Start(r.Context(), "notification.send")
		defer span.End()

		userID := r.URL.Query().Get("user")
		notifType := r.URL.Query().Get("type")
		span.SetAttributes(
			attribute.String("notification.user", userID),
			attribute.String("notification.type", notifType),
		)

		// Simulate sending notification (fast).
		time.Sleep(8 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"sent":true,"type":"%s","user":"%s"}`, notifType, userID)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	handler := otelhttp.NewHandler(
		unsample.Middleware(unsample.Config{Secret: secret})(mux),
		"notification",
	)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	srv := &http.Server{Addr: ":" + port, Handler: handler}
	log.Printf("notification starting on :%s", port)

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("notification: %v", err)
		}
	}()

	<-ctx.Done()
	srv.Shutdown(context.Background())
	log.Println("notification stopped")
}
