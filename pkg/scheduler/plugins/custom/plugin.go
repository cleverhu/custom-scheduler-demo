package custom

import (
	"context"
	"fmt"

	"github.com/cleverhu/custom-scheduler/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	frameworkruntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "CustomSchedulingPlugin"

	// ConfigMapName is the name of the ConfigMap containing the configuration
	ConfigMapName = "local-path-config"

	// ConfigMapNamespace is the namespace of the ConfigMap
	ConfigMapNamespace = "kube-system"

	// ConfigKey is the key in the ConfigMap data that contains our configuration
	ConfigKey = "config.json"
)

// PluginConfig 定义插件的配置结构
type PluginConfig struct {
	// ConfigMapName 是包含配置的 ConfigMap 名称
	ConfigMapName string `json:"configMapName,omitempty"`
	// ConfigMapNamespace 是 ConfigMap 所在的命名空间
	ConfigMapNamespace string `json:"configMapNamespace,omitempty"`
	// ConfigKey 是 ConfigMap 中的配置键名
	ConfigKey string `json:"configKey,omitempty"`
}

var _ framework.PreFilterPlugin = &Plugin{}
var _ framework.FilterPlugin = &Plugin{}
var _ framework.ScorePlugin = &Plugin{}
var _ framework.ReservePlugin = &Plugin{}

// Plugin is the implementation of scheduler plugin.
type Plugin struct {
	handle        framework.Handle
	configManager *config.Manager
	clientset     kubernetes.Interface
	// cmSharedInformer cache.SharedIndexInformer
	pluginConfig *PluginConfig
}

type Args struct {
	StorageConfig *PluginConfig `json:"storageConfig,omitempty"`
}

// New initializes a new plugin and returns it.
func New(ctx context.Context, configuration runtime.Object, f framework.Handle) (framework.Plugin, error) {
	klog.V(2).InfoS("Creating new CustomSchedulingPlugin")

	// 创建默认配置
	pluginConfig := &PluginConfig{
		ConfigMapName:      ConfigMapName,
		ConfigMapNamespace: ConfigMapNamespace,
		ConfigKey:          ConfigKey,
	}

	klog.Infof("pluginConfig: %+v", configuration)
	// 如果提供了配置对象，尝试解析它
	if configuration != nil {
		klog.V(2).InfoS("Using provided configuration")
		args := &Args{}
		err := frameworkruntime.DecodeInto(configuration, args)
		if err != nil {
			return nil, fmt.Errorf("failed to decode configuration: %v", err)
		}
		klog.Infof("args: %+v", *args.StorageConfig)

		// args, ok := configuration.(*apiconfig.CustomSchedulerArgs)
		// if !ok {
		// 	return nil, fmt.Errorf("want args to be of type CustomSchedulerArgs, got %T", configuration)
		// }

		// 从参数中读取配置
		if args.StorageConfig != nil {
			pluginConfig = args.StorageConfig
		}
	}

	configManager := config.NewManager()
	clientset := f.ClientSet()

	plugin := &Plugin{
		handle:        f,
		configManager: configManager,
		clientset:     clientset,
		pluginConfig:  pluginConfig,
	}

	// 使用配置加载初始配置
	if err := plugin.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load initial config: %v", err)
	}

	return plugin, nil
}

// loadConfig loads the configuration from the ConfigMap
func (p *Plugin) loadConfig() error {
	// Get the ConfigMap
	cm, err := p.clientset.CoreV1().ConfigMaps(p.pluginConfig.ConfigMapNamespace).Get(
		context.Background(),
		p.pluginConfig.ConfigMapName,
		metav1.GetOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to get ConfigMap %s/%s: %v",
			p.pluginConfig.ConfigMapNamespace,
			p.pluginConfig.ConfigMapName,
			err,
		)
	}

	// Get the configuration data
	configData, exists := cm.Data[p.pluginConfig.ConfigKey]
	if !exists {
		return fmt.Errorf("ConfigMap %s/%s does not contain key %s",
			p.pluginConfig.ConfigMapNamespace,
			p.pluginConfig.ConfigMapName,
			p.pluginConfig.ConfigKey,
		)
	}

	// Load the configuration
	if err := p.configManager.LoadConfig([]byte(configData)); err != nil {
		return fmt.Errorf("failed to load config data: %v", err)
	}

	klog.V(2).InfoS("Successfully loaded configuration from ConfigMap",
		"configmap", p.pluginConfig.ConfigMapName,
		"namespace", p.pluginConfig.ConfigMapNamespace,
		"key", p.pluginConfig.ConfigKey,
	)

	return nil
}

