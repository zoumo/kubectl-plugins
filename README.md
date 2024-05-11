# kubectl plugins
The `kubectl plugins` repository aims to collect a series of useful `kubectl` plugins for enhancing and extending the capabilities of `kubectl`. More plugins are planned to be added in the future.

## kubectl-watchdiff
`kubectl-watchdiff` is the first plugin in this repository. Its primary function is to monitor changes in Kubernetes resources through watch events and display them in YAML diff format. This plugin is designed to enable users to focus more on the specific changes in resources rather than the entire object event.

### Features
- **Field Ignoration**: Automatically ignores the `resourceVersion`, `managedFields` in ObjectMetadata, and the `kubectl.kubernetes.io/last-applied-configuration` in annotations.
- **Custom Ignoration**: Allows users to configure specific `keys` in `metadata` `labels` and `annotations` to be ignored.
- **Field Focus**: Supports using JSONPath expressions to specify fields that users want to focus on, helping to concentrate on specific changes within resources.

### Installation
Install `kubectl-watchdiff` directly from GitHub using the following `go install` command:
```sh
go install github.com/zoumo/kubectl-plugins/cmd/kubectl-watchdiff@latest
```
Make sure your `$GOPATH/bin` directory is in the system's PATH so that `kubectl` can find and execute the `kubectl-watchdiff` plugin.

### Usage
Here are some basic usage examples of `kubectl-watchdiff`:
```sh
# Monitor changes in all pods within a specified namespace:
kubectl watchdiff pods

# Monitor changes in a single pod:
kubectl watchdiff pod pod1

# Filter pods through label selection:
kubectl watchdiff pods -l app=foo

# Monitor all pods across all namespaces:
kubectl watchdiff pods --all-namespaces

# Ignore certain labels or annotations while comparing:
kubectl watchdiff pods --ignore-annotation-keys=kubectl.kubernetes.io/last-applied-configuration

# Only focus on the status changes of pods:
kubectl watchdiff pods --jsonpaths="$.status"

# Focus on specific annotation changes within pods:
kubectl watchdiff pods --jsonpaths="$.metadata.annotations['github.com/zoumo/kubectl-plugins']"
```
## Feedback and Contributions
Feedback, issues, or feature suggestions are welcome by submitting issues or contributing code to the [GitHub repository](https://github.com/zoumo/kubectl-plugins).
