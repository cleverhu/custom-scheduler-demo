以下是关于 Kubernetes 自定义调度器实现方案的详细说明：

# Kubernetes 自定义调度器实现方案

## 目录

1. [调度器扩展（Scheduler Extender）](#1-调度器扩展)
2. [多调度器方案](#2-多调度器方案)
3. [调度框架插件（Scheduling Framework）](#3-调度框架插件)
4. [直接修改 kube-scheduler 源码](#4-直接修改-kube-scheduler-源码)

## 1. 调度器扩展

通过 HTTP/HTTPS 回调的方式扩展默认调度器的功能。这是最简单的方式，但性能较差。

### 配置示例

```yaml
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
profiles:
  - schedulerName: default-scheduler
    plugins:
      filter:
        enabled:
          - name: DefaultPreemption
          - name: NodeResourcesFit
      score:
        enabled:
          - name: NodeResourcesBalancedAllocation
    extenders:
      - urlPrefix: "http://localhost:8888"
        filterVerb: "filter"
        prioritizeVerb: "prioritize"
        weight: 1
        enableHTTPS: false
```

### 实现示例

```go
package main

import (
    "encoding/json"
    "net/http"

    v1 "k8s.io/api/core/v1"
    schedulerapi "k8s.io/kube-scheduler/extender/v1"
)

type Extender struct{}

func (e *Extender) filter(args schedulerapi.ExtenderArgs) *schedulerapi.ExtenderFilterResult {
    pod := args.Pod
    nodes := args.Nodes.Items
    filteredNodes := []v1.Node{}

    for _, node := range nodes {
        // 实现自定义过滤逻辑
        if customFilter(pod, node) {
            filteredNodes = append(filteredNodes, node)
        }
    }

    return &schedulerapi.ExtenderFilterResult{
        Nodes: &v1.NodeList{Items: filteredNodes},
    }
}

func main() {
    http.HandleFunc("/filter", func(w http.ResponseWriter, r *http.Request) {
        var args schedulerapi.ExtenderArgs
        if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        result := (&Extender{}).filter(args)
        if err := json.NewEncoder(w).Encode(result); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
        }
    })

    http.ListenAndServe(":8888", nil)
}
```

## 2. 多调度器方案

部署自己的调度器，与默认调度器并存。Pod 可以通过 `schedulerName` 字段选择使用哪个调度器。

### 调度器实现示例

```go
package main

import (
    "context"
    "fmt"

    v1 "k8s.io/api/core/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

type CustomScheduler struct {
    clientset *kubernetes.Clientset
}

func NewCustomScheduler() (*CustomScheduler, error) {
    config, err := rest.InClusterConfig()
    if err != nil {
        return nil, err
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, err
    }

    return &CustomScheduler{
        clientset: clientset,
    }, nil
}

func (s *CustomScheduler) Run(ctx context.Context) {
    // 监听未调度的 Pod
    // 实现调度逻辑
    // 绑定 Pod 到选中的节点
}

func (s *CustomScheduler) schedulePod(pod *v1.Pod) error {
    // 1. 过滤节点
    nodes, err := s.filterNodes(pod)
    if err != nil {
        return err
    }

    // 2. 对节点评分
    node := s.prioritizeNodes(pod, nodes)

    // 3. 绑定 Pod 到选中的节点
    return s.bind(pod, node)
}
```

### 使用自定义调度器的 Pod 示例

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: custom-scheduled-pod
spec:
  schedulerName: custom-scheduler # 指定使用自定义调度器
  containers:
    - name: container
      image: nginx
```

## 3. 调度框架插件

从 Kubernetes 1.15 开始引入的新特性，允许通过插件方式扩展调度器的各个阶段。这是目前推荐的方式。

### 插件实现示例

```go
package main

import (
    "context"

    v1 "k8s.io/api/core/v1"
    "k8s.io/kubernetes/pkg/scheduler/framework"
)

type CustomPlugin struct{}

func (*CustomPlugin) Name() string {
    return "CustomPlugin"
}

// Filter 扩展点实现
func (*CustomPlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
    // 实现过滤逻辑
    return framework.NewStatus(framework.Success)
}

// Score 扩展点实现
func (*CustomPlugin) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
    // 实现打分逻辑
    return 100, framework.NewStatus(framework.Success)
}

// 插件注册
func New(_ runtime.Object, _ framework.Handle) (framework.Plugin, error) {
    return &CustomPlugin{}, nil
}
```

### 配置示例

```yaml
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
profiles:
  - schedulerName: custom-scheduler
    plugins:
      filter:
        enabled:
          - name: CustomPlugin
      score:
        enabled:
          - name: CustomPlugin
```

## 4. 直接修改 kube-scheduler 源码

直接修改 Kubernetes 调度器源码，这种方式最灵活但维护成本最高。

## 比较与建议

| 方案         | 优点                           | 缺点                           | 适用场景                   |
| ------------ | ------------------------------ | ------------------------------ | -------------------------- |
| 调度器扩展   | 实现简单，无需修改现有调度器   | 性能开销大，功能受限           | 简单的自定义需求           |
| 多调度器     | 完全自主控制，不影响默认调度器 | 需要维护独立的调度器，实现复杂 | 特殊场景下的完全自定义需求 |
| 调度框架插件 | 性能好，可复用现有调度器功能   | 需要了解调度框架 API           | 推荐方案，适合大多数场景   |
| 修改源码     | 灵活性最大                     | 维护成本高，升级困难           | 特殊场景且必须深度定制     |

### 建议

1. 对于大多数自定义调度需求，建议使用调度框架插件方式
2. 如果需求简单，可以考虑调度器扩展方式
3. 如果需要完全不同的调度逻辑，可以考虑多调度器方案
4. 除非特殊情况，不建议直接修改源码

## 参考资料

- [Kubernetes Scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/)
- [Scheduling Framework](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/)
- [Multiple Schedulers](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-multiple-schedulers/)
