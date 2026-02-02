# Quickstart: Operator-Generated Server Configuration

**Feature Branch**: `002-operator-generated-config`
**Date**: 2026-02-02

---

## Overview

This guide shows how to implement the operator-generated server configuration feature. After completing these steps, users will be able to configure LlamaStack with minimal YAML.

---

## Prerequisites

- Go 1.25+
- kubebuilder v4+
- Kubernetes 1.21+ cluster
- Existing llama-stack-k8s-operator codebase

---

## Implementation Steps

### Step 1: Add CRD Types

Create `api/v1alpha1/provider_types.go`:

```go
package v1alpha1

import (
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/runtime"
)

// ProviderEntry defines a single provider configuration
type ProviderEntry struct {
    ID             string                `json:"id,omitempty"`
    Provider       string                `json:"provider"`
    Endpoint       string                `json:"endpoint,omitempty"`
    APIKey         *SecretKeyRefSource   `json:"apiKey,omitempty"`
    Config         *runtime.RawExtension `json:"config,omitempty"`
    Region         string                `json:"region,omitempty"`
    DeploymentName string                `json:"deploymentName,omitempty"`
}

// SecretKeyRefSource references a key in a Kubernetes Secret
type SecretKeyRefSource struct {
    SecretKeyRef corev1.SecretKeySelector `json:"secretKeyRef"`
}
```

### Step 2: Add Fields to ServerSpec

In `api/v1alpha1/llamastackdistribution_types.go`, add to `ServerSpec`:

```go
// Providers configures inference, safety, and other API providers
// +optional
Providers *ProvidersSpec `json:"providers,omitempty"`

// Disabled lists provider types to exclude
// +optional
Disabled []string `json:"disabled,omitempty"`

// Storage configures the persistence backend
// +optional
Storage *StorageConfigSpec `json:"storage,omitempty"`

// Resources registers models, tools, and shields
// +optional
Resources *ResourcesSpec `json:"resources,omitempty"`
```

### Step 3: Regenerate CRD Manifests

```bash
make generate manifests
```

### Step 4: Create Config Generator Package

Create `pkg/configgen/generator.go`:

```go
package configgen

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"

    llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
    corev1 "k8s.io/api/core/v1"
    "sigs.k8s.io/yaml"
)

type Generator struct {
    distributionConfigs map[string][]byte
}

func NewGenerator() *Generator {
    return &Generator{
        distributionConfigs: loadDistributionConfigs(),
    }
}

func (g *Generator) Generate(instance *llamav1alpha1.LlamaStackDistribution) ([]byte, []corev1.EnvVar, error) {
    // 1. Load distribution base config
    baseConfig, err := g.loadBaseConfig(instance.Spec.Server.Distribution.Name)
    if err != nil {
        return nil, nil, err
    }

    // 2. Apply provider overrides
    config, envVars := g.applyProviders(baseConfig, instance.Spec.Server.Providers)

    // 3. Apply disabled list
    config = g.removeDisabled(config, instance.Spec.Server.Disabled)

    // 4. Apply storage config
    config, storageEnvVars := g.applyStorage(config, instance.Spec.Server.Storage)
    envVars = append(envVars, storageEnvVars...)

    // 5. Apply resources
    config = g.applyResources(config, instance.Spec.Server.Resources)

    // 6. Serialize to YAML
    configYAML, err := yaml.Marshal(config)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to serialize config: %w", err)
    }

    return configYAML, envVars, nil
}

func (g *Generator) ConfigMapName(llsdName string, configContent []byte) string {
    hash := sha256.Sum256(configContent)
    shortHash := hex.EncodeToString(hash[:])[:10]
    return fmt.Sprintf("%s-config-%s", llsdName, shortHash)
}
```

### Step 5: Add Controller Reconciliation

In `controllers/llamastackdistribution_controller.go`, add:

