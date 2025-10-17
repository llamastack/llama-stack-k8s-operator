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

package controllers

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Constants for validation limits.
const (
	// maxConfigMapKeyLength defines the maximum allowed length for ConfigMap keys
	// based on Kubernetes DNS subdomain name limits.
	maxConfigMapKeyLength = 253
)

// Probes configuration.
const (
	startupProbeInitialDelaySeconds = 15 // Time to wait before the first probe
	startupProbeTimeoutSeconds      = 30 // When the probe times out
	startupProbeFailureThreshold    = 3  // Pod is marked Unhealthy after 3 consecutive failures
	startupProbeSuccessThreshold    = 1  // Pod is marked Ready after 1 successful probe
)

// validConfigMapKeyRegex defines allowed characters for ConfigMap keys.
// Kubernetes ConfigMap keys must be valid DNS subdomain names or data keys.
var validConfigMapKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-_.]*[a-zA-Z0-9])?$`)

// startupScript is the script that will be used to start the server.
var startupScript = `
set -euo pipefail

# Create unique temporary file for certificate counting using mktemp for atomicity
cert_count_file=$(mktemp /tmp/cert_count_XXXXXX.txt)

# Cleanup function for error handling
cleanup() {
    exit_code=$?
    if [ $exit_code -ne 0 ]; then
        echo "Error occurred (exit code: $exit_code), cleaning up partial certificate processing..." >&2
        rm -rf /tmp/ca-bundle-processed
    fi
    # Always clean up the temp file
    rm -f "$cert_count_file"
}
trap cleanup EXIT

# Maximum number of certificates to prevent resource exhaustion
readonly MAX_CERTS=1000

