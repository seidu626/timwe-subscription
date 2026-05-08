package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/utils"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// PanicRecoveryMiddleware provides panic recovery for HTTP handlers
type PanicRecoveryMiddleware struct {
	panicHandler *utils.PanicHandler
	logger       *zap.Logger
}

// NewPanicRecoveryMiddleware creates a new panic recovery middleware
func NewPanicRecoveryMiddleware(logger *zap.Logger, panicHandler *utils.PanicHandler) *PanicRecoveryMiddleware {
	return &PanicRecoveryMiddleware{
		panicHandler: panicHandler,
		logger:       logger,
	}
}

// Wrap wraps a standard HTTP handler with panic recovery
func (m *PanicRecoveryMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				m.handlePanic(rec, r.Context(), w)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// WrapFastHTTP wraps a FastHTTP handler with panic recovery
func (m *PanicRecoveryMiddleware) WrapFastHTTP(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		defer func() {
			if rec := recover(); rec != nil {
				m.handleFastHTTPPanic(rec, ctx)
			}
		}()
		next(ctx)
	}
}

// handlePanic handles panics in standard HTTP handlers
func (m *PanicRecoveryMiddleware) handlePanic(rec interface{}, ctx context.Context, w http.ResponseWriter) {
	// Log the panic
	m.logger.Error("HTTP HANDLER PANIC",
		zap.Any("panic_value", rec),
		zap.String("panic_type", fmt.Sprintf("%T", rec)),
		zap.String("timestamp", time.Now().Format(time.RFC3339)),
		zap.String("stack_trace", string(debug.Stack())),
	)

	// Use panic handler if available
	if m.panicHandler != nil {
		m.panicHandler.HandlePanic(rec, ctx)
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Return error response
	w.WriteHeader(http.StatusInternalServerError)
	errorResponse := fmt.Sprintf(`{"error":"Internal Server Error","message":"A panic occurred","timestamp":"%s"}`,
		time.Now().Format(time.RFC3339))
	w.Write([]byte(errorResponse))
}

// handleFastHTTPPanic handles panics in FastHTTP handlers
func (m *PanicRecoveryMiddleware) handleFastHTTPPanic(rec interface{}, ctx *fasthttp.RequestCtx) {
	// Log the panic
	m.logger.Error("FastHTTP HANDLER PANIC",
		zap.Any("panic_value", rec),
		zap.String("panic_type", fmt.Sprintf("%T", rec)),
		zap.String("timestamp", time.Now().Format(time.RFC3339)),
		zap.String("stack_trace", string(debug.Stack())),
		zap.String("method", string(ctx.Method())),
		zap.String("uri", string(ctx.RequestURI())),
		zap.String("user_agent", string(ctx.UserAgent())),
		zap.String("remote_addr", ctx.RemoteAddr().String()),
	)

	// Use panic handler if available
	if m.panicHandler != nil {
		// Create a context for the panic handler
		panicCtx := context.Background()
		if deadline, ok := ctx.Deadline(); ok {
			var cancel context.CancelFunc
			panicCtx, cancel = context.WithDeadline(context.Background(), deadline)
			defer cancel()
		}
		m.panicHandler.HandlePanic(rec, panicCtx)
	}

	// Set appropriate headers
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.Response.Header.Set("X-Content-Type-Options", "nosniff")

	// Return error response
	ctx.Response.SetStatusCode(http.StatusInternalServerError)
	errorResponse := fmt.Sprintf(`{"error":"Internal Server Error","message":"A panic occurred","timestamp":"%s"}`,
		time.Now().Format(time.RFC3339))
	ctx.Response.SetBodyString(errorResponse)
}

// WrapWithMetrics wraps a handler with panic recovery and metrics
func (m *PanicRecoveryMiddleware) WrapWithMetrics(next http.Handler, handlerName string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		defer func() {
			if rec := recover(); rec != nil {
				// Log metrics
				duration := time.Since(start)
				m.logger.Error("HTTP HANDLER PANIC WITH METRICS",
					zap.Any("panic_value", rec),
					zap.String("handler_name", handlerName),
					zap.Duration("duration", duration),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.String("remote_addr", r.RemoteAddr),
					zap.String("user_agent", r.UserAgent()),
				)

				m.handlePanic(rec, r.Context(), w)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// WrapFastHTTPWithMetrics wraps a FastHTTP handler with panic recovery and metrics
func (m *PanicRecoveryMiddleware) WrapFastHTTPWithMetrics(next fasthttp.RequestHandler, handlerName string) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()

		defer func() {
			if rec := recover(); rec != nil {
				// Log metrics
				duration := time.Since(start)
				m.logger.Error("FastHTTP HANDLER PANIC WITH METRICS",
					zap.Any("panic_value", rec),
					zap.String("handler_name", handlerName),
					zap.Duration("duration", duration),
					zap.String("method", string(ctx.Method())),
					zap.String("path", string(ctx.RequestURI())),
					zap.String("remote_addr", ctx.RemoteAddr().String()),
					zap.String("user_agent", string(ctx.UserAgent())),
				)

				m.handleFastHTTPPanic(rec, ctx)
			}
		}()

		next(ctx)
	}
}

// GetPanicHandler returns the underlying panic handler
func (m *PanicRecoveryMiddleware) GetPanicHandler() *utils.PanicHandler {
	return m.panicHandler
}

// UpdatePanicHandlerConfig updates the panic handler configuration
func (m *PanicRecoveryMiddleware) UpdatePanicHandlerConfig(config *utils.PanicHandlerConfig) {
	if m.panicHandler != nil {
		m.panicHandler.UpdateConfig(config)
		m.logger.Info("Panic recovery middleware configuration updated",
			zap.Bool("enable_recovery", config.EnableRecovery),
			zap.Bool("log_stack_traces", config.LogStackTraces),
			zap.Bool("exit_on_fatal", config.ExitOnFatal),
		)
	}
}