```go
func (r *LlamaStackDistributionReconciler) reconcileGeneratedConfig(
    ctx context.Context,
    instance *llamav1alpha1.LlamaStackDistribution,
) error {
    log := log.FromContext(ctx)

    // Skip if using userConfig ConfigMap
    if instance.Spec.Server.UserConfig != nil &&
       instance.Spec.Server.UserConfig.ConfigMapName != "" {
        return nil
    }

    // Skip if no providers configured
    if instance.Spec.Server.Providers == nil {
        return nil
    }

    // Generate config
    generator := configgen.NewGenerator()
    configYAML, envVars, err := generator.Generate(instance)
    if err != nil {
        r.setCondition(instance, ConditionTypeConfigReady, metav1.ConditionFalse,
            ReasonConfigGenerationFailed, err.Error())
        return fmt.Errorf("failed to generate config: %w", err)
    }

    // Create/update ConfigMap
    cmName := generator.ConfigMapName(instance.Name, configYAML)
    cm := &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{
            Name:      cmName,
            Namespace: instance.Namespace,
        },
        Data: map[string]string{
            "config.yaml": string(configYAML),
        },
    }

    // Set owner reference
    if err := controllerutil.SetControllerReference(instance, cm, r.Scheme); err != nil {
        return fmt.Errorf("failed to set owner reference: %w", err)
    }

    // Apply ConfigMap
    if err := r.Client.Create(ctx, cm); err != nil {
        if !k8serrors.IsAlreadyExists(err) {
            return fmt.Errorf("failed to create ConfigMap: %w", err)
        }
    }

    // Update status
    instance.Status.GeneratedConfigMap = cmName
    r.setCondition(instance, ConditionTypeConfigReady, metav1.ConditionTrue,
        ReasonConfigGenerated, "Configuration generated successfully")

    log.Info("generated config", "configMap", cmName)
    return nil
}
```

### Step 6: Add Status Conditions

In `controllers/status.go`, add condition helpers:

```go
const (
    ConditionTypeValidationSucceeded = "ValidationSucceeded"
    ConditionTypeSecretsResolved     = "SecretsResolved"
    ConditionTypeConfigReady         = "ConfigReady"
)

func (r *LlamaStackDistributionReconciler) setConfigCondition(
    instance *llamav1alpha1.LlamaStackDistribution,
    conditionType string,
    status metav1.ConditionStatus,
    reason, message string,
) {
    condition := metav1.Condition{
        Type:               conditionType,
        Status:             status,
        ObservedGeneration: instance.Generation,
        LastTransitionTime: metav1.Now(),
        Reason:             reason,
        Message:            message,
    }
    meta.SetStatusCondition(&instance.Status.Conditions, condition)
}
```

### Step 7: Write Tests

Create `pkg/configgen/generator_test.go`:

```go
func TestGenerator_SingleProvider(t *testing.T) {
    gen := NewGenerator()

    instance := &llamav1alpha1.LlamaStackDistribution{
        ObjectMeta: metav1.ObjectMeta{Name: "test"},
        Spec: llamav1alpha1.LlamaStackDistributionSpec{
            Server: llamav1alpha1.ServerSpec{
                Distribution: llamav1alpha1.DistributionType{Name: "starter"},
                Providers: &llamav1alpha1.ProvidersSpec{
                    Inference: &llamav1alpha1.ProviderConfigOrList{
                        Single: &llamav1alpha1.ProviderEntry{
                            Provider: "vllm",
                            Endpoint: "http://vllm:8000",
                        },
                    },
                },
            },
        },
    }

    configYAML, envVars, err := gen.Generate(instance)
    require.NoError(t, err)
    require.NotEmpty(t, configYAML)

    // Verify provider mapping
    var config map[string]interface{}
    require.NoError(t, yaml.Unmarshal(configYAML, &config))

    providers := config["providers"].(map[string]interface{})
    inference := providers["inference"].([]interface{})
    require.Len(t, inference, 1)

    firstProvider := inference[0].(map[string]interface{})
    require.Equal(t, "remote::vllm", firstProvider["provider_type"])
}
```

---

## Example Usage

After implementation, users can deploy with minimal YAML:

```yaml
apiVersion: llama.meta.com/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-stack
spec:
  server:
    distribution:
      name: starter
    providers:
      inference:
        provider: vllm
        endpoint: "http://vllm:8000"
        apiKey:
          secretKeyRef:
            name: vllm-creds
            key: token
    resources:
      models:
        - "llama3.2-8b"
```

The operator generates and mounts a complete `config.yaml`.

---

## Verification Checklist

- [ ] `make generate manifests` succeeds
- [ ] `make test` passes
- [ ] CRD validation rejects invalid configs
- [ ] Single provider auto-generates ID
- [ ] Multiple providers require explicit IDs
- [ ] Secrets resolve to env vars
- [ ] ConfigMap has owner reference
- [ ] Status conditions update correctly
