package monitoring

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// RealTimeMonitor handles WebSocket connections for real-time updates
type RealTimeMonitor struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan interface{}
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
	logger     *zap.Logger
	monitor    *ChargingFailureMonitor
	isRunning  bool
	stopChan   chan struct{}
}

// RealTimeMessage represents a message sent to WebSocket clients
type RealTimeMessage struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin for now
		// In production, implement proper origin checking
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// NewRealTimeMonitor creates a new real-time monitor
func NewRealTimeMonitor(monitor *ChargingFailureMonitor, logger *zap.Logger) *RealTimeMonitor {
	return &RealTimeMonitor{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan interface{}, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		logger:     logger,
		monitor:    monitor,
		stopChan:   make(chan struct{}),
	}
}

// Start begins the real-time monitoring service
func (rt *RealTimeMonitor) Start(ctx context.Context) error {
	if rt.isRunning {
		return nil
	}

	rt.isRunning = true
	rt.logger.Info("Starting real-time monitor")

	// Start the hub goroutine
	go rt.run(ctx)

	// Start periodic data broadcasting
	go rt.periodicBroadcast(ctx)

	return nil
}

// Stop stops the real-time monitoring service
func (rt *RealTimeMonitor) Stop() {
	if !rt.isRunning {
		return
	}

	rt.logger.Info("Stopping real-time monitor")
	close(rt.stopChan)
	rt.isRunning = false
}

// HandleWebSocket handles WebSocket connection upgrades
func (rt *RealTimeMonitor) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		rt.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	rt.logger.Info("New WebSocket connection established",
		zap.String("remote_addr", conn.RemoteAddr().String()))

	// Register the connection
	rt.register <- conn

	// Handle the connection
	go rt.handleConnection(conn)
}

// run manages WebSocket connections and message broadcasting
func (rt *RealTimeMonitor) run(ctx context.Context) {
	defer func() {
		// Close all connections when shutting down
		rt.mu.Lock()
		for conn := range rt.clients {
			conn.Close()
		}
		rt.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rt.stopChan:
			return

		case conn := <-rt.register:
			rt.mu.Lock()
			rt.clients[conn] = true
			rt.mu.Unlock()
			rt.logger.Info("Client registered",
				zap.Int("total_clients", len(rt.clients)))

			// Send initial data to new client
			rt.sendInitialData(conn)

		case conn := <-rt.unregister:
			rt.mu.Lock()
			if _, ok := rt.clients[conn]; ok {
				delete(rt.clients, conn)
				conn.Close()
			}
			rt.mu.Unlock()
			rt.logger.Info("Client unregistered",
				zap.Int("total_clients", len(rt.clients)))

		case message := <-rt.broadcast:
			rt.mu.RLock()
			for conn := range rt.clients {
				select {
				case <-ctx.Done():
					rt.mu.RUnlock()
					return
				default:
					if err := rt.sendMessage(conn, message); err != nil {
						rt.logger.Warn("Failed to send message to client", zap.Error(err))
						// Remove failed connection
						delete(rt.clients, conn)
						conn.Close()
					}
				}
			}
			rt.mu.RUnlock()
		}
	}
}

// handleConnection handles individual WebSocket connections
func (rt *RealTimeMonitor) handleConnection(conn *websocket.Conn) {
	defer func() {
		rt.unregister <- conn
	}()

	// Set read deadline and pong handler for keepalive
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start ping ticker
	ticker := time.NewTicker(54 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// Read messages from client (mainly for keepalive)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				rt.logger.Error("WebSocket error", zap.Error(err))
			}
			break
		}
	}
}

// sendMessage sends a message to a specific WebSocket connection
func (rt *RealTimeMonitor) sendMessage(conn *websocket.Conn, message interface{}) error {
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(message)
}

// sendInitialData sends initial dashboard data to a new client
func (rt *RealTimeMonitor) sendInitialData(conn *websocket.Conn) {
	dashboardData := rt.monitor.GetDashboardData()

	message := RealTimeMessage{
		Type:      "initial_data",
		Timestamp: time.Now(),
		Data:      dashboardData,
	}

	if err := rt.sendMessage(conn, message); err != nil {
		rt.logger.Error("Failed to send initial data", zap.Error(err))
	}
}

// periodicBroadcast sends periodic updates to all connected clients
func (rt *RealTimeMonitor) periodicBroadcast(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // More frequent updates via WebSocket
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rt.stopChan:
			return
		case <-ticker.C:
			rt.broadcastUpdate()
		}
	}
}

// broadcastUpdate broadcasts current metrics to all connected clients
func (rt *RealTimeMonitor) broadcastUpdate() {
	if len(rt.clients) == 0 {
		return // No clients connected
	}

	dashboardData := rt.monitor.GetDashboardData()

	message := RealTimeMessage{
		Type:      "metrics_update",
		Timestamp: time.Now(),
		Data:      dashboardData,
	}

	select {
	case rt.broadcast <- message:
	default:
		rt.logger.Warn("Broadcast channel full, dropping message")
	}
}

// BroadcastAlert sends an alert immediately to all connected clients
func (rt *RealTimeMonitor) BroadcastAlert(alert *Alert) {
	if len(rt.clients) == 0 {
		return
	}

	message := RealTimeMessage{
		Type:      "alert",
		Timestamp: time.Now(),
		Data:      alert,
	}

	select {
	case rt.broadcast <- message:
	default:
		rt.logger.Warn("Broadcast channel full, dropping alert")
	}
}

// BroadcastMetricsUpdate sends metrics update immediately
func (rt *RealTimeMonitor) BroadcastMetricsUpdate(metrics *ChargingFailureMetrics) {
	if len(rt.clients) == 0 {
		return
	}

	message := RealTimeMessage{
		Type:      "metrics_update",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"metrics": metrics,
		},
	}

	select {
	case rt.broadcast <- message:
	default:
		rt.logger.Warn("Broadcast channel full, dropping metrics update")
	}
}

// GetConnectedClients returns the number of connected WebSocket clients
func (rt *RealTimeMonitor) GetConnectedClients() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return len(rt.clients)
}

// IsRunning returns whether the real-time monitor is running
func (rt *RealTimeMonitor) IsRunning() bool {
	return rt.isRunning
}