// Name returns the name of the plugin.
func (p *Plugin) Name() string {
	return Name
}

// PreFilter performs the pre-filter operation.
func (p *Plugin) PreFilter(ctx context.Context, state *framework.CycleState, pod *corev1.Pod) (*framework.PreFilterResult, *framework.Status) {
	klog.V(3).InfoS("Running PreFilter", "pod", klog.KObj(pod))

	// Reload configuration to ensure we have the latest
	if err := p.loadConfig(); err != nil {
		klog.ErrorS(err, "Failed to reload config in PreFilter")
		// Continue with existing config if reload fails
	}

	nodes, err := p.handle.SnapshotSharedLister().NodeInfos().List()
	if err != nil {
		return nil, framework.NewStatus(framework.Error, fmt.Sprintf("error listing nodes: %v", err))
	}

	// Check if there are any nodes available
	if len(nodes) == 0 {
		return nil, framework.NewStatus(framework.Unschedulable, "no nodes available to schedule")
	}

	// If we have default path configured, we can proceed with all nodes
	if p.configManager.GetConfig().HasDefaultPath() {
		return nil, framework.NewStatus(framework.Success)
	}

	// Check if there are any allowed nodes
	hasAllowedNode := false
	for _, nodeInfo := range nodes {
		if p.configManager.IsNodeAllowed(nodeInfo.Node().Name) {
			hasAllowedNode = true
			break
		}
	}

	if !hasAllowedNode {
		return nil, framework.NewStatus(framework.Unschedulable, "no nodes match the path configuration")
	}

	return nil, framework.NewStatus(framework.Success)
}

// PreFilterExtensions returns prefilter extensions, pod add and remove.
func (p *Plugin) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

// Filter checks if the node matches the pod requirements.
func (p *Plugin) Filter(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	klog.V(3).InfoS("Running Filter", "pod", klog.KObj(pod), "node", klog.KObj(nodeInfo.Node()))

	if nodeInfo.Node() == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}

	// Check if the node is allowed based on configuration
	if !p.configManager.IsNodeAllowed(nodeInfo.Node().Name) {
		return framework.NewStatus(framework.Unschedulable, "node is not in the allowed nodes list")
	}

	return framework.NewStatus(framework.Success)
}

// Score ranks nodes that have passed the filtering stage.
func (p *Plugin) Score(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeName string) (int64, *framework.Status) {
	nodeInfo, err := p.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
	}

	score := int64(50) // Default score
	node := nodeInfo.Node()

	// Give higher score to nodes with more paths available
	if paths := p.configManager.GetNodePaths(nodeName); paths != nil {
		score += int64(len(paths) * 10)
	}

	klog.V(4).InfoS("Calculated score", "pod", klog.KObj(pod), "node", klog.KObj(node), "score", score)
	return score, framework.NewStatus(framework.Success)
}

// ScoreExtensions returns the score extension.
func (p *Plugin) ScoreExtensions() framework.ScoreExtensions {
	return nil
}

// Reserve is called when the scheduler reserves a pod on a node.
func (p *Plugin) Reserve(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeName string) *framework.Status {
	klog.V(3).InfoS("Running Reserve", "pod", klog.KObj(pod), "node", nodeName)
	return framework.NewStatus(framework.Success)
}

// Unreserve is called when a reserved pod was rejected.
func (p *Plugin) Unreserve(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeName string) {
	klog.V(3).InfoS("Running Unreserve", "pod", klog.KObj(pod), "node", nodeName)
}