# Process CA bundle certificates if they exist
# Check both explicit CA bundle directory and ODH CA bundle directory
if [ -d "/etc/ssl/certs/ca-certificates" ] || [ -d "/etc/ssl/certs/ca-certificates/odh" ]; then
    if [ "$(ls -A /etc/ssl/certs/ca-certificates 2>/dev/null)" ] || [ "$(ls -A /etc/ssl/certs/ca-certificates/odh 2>/dev/null)" ]; then
        echo "Processing CA bundle certificates..."

        # Create the processed directory
        mkdir -p /tmp/ca-bundle-processed

        # Counter for split certificates
        cert_counter=0

        # Process explicit CA bundle files first
        if [ -d "/etc/ssl/certs/ca-certificates" ]; then
            # Use process substitution to avoid subshell variable isolation
            # -L flag follows symlinks (required because Kubernetes mounts ConfigMaps as symlinks)
            while IFS= read -r -d '' cert_file; do
                echo "Processing explicit CA bundle file: $cert_file"

                # Validate that file contains PEM certificates before processing
                if ! grep -q "BEGIN CERTIFICATE" "$cert_file"; then
                    echo "Warning: $cert_file does not contain valid PEM certificates, skipping" >&2
                    continue
                fi

                # Split multi-certificate PEM files into individual certificates
                # AWK script includes resource exhaustion protection
                awk -v output_dir="/tmp/ca-bundle-processed" -v base_counter="$cert_counter" -v max_certs="$MAX_CERTS" '
                    BEGIN {
                        counter = base_counter;
                        in_cert = 0;
                    }
                    /-----BEGIN CERTIFICATE-----/ {
                        if (counter >= max_certs) {
                            print "ERROR: Certificate limit exceeded (max " max_certs "). Possible malformed input or resource exhaustion attack." > "/dev/stderr";
                            exit 1;
                        }
                        in_cert = 1;
                        counter++;
                        filename = sprintf("%s/cert-%d.pem", output_dir, counter);
                    }
                    in_cert {
                        print > filename;
                    }
                    /-----END CERTIFICATE-----/ {
                        in_cert = 0;
                        close(filename);
                    }
                    END {
                        print counter;
                    }
                ' "$cert_file" > "$cert_count_file"

                # Update counter
                cert_counter=$(cat "$cert_count_file")
            done < <(find -L /etc/ssl/certs/ca-certificates -maxdepth 1 -type f -print0)
        fi

        # Process ODH CA bundle files
        if [ -d "/etc/ssl/certs/ca-certificates/odh" ]; then
            # Use process substitution to avoid subshell variable isolation
            # -L flag follows symlinks (required because Kubernetes mounts ConfigMaps as symlinks)
            while IFS= read -r -d '' cert_file; do
                echo "Processing ODH CA bundle file: $cert_file"

                # Validate that file contains PEM certificates before processing
                if ! grep -q "BEGIN CERTIFICATE" "$cert_file"; then
                    echo "Warning: $cert_file does not contain valid PEM certificates, skipping" >&2
                    continue
                fi

                # Split multi-certificate PEM files into individual certificates
                # AWK script includes resource exhaustion protection
                awk -v output_dir="/tmp/ca-bundle-processed" -v base_counter="$cert_counter" -v max_certs="$MAX_CERTS" '
                    BEGIN {
                        counter = base_counter;
                        in_cert = 0;
                    }
                    /-----BEGIN CERTIFICATE-----/ {
                        if (counter >= max_certs) {
                            print "ERROR: Certificate limit exceeded (max " max_certs "). Possible malformed input or resource exhaustion attack." > "/dev/stderr";
                            exit 1;
                        }
                        in_cert = 1;
                        counter++;
                        filename = sprintf("%s/cert-%d.pem", output_dir, counter);
                    }
                    in_cert {
                        print > filename;
                    }
                    /-----END CERTIFICATE-----/ {
                        in_cert = 0;
                        close(filename);
                    }
                    END {
                        print counter;
                    }
                ' "$cert_file" > "$cert_count_file"

                # Update counter
                cert_counter=$(cat "$cert_count_file")
            done < <(find -L /etc/ssl/certs/ca-certificates/odh -maxdepth 1 -type f -print0)
        fi

        # Run c_rehash to create hash symlinks for OpenSSL
        # Fail explicitly if rehashing fails as this breaks certificate validation
        if command -v c_rehash >/dev/null 2>&1; then
            if ! c_rehash /tmp/ca-bundle-processed; then
                echo "ERROR: c_rehash failed to process certificates" >&2
                exit 1
            fi
        elif command -v openssl >/dev/null 2>&1; then
            if ! openssl rehash /tmp/ca-bundle-processed; then
                echo "ERROR: openssl rehash failed to process certificates" >&2
                exit 1
            fi
        else
            echo "ERROR: Neither c_rehash nor openssl rehash available - cannot process certificates" >&2
            exit 1
        fi

        echo "CA bundle processing complete: processed $cert_counter certificate(s)"
    else
        echo "No CA bundle certificates found, skipping processing"
    fi
else
    echo "No CA bundle directories found, skipping processing"
fi

# Determine which CLI to use based on llama-stack version
VERSION_CODE=$(python -c "
import sys
from importlib.metadata import version
from packaging import version as pkg_version

try:
    llama_version = version('llama_stack')
    print(f'Detected llama-stack version: {llama_version}', file=sys.stderr)

    v = pkg_version.parse(llama_version)

    if v < pkg_version.parse('0.2.17'):
        print('Using legacy module path (llama_stack.distribution.server.server)', file=sys.stderr)
        print(0)
    elif v < pkg_version.parse('0.3.0'):
        print('Using core module path (llama_stack.core.server.server)', file=sys.stderr)
        print(1)
    else:
        print('Using new CLI command (llama stack run)', file=sys.stderr)
        print(2)
except Exception as e:
    print(f'Version detection failed, defaulting to new CLI: {e}', file=sys.stderr)
    print(2)
")

# Execute the appropriate CLI based on version
# Check if custom config exists, otherwise use container default behavior
if [ -f "/etc/llama-stack/run.yaml" ]; then
    echo "Using custom config file: /etc/llama-stack/run.yaml"
    case $VERSION_CODE in
        0) exec python3 -m llama_stack.distribution.server.server --config /etc/llama-stack/run.yaml ;;
        1) exec python3 -m llama_stack.core.server.server /etc/llama-stack/run.yaml ;;
        2) exec llama stack run /etc/llama-stack/run.yaml ;;
        *) echo "Invalid version code: $VERSION_CODE, using new CLI"; exec llama stack run /etc/llama-stack/run.yaml ;;
    esac
