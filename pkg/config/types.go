package config

// NodePath defines the mapping between node and its storage paths
type NodePath struct {
	Node  string   `json:"node"`
	Paths []string `json:"paths"`
}

// SchedulerConfig defines the configuration for the custom scheduler
type SchedulerConfig struct {
	NodePathMap []NodePath `json:"nodePathMap"`
}

// IsDefaultPath checks if the node path is the default path configuration
const DefaultNodePath = "DEFAULT_PATH_FOR_NON_LISTED_NODES"

// HasDefaultPath checks if the configuration contains default path
func (c *SchedulerConfig) HasDefaultPath() bool {
	for _, np := range c.NodePathMap {
		if np.Node == DefaultNodePath {
			return true
		}
	}
	return false
}

// GetNodePaths returns the paths for a specific node
func (c *SchedulerConfig) GetNodePaths(nodeName string) []string {
	for _, np := range c.NodePathMap {
		if np.Node == nodeName {
			return np.Paths
		}
	}
	return nil
}

// IsNodeAllowed checks if a node is allowed based on the configuration
func (c *SchedulerConfig) IsNodeAllowed(nodeName string) bool {
	// If default path is configured, all nodes are allowed
	if c.HasDefaultPath() {
		return true
	}

	// Check if node is explicitly listed
	for _, np := range c.NodePathMap {
		if np.Node == nodeName {
			return true
		}
	}
	return false
}
