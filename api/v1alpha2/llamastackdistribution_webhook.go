/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha2

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var llamastacklog = logf.Log.WithName("llamastackdistribution-webhook")

// LlamaStackDistributionValidator validates LlamaStackDistribution resources.
type LlamaStackDistributionValidator struct{}

var _ admission.CustomValidator = &LlamaStackDistributionValidator{}

// SetupWebhookWithManager registers the validating webhook.
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&LlamaStackDistribution{}).
		WithValidator(&LlamaStackDistributionValidator{}).
		Complete()
}

//+kubebuilder:webhook:path=/validate-llamastack-io-v1alpha2-llamastackdistribution,mutating=false,failurePolicy=fail,sideEffects=None,groups=llamastack.io,resources=llamastackdistributions,verbs=create;update,versions=v1alpha2,name=vllamastackdistribution.kb.io,admissionReviewVersions=v1

// ValidateCreate implements admission.CustomValidator.
func (v *LlamaStackDistributionValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	r, ok := obj.(*LlamaStackDistribution)
	if !ok {
		return nil, fmt.Errorf("expected *LlamaStackDistribution, got %T", obj)
	}
	llamastacklog.Info("validating create", "name", r.Name)
	return validate(r)
}

// ValidateUpdate implements admission.CustomValidator.
func (v *LlamaStackDistributionValidator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	r, ok := newObj.(*LlamaStackDistribution)
	if !ok {
		return nil, fmt.Errorf("expected *LlamaStackDistribution, got %T", newObj)
	}
	llamastacklog.Info("validating update", "name", r.Name)
	return validate(r)
}

// ValidateDelete implements admission.CustomValidator.
func (v *LlamaStackDistributionValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validate(r *LlamaStackDistribution) (admission.Warnings, error) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	if r.Spec.Providers != nil {
		if errs := validateProviderIDUniqueness(r.Spec.Providers); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	if r.Spec.Resources != nil && r.Spec.Providers != nil {
		if errs, warns := validateProviderReferences(r.Spec.Resources, r.Spec.Providers); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
			warnings = append(warnings, warns...)
		}
	}

	if len(r.Spec.Disabled) > 0 && r.Spec.Providers != nil {
		if warns := checkDisabledConflicts(r.Spec.Disabled, r.Spec.Providers); len(warns) > 0 {
			warnings = append(warnings, warns...)
		}
	}

	if len(allErrs) > 0 {
		return warnings, allErrs.ToAggregate()
	}

	return warnings, nil
}

func validateProviderIDUniqueness(spec *ProvidersSpec) field.ErrorList {
	var errs field.ErrorList
	seenIDs := make(map[string]string)

	fields := []struct {
		name string
		raw  []byte
	}{
		{"inference", jsonRawBytes(spec.Inference)},
		{"safety", jsonRawBytes(spec.Safety)},
		{"vectorIo", jsonRawBytes(spec.VectorIo)},
		{"toolRuntime", jsonRawBytes(spec.ToolRuntime)},
		{"telemetry", jsonRawBytes(spec.Telemetry)},
	}

	for _, f := range fields {
		if len(f.raw) == 0 {
			continue
		}

		ids := extractProviderIDs(f.raw)
		for _, id := range ids {
			if existingAPI, exists := seenIDs[id]; exists {
				errs = append(errs, field.Invalid(
					field.NewPath("spec", "providers", f.name),
					id,
					fmt.Sprintf("provider ID %q conflicts with provider in %q; all provider IDs must be unique across all API types", id, existingAPI),
				))
			}
			seenIDs[id] = f.name
		}
	}

	return errs
}

func validateProviderReferences(resources *ResourcesSpec, providers *ProvidersSpec) (field.ErrorList, admission.Warnings) {
	var errs field.ErrorList
	var warnings admission.Warnings

	providerIDs := collectAllProviderIDs(providers)

	for i, raw := range resources.Models {
		var mc ModelConfig
		if err := json.Unmarshal(raw.Raw, &mc); err != nil {
			continue
		}
		if mc.Provider != "" {
			if _, ok := providerIDs[mc.Provider]; !ok {
				errs = append(errs, field.Invalid(
					field.NewPath("spec", "resources", "models").Index(i).Child("provider"),
					mc.Provider,
					fmt.Sprintf("references unknown provider ID; available: %v", sortedMapKeys(providerIDs)),
				))
			}
		}
	}

	return errs, warnings
}

func checkDisabledConflicts(disabled []string, providers *ProvidersSpec) admission.Warnings {
	var warnings admission.Warnings

	apiFieldMap := map[string][]byte{
		"inference":    jsonRawBytes(providers.Inference),
		"safety":       jsonRawBytes(providers.Safety),
		"vector_io":    jsonRawBytes(providers.VectorIo),
		"tool_runtime": jsonRawBytes(providers.ToolRuntime),
		"telemetry":    jsonRawBytes(providers.Telemetry),
	}

	for _, api := range disabled {
		if raw, ok := apiFieldMap[api]; ok && len(raw) > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"API %q is disabled but has providers configured; disabled takes precedence and provider config will be ignored",
				api,
			))
		}
	}

	return warnings
}

func jsonRawBytes(raw interface{ MarshalJSON() ([]byte, error) }) []byte {
	if raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil || string(b) == "null" {
		return nil
	}
	return b
}

func extractProviderIDs(raw []byte) []string {
	var single struct {
		ID       string `json:"id"`
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(raw, &single); err == nil && single.Provider != "" {
		id := single.ID
		if id == "" {
			id = single.Provider
		}
		return []string{id}
	}

	var list []struct {
		ID       string `json:"id"`
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(raw, &list); err == nil {
		var ids []string
		for _, p := range list {
			id := p.ID
			if id == "" {
				id = p.Provider
			}
			ids = append(ids, id)
		}
		return ids
	}

	return nil
}

func collectAllProviderIDs(spec *ProvidersSpec) map[string]bool {
	ids := make(map[string]bool)
	for _, raw := range [][]byte{
		jsonRawBytes(spec.Inference),
		jsonRawBytes(spec.Safety),
		jsonRawBytes(spec.VectorIo),
		jsonRawBytes(spec.ToolRuntime),
		jsonRawBytes(spec.Telemetry),
	} {
		for _, id := range extractProviderIDs(raw) {
			ids[id] = true
		}
	}
	return ids
}

func sortedMapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
