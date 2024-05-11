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

package flags

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
)

// ToBuilder gives you back a resource finder to visit resources that are located
func ToBuilder(o *genericclioptions.ResourceBuilderFlags, configFlags *genericclioptions.ConfigFlags, resources []string) *resource.Builder {
	namespace, enforceNamespace, namespaceErr := configFlags.ToRawKubeConfigLoader().Namespace()

	builder := resource.NewBuilder(configFlags).
		NamespaceParam(namespace).DefaultNamespace()

	if o.AllNamespaces != nil {
		builder.AllNamespaces(*o.AllNamespaces)
	}

	if o.Scheme != nil {
		builder.WithScheme(o.Scheme, o.Scheme.PrioritizedVersionsAllGroups()...)
	} else {
		builder.Unstructured()
	}

	if o.FileNameFlags != nil {
		opts := o.FileNameFlags.ToOptions()
		builder.FilenameParam(enforceNamespace, &opts)
	}

	if o.Local == nil || !*o.Local {
		// resource type/name tuples only work non-local
		if o.All != nil {
			builder.ResourceTypeOrNameArgs(*o.All, resources...)
		} else {
			builder.ResourceTypeOrNameArgs(false, resources...)
		}
		// label selectors only work non-local (for now)
		if o.LabelSelector != nil {
			builder.LabelSelectorParam(*o.LabelSelector)
		}
		// field selectors only work non-local (forever)
		if o.FieldSelector != nil {
			builder.FieldSelectorParam(*o.FieldSelector)
		}
		// latest only works non-local (forever)
		if o.Latest {
			builder.Latest()
		}

	} else {
		builder.Local()

		if len(resources) > 0 {
			builder.AddError(resource.LocalResourceError)
		}
	}

	if !o.StopOnFirstError {
		builder.ContinueOnError()
	}

	return builder.
		Flatten(). // I think we're going to recommend this everywhere
		AddError(namespaceErr)
}
