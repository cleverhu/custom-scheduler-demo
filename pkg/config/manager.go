package config

import (
	"encoding/json"
	"fmt"
	"sync"

	"k8s.io/klog/v2"
)

// Manager handles the scheduler configuration
type Manager struct {
	mu     sync.RWMutex
	config *SchedulerConfig
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{
		config: &SchedulerConfig{},
	}
}

// LoadConfig loads configuration from JSON data
func (m *Manager) LoadConfig(data []byte) error {
	var config SchedulerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = &config
	klog.V(2).InfoS("Loaded scheduler configuration", "config", config)
	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *SchedulerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// IsNodeAllowed checks if a node is allowed based on current configuration
func (m *Manager) IsNodeAllowed(nodeName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.IsNodeAllowed(nodeName)
}

// GetNodePaths returns the paths for a specific node
func (m *Manager) GetNodePaths(nodeName string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.GetNodePaths(nodeName)
}