else
    echo "No custom config file found, using container default entrypoint with distribution name"
    # Use the container's default behavior - pass distribution name as argument
    case $VERSION_CODE in
        0) exec python3 -m llama_stack.distribution.server.server starter ;;
        1) exec python3 -m llama_stack.core.server.server starter ;;
        2) exec llama stack run starter ;;
        *) echo "Invalid version code: $VERSION_CODE, using new CLI"; exec llama stack run starter ;;
    esac
fi`

// validateConfigMapKeys validates that all ConfigMap keys contain only safe characters.
// Note: This function validates key names only. PEM content validation is performed
// separately in the controller's reconcileCABundleConfigMap function.
func validateConfigMapKeys(keys []string) error {
	for _, key := range keys {
		if key == "" {
			return errors.New("ConfigMap key cannot be empty")
		}
		if len(key) > maxConfigMapKeyLength {
			return fmt.Errorf("failed to validate ConfigMap key '%s': too long (max %d characters)", key, maxConfigMapKeyLength)
		}
		if !validConfigMapKeyRegex.MatchString(key) {
			return fmt.Errorf("failed to validate ConfigMap key '%s': contains invalid characters. Only alphanumeric characters, hyphens, underscores, and dots are allowed", key)
		}
		// Additional security check: prevent path traversal attempts
		if strings.Contains(key, "..") || strings.Contains(key, "/") {
			return fmt.Errorf("failed to validate ConfigMap key '%s': contains invalid path characters", key)
		}
	}
	return nil
}

// getHealthProbe returns the health probe handler for the container.
func getHealthProbe(instance *llamav1alpha1.LlamaStackDistribution) corev1.ProbeHandler {
	return corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/v1/health",
			Port: intstr.FromInt(int(getContainerPort(instance))),
		},
	}
}

// getStartupProbe returns the startup probe for the container.
func getStartupProbe(instance *llamav1alpha1.LlamaStackDistribution) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        getHealthProbe(instance),
		InitialDelaySeconds: startupProbeInitialDelaySeconds,
		TimeoutSeconds:      startupProbeTimeoutSeconds,
		FailureThreshold:    startupProbeFailureThreshold,
		SuccessThreshold:    startupProbeSuccessThreshold,
	}
}

// buildContainerSpec creates the container specification.
func buildContainerSpec(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, image string) corev1.Container {
	container := corev1.Container{
		Name:         getContainerName(instance),
		Image:        image,
		Resources:    instance.Spec.Server.ContainerSpec.Resources,
		Ports:        []corev1.ContainerPort{{ContainerPort: getContainerPort(instance)}},
		StartupProbe: getStartupProbe(instance),
	}

	// Configure environment variables and mounts
	configureContainerEnvironment(ctx, r, instance, &container)
	configureContainerMounts(ctx, r, instance, &container)
	configureContainerCommands(ctx, r, instance, &container)

	return container
}

// getContainerName returns the container name, using custom name if specified.
func getContainerName(instance *llamav1alpha1.LlamaStackDistribution) string {
	if instance.Spec.Server.ContainerSpec.Name != "" {
		return instance.Spec.Server.ContainerSpec.Name
	}
	return llamav1alpha1.DefaultContainerName
}

// getContainerPort returns the container port, using custom port if specified.
func getContainerPort(instance *llamav1alpha1.LlamaStackDistribution) int32 {
	if instance.Spec.Server.ContainerSpec.Port != 0 {
		return instance.Spec.Server.ContainerSpec.Port
	}
	return llamav1alpha1.DefaultServerPort
}

// configureContainerEnvironment sets up environment variables for the container.
func configureContainerEnvironment(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	mountPath := getMountPath(instance)

	// Add HF_HOME variable to our mount path so that downloaded models and datasets are stored
	// on the same volume as the storage. This is not critical but useful if the server is
	// restarted so the models and datasets are not lost and need to be downloaded again.
	// For more information, see https://huggingface.co/docs/datasets/en/cache
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "HF_HOME",
		Value: mountPath,
	})

	// Add CA bundle environment variable if any CA bundles are configured
	// (explicit or auto-detected ODH bundles)
	if hasAnyCABundle(ctx, r, instance) {
		// Set SSL_CERT_DIR to point to the processed CA bundle directory
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "SSL_CERT_DIR",
			Value: CABundleProcessedDir,
		})
	}

	// Finally, add the user provided env vars
	container.Env = append(container.Env, instance.Spec.Server.ContainerSpec.Env...)
}

// configureContainerMounts sets up volume mounts for the container.
func configureContainerMounts(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	// Add volume mount for storage
	addStorageVolumeMount(instance, container)

	// Add ConfigMap volume mount if user config is specified
	addUserConfigVolumeMount(instance, container)

	// Add CA bundle volume mount if TLS config is specified or auto-detected
	addCABundleVolumeMount(ctx, r, instance, container)
}

// hasAnyCABundle checks if any CA bundle will be mounted (explicit or auto-detected).
func hasAnyCABundle(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution) bool {
	// Check for explicit CA bundle configuration
	if instance.Spec.Server.TLSConfig != nil && instance.Spec.Server.TLSConfig.CABundle != nil {
		return true
	}

	// Check for auto-detected ODH trusted CA bundle
	if r != nil {
		if _, keys, err := r.detectODHTrustedCABundle(ctx, instance); err == nil && len(keys) > 0 {
			return true
		}
	}

	return false
}

// configureContainerCommands sets up container commands and args.
func configureContainerCommands(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	// Use startup script if user config is specified OR if any CA bundle is configured (explicit or auto-detected)
	hasUserConfig := instance.Spec.Server.UserConfig != nil && instance.Spec.Server.UserConfig.ConfigMapName != ""

	if hasUserConfig || hasAnyCABundle(ctx, r, instance) {
		// Override the container entrypoint to use the startup script
		// The script will:
		// 1. Process CA bundle certificates if they exist (explicit or auto-detected ODH bundles)
		// 2. Determine the llama-stack version and use the appropriate module path to start the server
		// Use /bin/bash explicitly as the script uses bash-specific features (read -d)
		container.Command = []string{"/bin/bash", "-c", startupScript}
		container.Args = []string{}
	}

	// Apply user-specified command and args (takes precedence)
	if len(instance.Spec.Server.ContainerSpec.Command) > 0 {
		container.Command = instance.Spec.Server.ContainerSpec.Command
	}

	if len(instance.Spec.Server.ContainerSpec.Args) > 0 {
		container.Args = instance.Spec.Server.ContainerSpec.Args
	}
}

// getMountPath returns the mount path, using custom path if specified.
func getMountPath(instance *llamav1alpha1.LlamaStackDistribution) string {
	if instance.Spec.Server.Storage != nil && instance.Spec.Server.Storage.MountPath != "" {
		return instance.Spec.Server.Storage.MountPath
	}
	return llamav1alpha1.DefaultMountPath
}

// addStorageVolumeMount adds the storage volume mount to the container.
func addStorageVolumeMount(instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	mountPath := getMountPath(instance)
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      "lls-storage",
		MountPath: mountPath,
	})
}

// addUserConfigVolumeMount adds the user config volume mount to the container if specified.
func addUserConfigVolumeMount(instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	if instance.Spec.Server.UserConfig != nil && instance.Spec.Server.UserConfig.ConfigMapName != "" {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "user-config",
			MountPath: "/etc/llama-stack/",
			ReadOnly:  true,
		})
	}
}

// addCABundleVolumeMount adds the CA bundle volume mount to the container if TLS config is specified.
// Mounts the ConfigMap directly to a directory where the startup script can process the certificates.
// Also handles auto-detected ODH trusted CA bundle ConfigMaps.
// Both explicit and ODH bundles can be mounted together (additive).
func addCABundleVolumeMount(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	// Mount explicit CA bundle if configured
	if instance.Spec.Server.TLSConfig != nil && instance.Spec.Server.TLSConfig.CABundle != nil {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      CABundleVolumeName,
			MountPath: CABundleSourceMountDir,
			ReadOnly:  true,
		})
	}

	// Also mount ODH trusted CA bundle if it exists (additive to explicit config)
	if r != nil {
		if _, keys, err := r.detectODHTrustedCABundle(ctx, instance); err == nil && len(keys) > 0 {
			container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
				Name:      "odh-ca-bundle",
				MountPath: CABundleSourceMountDir + "/odh",
				ReadOnly:  true,
			})
		}
	}
}

// createCABundleVolume creates the volume configuration for CA bundles.
// Mounts the ConfigMap directly with all specified keys.
func createCABundleVolume(caBundleConfig *llamav1alpha1.CABundleConfig) corev1.Volume {
	volume := corev1.Volume{
		Name: CABundleVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: caBundleConfig.ConfigMapName,
				},
			},
		},
	}

	// If specific keys are specified, create items to map them
	if len(caBundleConfig.ConfigMapKeys) > 0 {
		items := make([]corev1.KeyToPath, 0, len(caBundleConfig.ConfigMapKeys))
		for _, key := range caBundleConfig.ConfigMapKeys {
			items = append(items, corev1.KeyToPath{
				Key:  key,
				Path: key, // Mount with the same filename
			})
		}
		volume.VolumeSource.ConfigMap.Items = items
	}
	// If no keys specified, all keys in the ConfigMap will be mounted

	return volume
}

// configurePodStorage configures the pod storage and returns the complete pod spec.
func configurePodStorage(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, container corev1.Container) corev1.PodSpec {
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{container},
	}

	// Configure storage volumes and init containers
	configureStorage(instance, &podSpec)

	// Configure TLS CA bundle (with auto-detection support)
	configureTLSCABundle(ctx, r, instance, &podSpec)

	// Configure user config
	configureUserConfig(instance, &podSpec)

	// Apply pod overrides including ServiceAccount, volumes, and volume mounts
	configurePodOverrides(instance, &podSpec)

	return podSpec
}

// configureStorage handles storage volume configuration.
func configureStorage(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	if instance.Spec.Server.Storage != nil {
		configurePersistentStorage(instance, podSpec)
	} else {
		configureEmptyDirStorage(podSpec)
	}
}

// configurePersistentStorage sets up PVC-based storage with init container for permissions.
func configurePersistentStorage(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	// Use PVC for persistent storage
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "lls-storage",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: instance.Name + "-pvc",
			},
		},
	})
}

// configureEmptyDirStorage sets up temporary storage using emptyDir.
func configureEmptyDirStorage(podSpec *corev1.PodSpec) {
	// Use emptyDir for non-persistent storage
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "lls-storage",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
}

// configureTLSCABundle handles TLS CA bundle configuration.
// Supports both explicit CA bundles and auto-detected ODH bundles.
// Both can be configured simultaneously (additive).
func configureTLSCABundle(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	tlsConfig := instance.Spec.Server.TLSConfig

	// Handle explicit CA bundle configuration
	if tlsConfig != nil && tlsConfig.CABundle != nil {
		addExplicitCABundle(tlsConfig.CABundle, podSpec)
	}

	// Also check for ODH trusted CA bundle auto-detection (additive to explicit config)
	if r != nil {
		addAutoDetectedCABundle(ctx, r, instance, podSpec)
	}
}

// addExplicitCABundle handles explicitly configured CA bundles.
func addExplicitCABundle(caBundleConfig *llamav1alpha1.CABundleConfig, podSpec *corev1.PodSpec) {
	// Add CA bundle ConfigMap volume - mounts directly to the main container
	volume := createCABundleVolume(caBundleConfig)
	podSpec.Volumes = append(podSpec.Volumes, volume)
}

// addAutoDetectedCABundle handles auto-detection of ODH trusted CA bundle ConfigMap.
// This is additive to any explicit CA bundle configuration.
func addAutoDetectedCABundle(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	if r == nil {
		return
	}

	configMap, keys, err := r.detectODHTrustedCABundle(ctx, instance)
	if err != nil {
		// Log error but don't fail the reconciliation
		log.FromContext(ctx).Error(err, "Failed to detect ODH trusted CA bundle ConfigMap")
		return
	}

	if configMap == nil || len(keys) == 0 {
		// No ODH trusted CA bundle found or no keys available
		return
	}

	// Create ODH CA bundle volume with separate name to avoid conflicts with explicit CA bundle
	volume := corev1.Volume{
		Name: "odh-ca-bundle",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMap.Name,
				},
			},
		},
	}

	// Mount all available keys from the ODH bundle
	if len(keys) > 0 {
		items := make([]corev1.KeyToPath, 0, len(keys))
		for _, key := range keys {
			items = append(items, corev1.KeyToPath{
				Key:  key,
				Path: key, // Mount with the same filename
			})
		}
		volume.VolumeSource.ConfigMap.Items = items
	}

	podSpec.Volumes = append(podSpec.Volumes, volume)

	log.FromContext(ctx).Info("Auto-configured ODH trusted CA bundle",
		"configMapName", configMap.Name,
		"keys", keys)
}

// configureUserConfig handles user configuration setup.
func configureUserConfig(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	userConfig := instance.Spec.Server.UserConfig
	if userConfig == nil || userConfig.ConfigMapName == "" {
		return
	}

	// Add ConfigMap volume if user config is specified
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "user-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: userConfig.ConfigMapName,
				},
			},
		},
	})
}

// configurePodOverrides applies pod-level overrides from the LlamaStackDistribution spec.
func configurePodOverrides(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	// Set ServiceAccount name - use override if specified, otherwise use default
	if instance.Spec.Server.PodOverrides != nil && instance.Spec.Server.PodOverrides.ServiceAccountName != "" {
		podSpec.ServiceAccountName = instance.Spec.Server.PodOverrides.ServiceAccountName
	} else {
		podSpec.ServiceAccountName = instance.Name + "-sa"
	}

	// Configure pod-level security context for OpenShift SCC compatibility
	if podSpec.SecurityContext == nil {
		podSpec.SecurityContext = &corev1.PodSecurityContext{}
	}

	// Set fsGroup to allow write access to mounted volumes
	const defaultFSGroup = 1001
	if podSpec.SecurityContext.FSGroup == nil {
		podSpec.SecurityContext.FSGroup = ptr.To(int64(defaultFSGroup))
	}

	// Apply other pod overrides if specified
	if instance.Spec.Server.PodOverrides != nil {
		// Add volumes if specified
		if len(instance.Spec.Server.PodOverrides.Volumes) > 0 {
			podSpec.Volumes = append(podSpec.Volumes, instance.Spec.Server.PodOverrides.Volumes...)
		}

		// Add volume mounts if specified
		if len(instance.Spec.Server.PodOverrides.VolumeMounts) > 0 {
			if len(podSpec.Containers) > 0 {
				podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, instance.Spec.Server.PodOverrides.VolumeMounts...)
			}
		}
	}
}

// validateDistribution validates the distribution configuration.
func (r *LlamaStackDistributionReconciler) validateDistribution(instance *llamav1alpha1.LlamaStackDistribution) error {
	// If using distribution name, validate it exists in clusterInfo
	if instance.Spec.Server.Distribution.Name != "" {
		if r.ClusterInfo == nil {
			return errors.New("failed to initialize cluster info")
		}
		if _, exists := r.ClusterInfo.DistributionImages[instance.Spec.Server.Distribution.Name]; !exists {
			return fmt.Errorf("failed to validate distribution: %s. Distribution name not supported", instance.Spec.Server.Distribution.Name)
		}
	}

	return nil
}

// resolveImage determines the container image to use based on the distribution configuration.
// It returns the resolved image and any error encountered.
func (r *LlamaStackDistributionReconciler) resolveImage(distribution llamav1alpha1.DistributionType) (string, error) {
	distributionMap := r.ClusterInfo.DistributionImages
	switch {
	case distribution.Name != "":
		if _, exists := distributionMap[distribution.Name]; !exists {
			return "", fmt.Errorf("failed to validate distribution name: %s", distribution.Name)
		}
		return distributionMap[distribution.Name], nil
	case distribution.Image != "":
		return distribution.Image, nil
	default:
		return "", errors.New("failed to validate distribution: either distribution.name or distribution.image must be set")
	}
}
