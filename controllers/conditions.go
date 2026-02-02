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

// Config generation condition types.
const (
	// ConditionTypeValidationSucceeded indicates CRD schema validation passed.
	ConditionTypeValidationSucceeded = "ValidationSucceeded"

	// ConditionTypeSecretsResolved indicates all secretKeyRef references resolved.
	ConditionTypeSecretsResolved = "SecretsResolved"

	// ConditionTypeConfigReady indicates config.yaml generation succeeded.
	ConditionTypeConfigReady = "ConfigReady"
)

// Config generation condition reasons.
const (
	// ReasonValidationSucceeded indicates validation passed.
	ReasonValidationSucceeded = "ValidationSucceeded"
	// ReasonValidationFailed indicates validation failed.
	ReasonValidationFailed = "ValidationFailed"
	// ReasonValidationPending indicates validation is in progress.
	ReasonValidationPending = "ValidationPending"

	// ReasonSecretsResolved indicates all secrets were resolved.
	ReasonSecretsResolved = "SecretsResolved"
	// ReasonSecretNotFound indicates a referenced secret was not found.
	ReasonSecretNotFound = "SecretNotFound"
	// ReasonSecretKeyNotFound indicates a key was not found in a secret.
	ReasonSecretKeyNotFound = "SecretKeyNotFound"
	// ReasonSecretResolutionPending indicates secret resolution is in progress.
	ReasonSecretResolutionPending = "SecretResolutionPending"

	// ReasonConfigGenerated indicates config was successfully generated.
	ReasonConfigGenerated = "ConfigGenerated"
	// ReasonConfigGenerationFailed indicates config generation failed.
	ReasonConfigGenerationFailed = "ConfigGenerationFailed"
	// ReasonDistributionConfigNotFound indicates the distribution config was not found.
	ReasonDistributionConfigNotFound = "DistributionConfigNotFound"
	// ReasonConfigMapCreationFailed indicates ConfigMap creation failed.
	ReasonConfigMapCreationFailed = "ConfigMapCreationFailed"
	// ReasonConfigGenerationPending indicates config generation is in progress.
	ReasonConfigGenerationPending = "ConfigGenerationPending"
)

// Config generation condition messages.
const (
	// MessageValidationSucceeded indicates validation passed.
	MessageValidationSucceeded = "CRD schema validation passed"
	// MessageSecretsResolved indicates all secrets were resolved.
	MessageSecretsResolved = "All secret references resolved successfully"
	// MessageConfigGenerated indicates config was successfully generated.
	MessageConfigGenerated = "Configuration generated successfully"
)
