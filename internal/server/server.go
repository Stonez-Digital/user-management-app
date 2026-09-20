package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)


func ValidateEnvironment() error {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		if strings.TrimSpace(os.Getenv("DB_DRIVER")) != "postgres" {
			return fmt.Errorf("production requires DB_DRIVER=postgres")
		}
		if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
			return fmt.Errorf("production requires DATABASE_URL")
		}
		if strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")) == "" {
			return fmt.Errorf("production requires CORS_ALLOWED_ORIGINS")
		}
	}
	return nil
}

func Configure(r *gin.Engine) {
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	allowed := configuredOrigins()
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			if origin == "" || allowed[origin] {
				c.Status(http.StatusNoContent)
				c.Abort()
				return
			}
			c.Status(http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	})
}

func configuredOrigins() map[string]bool {
	origins := map[string]bool{}
	for _, raw := range strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",") {
		if origin := strings.TrimSpace(raw); origin != "" {
			origins[origin] = true
		}
	}
	if len(origins) == 0 {
		origins["http://localhost:3000"] = true
	}
	return origins
}

func Start(r *gin.Engine, port string) error {
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-shutdownSignal():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

func shutdownSignal() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		// The signal channel is intentionally initialized in a small helper so
		// the HTTP server lifecycle remains isolated from application wiring.
		signalCh := make(chan os.Signal, 1)
		// SIGINT/SIGTERM are registered without importing signal handling into main.
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
		<-signalCh
		close(ch)
	}()
	return ch
}
