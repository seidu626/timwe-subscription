package monitoring

import (
	"context"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// ConfigWatcher monitors configuration files for changes
type ConfigWatcher struct {
	configPath string
	watcher    *fsnotify.Watcher
	logger     *zap.Logger
	stopChan   chan struct{}
	isRunning  bool
	callback   func()
}

// NewConfigWatcher creates a new configuration file watcher
func NewConfigWatcher(configPath string, logger *zap.Logger) *ConfigWatcher {
	return &ConfigWatcher{
		configPath: configPath,
		logger:     logger,
		stopChan:   make(chan struct{}),
	}
}

// Start begins watching the configuration file
func (cw *ConfigWatcher) Start(ctx context.Context, callback func()) error {
	if cw.isRunning {
		return nil
	}

	cw.callback = callback
	cw.isRunning = true

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	cw.watcher = watcher

	// Watch the directory containing the config file
	configDir := filepath.Dir(cw.configPath)
	if err := cw.watcher.Add(configDir); err != nil {
		return err
	}

	cw.logger.Info("Started watching configuration directory", zap.String("path", configDir))

	// Start watching goroutine
	go cw.watch(ctx)

	return nil
}

// Stop stops watching the configuration file
func (cw *ConfigWatcher) Stop() {
	if !cw.isRunning {
		return
	}

	cw.isRunning = false
	close(cw.stopChan)

	if cw.watcher != nil {
		cw.watcher.Close()
	}

	cw.logger.Info("Stopped watching configuration directory")
}

// watch monitors for file system events
func (cw *ConfigWatcher) watch(ctx context.Context) {
	defer func() {
		if cw.watcher != nil {
			cw.watcher.Close()
		}
	}()

	// Debounce timer for config changes
	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			return
		case <-cw.stopChan:
			return
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			// Check if the changed file is our config file
			if filepath.Clean(event.Name) == filepath.Clean(cw.configPath) {
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					cw.logger.Info("Configuration file changed",
						zap.String("file", event.Name),
						zap.String("operation", event.Op.String()))

					// Cancel previous timer if it exists
					if debounceTimer != nil {
						debounceTimer.Stop()
					}

					// Debounce rapid changes (wait 500ms before triggering callback)
					debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
						if cw.callback != nil {
							cw.callback()
						}
					})
				}
			}

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			cw.logger.Error("Configuration watcher error", zap.Error(err))
		}
	}
}

// IsRunning returns whether the watcher is running
func (cw *ConfigWatcher) IsRunning() bool {
	return cw.isRunning
}
