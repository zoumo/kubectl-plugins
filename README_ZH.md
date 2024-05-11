# kubectl plugins

`kubectl plugins` 仓库旨在收集一系列有用的 `kubectl` 插件，用于增强和扩展 `kubectl` 的功能。计划未来会添加更多的插件。

## kubectl-watchdiff

`kubectl-watchdiff` 是此仓库中的第一个插件，主要用于通过监听 Kubernetes 资源的变化事件并以 YAML 差异格式展示。该插件的目的是让用户能够更加专注地观察资源的具体变动部分，而不是完整的对象事件。

### 特点

- **字段忽略**：自动忽略 `resourceVersion`、`managedFields` 字段以及 annotations 中的 `kubectl.kubernetes.io/last-applied-configuration`。
- **定制忽略**：允许用户配置忽略 `metadata` 中的 `labels` 和 `annotations` 的特定 `key`。
- **字段关注**：支持通过 JSONPath 表达式来指定需要关注的字段，有助于用户集中注意力于资源中的特定变动。

### 安装

通过以下 `go install` 命令直接从 GitHub 安装 `kubectl-watchdiff`：

```sh
go install github.com/zoumo/kubectl-plugins/cmd/kubectl-watchdiff@latest
```

请确保您的 `$GOPATH/bin` 目录已在系统的 PATH 中，以便 `kubectl` 能够找到和执行 `kubectl-watchdiff` 插件。

### 使用

以下是一些 `kubectl-watchdiff` 的基本用法示例：

```sh
# 监控指定命名空间中所有 pod 的变化：
kubectl watchdiff pods

# 监控单个 pod 的变化：
kubectl watchdiff pod pod1

# 通过标签选择过滤 pods：
kubectl watchdiff pods -l app=foo

# 监控所有命名空间的所有 pods：
kubectl watchdiff pods --all-namespaces

# 在比较时忽略某些标签或注释：
kubectl watchdiff pods --ignore-annotation-keys=kubectl.kubernetes.io/last-applied-configuration

# 仅关注 pods 的状态变化：
kubectl watchdiff pods --jsonpaths="$.status"

# 关注 pods 中的特定 annotation 变化：
kubectl watchdiff pods --jsonpaths="$.metadata.annotations['github.com/zoumo/kubectl-plugins']"
```

## 反馈和贡献

欢迎通过在 [GitHub 仓库](https://github.com/zoumo/kubectl-plugins) 提交 issue 或贡献代码来反馈问题或提出新的功能建议。
