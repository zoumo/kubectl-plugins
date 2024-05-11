/**
 * Copyright 2024 jim.zoumo@gmail.com
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/ohler55/ojg/jp"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	watchtools "k8s.io/client-go/tools/watch"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/interrupt"

	"github.com/zoumo/kubectl-plugins/pkg/flags"
)

var example = `
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
`

func main() {
	cmd := newCommand()
	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}

func newCommand() *cobra.Command {
	opt := newOptions()
	cmd := &cobra.Command{
		Use:          "kubectl watchdiff [resource] [name]",
		Short:        "A kubectl plugin designed for monitoring diffs in resources.",
		Example:      example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.PrintFlags(cmd.Flags())
			verflag.PrintAndExitIfRequested()
			cfg, err := opt.Complete(args)
			if err != nil {
				return err
			}
			return cfg.Run(context.Background())
		},
	}

	flags.AddFlagsAndUsage(cmd, opt.Flags())
	return cmd
}

type options struct {
	Streams              genericclioptions.IOStreams
	ConfigFlags          *genericclioptions.ConfigFlags
	ResourceBuilderFlags *genericclioptions.ResourceBuilderFlags

	JSONPaths            []string
	IgnoreLabelKeys      []string
	IgnoreAnnotationKeys []string
}

func newOptions() *options {
	streams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	resourceBuilder := genericclioptions.NewResourceBuilderFlags().
		WithLabelSelector("").
		WithFieldSelector("").
		WithAll(true).
		WithAllNamespaces(false).
		WithLatest()
	// no need to identifying resources from file
	resourceBuilder.FileNameFlags = nil

	return &options{
		Streams:              streams,
		ConfigFlags:          genericclioptions.NewConfigFlags(true),
		ResourceBuilderFlags: resourceBuilder,
	}
}

func (o *options) Flags() *cliflag.NamedFlagSets {
	fss := &cliflag.NamedFlagSets{}

	fs := fss.FlagSet("options")
	fs.StringSliceVar(&o.JSONPaths, "jsonpaths", o.JSONPaths, "Select JSON path expressions to include in the output.")
	fs.StringSliceVar(&o.IgnoreLabelKeys, "ignore-label-keys", o.IgnoreLabelKeys, "if set, the specified labels will be ignored when comparing objects.")
	fs.StringSliceVar(&o.IgnoreAnnotationKeys, "ignore-annotation-keys", o.IgnoreLabelKeys, "if set, the specified annotations will be ignored when comparing objects. kubectl.kubernetes.io/last-applied-configuration will be always ignored")

	o.ResourceBuilderFlags.AddFlags(fss.FlagSet("resource"))
	o.ConfigFlags.AddFlags(fss.FlagSet("config"))

	return fss
}

func (o *options) Complete(args []string) (*config, error) {
	builder := flags.ToBuilder(o.ResourceBuilderFlags, o.ConfigFlags, args).SingleResourceType()

	jsonPaths := []jp.Expr{}

	for _, expr := range o.JSONPaths {
		p, err := jp.ParseString(expr)
		if err != nil {
			return nil, err
		}
		jsonPaths = append(jsonPaths, p)
	}

	// ignore kubectl applyed annotation key
	o.IgnoreAnnotationKeys = append(o.IgnoreAnnotationKeys, "kubectl.kubernetes.io/last-applied-configuration")

	return &config{
		IOStreams:            o.Streams,
		ResourceBuilder:      builder,
		JSONPaths:            jsonPaths,
		IngoreLabelKeys:      o.IgnoreLabelKeys,
		IgnoreAnnotationKeys: o.IgnoreAnnotationKeys,
	}, nil
}

type config struct {
	genericclioptions.IOStreams
	ResourceBuilder *resource.Builder

	JSONPaths            []jp.Expr
	IngoreLabelKeys      []string
	IgnoreAnnotationKeys []string
}

func (o *config) Run(ctx context.Context) error {
	r := o.ResourceBuilder.Do()
	if err := r.Err(); err != nil {
		return err
	}

	infos, err := r.Infos()
	if err != nil {
		return err
	}
	if multipleGVKsRequested(infos) {
		return i18n.Errorf("watch is only supported on individual resources and resource collections - more than 1 resource was found")
	}

	// set rv to "0" to get first ADDED event
	rv := "0"
	w, err := r.Watch(rv)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cache := make(map[string]*diffObj)

	intr := interrupt.New(nil, cancel)
	// nolint
	return intr.Run(func() error {
		_, err := watchtools.UntilWithoutRetry(ctx, w, func(event watch.Event) (bool, error) {
			obj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				return true, fmt.Errorf("event.Object is not unstructured.Unstructured")
			}

			if event.Type == watch.Error {
				// print error event
				fmt.Fprintln(o.ErrOut, obj)
				return true, nil
			}

			uid := string(obj.GetUID())
			rv := obj.GetResourceVersion()

			inCache, ok := cache[uid]
			if !ok {
				gvk := obj.GetObjectKind().GroupVersionKind()
				inCache = &diffObj{
					output:    o.Out,
					kind:      gvk.Kind,
					namespace: obj.GetNamespace(),
					name:      obj.GetName(),
					marshal: func(obj *unstructured.Unstructured) string {
						// delete managed field
						obj.SetManagedFields(nil)
						// delete resource version
						obj.SetResourceVersion("")

						// delete ignored label keys
						labels := obj.GetLabels()
						for _, key := range o.IngoreLabelKeys {
							delete(labels, key)
						}
						obj.SetLabels(labels)

						// delete ignored annotation keys
						annos := obj.GetAnnotations()
						for _, key := range o.IgnoreAnnotationKeys {
							delete(annos, key)
						}
						obj.SetAnnotations(annos)

						if len(o.JSONPaths) > 0 {
							newObj := &unstructured.Unstructured{
								Object: map[string]interface{}{},
							}
							for _, expr := range o.JSONPaths {
								result := expr.Get(obj.Object)
								if len(result) > 0 {
									key := fmt.Sprintf("<fieldByJsonPath = %s>", expr.String())
									newObj.Object[key] = result[0]
								}
							}
							obj = newObj
						}

						d, _ := yaml.Marshal(obj)
						return string(d)
					},
				}
				cache[uid] = inCache
				klog.InfoS("start watching diff of resource",
					"apiVersion", gvk.GroupVersion().String(),
					"kind", gvk.Kind,
					"namespace", inCache.namespace,
					"name", inCache.name,
					"rv", rv,
				)
			}

			inCache.diffWithPrevious(rv, obj)

			if event.Type == watch.Deleted {
				delete(cache, uid)
			}

			return false, nil
		})
		return err
	})
}

type diffObj struct {
	kind       string
	namespace  string
	name       string
	rv         string
	marshal    func(*unstructured.Unstructured) string
	output     io.Writer
	baseObject *unstructured.Unstructured
	baseYAML   string
}

func (o *diffObj) getNamePath(rv string) string {
	if o.namespace == "" {
		return path.Join(o.kind, o.name, rv)
	}
	return path.Join(o.kind, o.namespace, o.name, rv)
}

func (o *diffObj) diffWithPrevious(newRV string, newObj *unstructured.Unstructured) {
	newYaml := o.marshal(newObj)

	baseRV := o.rv
	baseYaml := o.baseYAML

	// ignore if the new coming obj is the same as previous one
	if o.rv == newRV {
		return
	}

	o.diff(baseRV, baseYaml, newRV, newYaml)
	o.baseObject = newObj
	o.baseYAML = newYaml
	o.rv = newRV
}

func (o diffObj) diff(rvBase, baseYaml, rvDiff, diffYaml string) {
	if len(rvBase) == 0 {
		rvBase = "0"
	}
	edits := myers.ComputeEdits(span.URIFromPath(""), baseYaml, diffYaml)
	baseName := o.getNamePath(rvBase)
	newName := o.getNamePath(rvDiff)

	unified := gotextdiff.ToUnified(baseName, newName, baseYaml, edits)
	diffStr := fmt.Sprint(unified)

	if len(diffStr) == 0 {
		return
	}

	lines := strings.Split(diffStr, "\n")
	// print color
	for _, line := range lines {
		if len(line) > 1 {
			if line[0] == '+' {
				// print green line in terminal
				line = fmt.Sprintf("\x1b[32m%s\x1b[0m", line)
			} else if line[0] == '-' {
				// print red line in terminal
				line = fmt.Sprintf("\x1b[31m%s\x1b[0m", line)
			}
		}
		fmt.Fprintln(o.output, line)
	}
}

func multipleGVKsRequested(infos []*resource.Info) bool {
	if len(infos) < 2 {
		return false
	}
	gvk := infos[0].Mapping.GroupVersionKind
	for _, info := range infos {
		if info.Mapping.GroupVersionKind != gvk {
			return true
		}
	}
	return false
}
