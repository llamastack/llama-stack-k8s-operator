package controllers

import (
	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition types.
const (
	// ConditionTypeDeploymentReady indicates whether the deployment is ready.
	ConditionTypeDeploymentReady = "DeploymentReady"
	// ConditionTypeHealthCheck indicates whether the health check passed.
	ConditionTypeHealthCheck = "HealthCheck"
	// ConditionTypeStorageReady indicates whether the storage is ready.
	ConditionTypeStorageReady = "StorageReady"
	// ConditionTypeServiceReady indicates whether the service is ready.
	ConditionTypeServiceReady = "ServiceReady"
)

// Condition reasons.
const (
	// ReasonDeploymentReady indicates the deployment is ready.
	ReasonDeploymentReady = "DeploymentReady"
	// ReasonDeploymentFailed indicates the deployment failed.
	ReasonDeploymentFailed = "DeploymentFailed"
	// ReasonDeploymentPending indicates the deployment is pending.
	ReasonDeploymentPending = "DeploymentPending"
	// ReasonHealthCheckPassed indicates the health check passed.
	ReasonHealthCheckPassed = "HealthCheckPassed"
	// ReasonHealthCheckFailed indicates the health check failed.
	ReasonHealthCheckFailed = "HealthCheckFailed"
	// ReasonStorageReady indicates the storage is ready.
	ReasonStorageReady = "StorageReady"
	// ReasonStorageFailed indicates the storage failed.
	ReasonStorageFailed = "StorageFailed"
	// ReasonServiceReady indicates the service is ready.
	ReasonServiceReady = "ServiceReady"
	// ReasonServiceFailed indicates the service failed.
	ReasonServiceFailed = "ServiceFailed"
)

// Condition messages.
const (
	// MessageDeploymentReady indicates the deployment is ready.
	MessageDeploymentReady = "Deployment is ready"
	// MessageDeploymentFailed indicates the deployment failed.
	MessageDeploymentFailed = "Deployment failed"
	// MessageDeploymentPending indicates the deployment is pending.
	MessageDeploymentPending = "Deployment is pending"
	// MessageHealthCheckPassed indicates the health check passed.
	MessageHealthCheckPassed = "Health check passed"
	// MessageHealthCheckFailed indicates the health check failed.
	MessageHealthCheckFailed = "Health check failed"
	// MessageStorageReady indicates the storage is ready.
	MessageStorageReady = "Storage is ready"
	// MessageStorageFailed indicates the storage failed.
	MessageStorageFailed = "Storage failed"
	// MessageServiceReady indicates the service is ready.
	MessageServiceReady = "Service is ready"
	// MessageServiceFailed indicates the service failed.
	MessageServiceFailed = "Service failed"
)

// SetDeploymentReadyCondition sets the deployment ready condition.
func SetDeploymentReadyCondition(status *llamav1alpha1.LlamaStackDistributionStatus, ready bool, message string) {
	condition := metav1.Condition{
		Type:               ConditionTypeDeploymentReady,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonDeploymentReady,
		Message:            MessageDeploymentReady,
		LastTransitionTime: metav1.NewTime(metav1.Now().UTC()),
	}

	if !ready {
		condition.Status = metav1.ConditionFalse
		condition.Reason = ReasonDeploymentFailed
		condition.Message = message
	}

	SetCondition(status, condition)
}

// SetHealthCheckCondition sets the health check condition.
func SetHealthCheckCondition(status *llamav1alpha1.LlamaStackDistributionStatus, healthy bool, message string) {
	condition := metav1.Condition{
		Type:               ConditionTypeHealthCheck,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonHealthCheckPassed,
		Message:            MessageHealthCheckPassed,
		LastTransitionTime: metav1.NewTime(metav1.Now().UTC()),
	}

	if !healthy {
		condition.Status = metav1.ConditionFalse
		condition.Reason = ReasonHealthCheckFailed
		condition.Message = message
	}

	SetCondition(status, condition)
}

// SetStorageReadyCondition sets the storage ready condition.
func SetStorageReadyCondition(status *llamav1alpha1.LlamaStackDistributionStatus, ready bool, message string) {
	condition := metav1.Condition{
		Type:               ConditionTypeStorageReady,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonStorageReady,
		Message:            MessageStorageReady,
		LastTransitionTime: metav1.NewTime(metav1.Now().UTC()),
	}

	if !ready {
		condition.Status = metav1.ConditionFalse
		condition.Reason = ReasonStorageFailed
		condition.Message = message
	}

	SetCondition(status, condition)
}

// SetServiceReadyCondition sets the service ready condition.
func SetServiceReadyCondition(status *llamav1alpha1.LlamaStackDistributionStatus, ready bool, message string) {
	condition := metav1.Condition{
		Type:               ConditionTypeServiceReady,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonServiceReady,
		Message:            MessageServiceReady,
		LastTransitionTime: metav1.NewTime(metav1.Now().UTC()),
	}

	if !ready {
		condition.Status = metav1.ConditionFalse
		condition.Reason = ReasonServiceFailed
		condition.Message = message
	}

	SetCondition(status, condition)
}

// SetCondition sets a condition in the status.
func SetCondition(status *llamav1alpha1.LlamaStackDistributionStatus, condition metav1.Condition) {
	// Initialize conditions if needed
	if status.Conditions == nil {
		status.Conditions = make([]metav1.Condition, 0)
	}

	// Find existing condition
	for i := range status.Conditions {
		if status.Conditions[i].Type == condition.Type {
			// Update existing condition
			status.Conditions[i] = condition
			return
		}
	}

	// Add new condition
	status.Conditions = append(status.Conditions, condition)
}

// GetCondition returns a condition by type.
func GetCondition(status *llamav1alpha1.LlamaStackDistributionStatus, conditionType string) *metav1.Condition {
	if status == nil || status.Conditions == nil {
		return nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return &status.Conditions[i]
		}
	}
	return nil
}

// IsConditionTrue returns true if the condition is true.
func IsConditionTrue(status *llamav1alpha1.LlamaStackDistributionStatus, conditionType string) bool {
	condition := GetCondition(status, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// IsConditionFalse returns true if the condition is false.
func IsConditionFalse(status *llamav1alpha1.LlamaStackDistributionStatus, conditionType string) bool {
	condition := GetCondition(status, conditionType)
	return condition != nil && condition.Status == metav1.ConditionFalse
}

// InitializeExternalProviderStatus initializes status tracking for all configured external providers.
// This should be called when a LlamaStackDistribution with external providers is first reconciled.
func InitializeExternalProviderStatus(status *llamav1alpha1.LlamaStackDistributionStatus, ep *llamav1alpha1.ExternalProvidersSpec) {
	if ep == nil {
		return
	}

	var providerStatuses []llamav1alpha1.ExternalProviderStatus

	// Collect all providers maintaining CRD order
	providerStatuses = appendProviderStatuses(providerStatuses, ep.Inference)
	providerStatuses = appendProviderStatuses(providerStatuses, ep.Safety)
	providerStatuses = appendProviderStatuses(providerStatuses, ep.Agents)
	providerStatuses = appendProviderStatuses(providerStatuses, ep.VectorIO)
	providerStatuses = appendProviderStatuses(providerStatuses, ep.DatasetIO)
	providerStatuses = appendProviderStatuses(providerStatuses, ep.Scoring)
	providerStatuses = appendProviderStatuses(providerStatuses, ep.Eval)
	providerStatuses = appendProviderStatuses(providerStatuses, ep.ToolRuntime)
	providerStatuses = appendProviderStatuses(providerStatuses, ep.PostTraining)

	status.ExternalProviderStatus = providerStatuses
}

// appendProviderStatuses adds provider status entries for a list of provider refs.
func appendProviderStatuses(statuses []llamav1alpha1.ExternalProviderStatus, refs []llamav1alpha1.ExternalProviderRef) []llamav1alpha1.ExternalProviderStatus {
	for _, ref := range refs {
		statuses = append(statuses, llamav1alpha1.ExternalProviderStatus{
			ProviderID:         ref.ProviderID,
			Image:              ref.Image,
			Phase:              llamav1alpha1.ExternalProviderPhasePending,
			Message:            "Waiting for init container to run",
			InitContainerName:  "install-provider-" + ref.ProviderID,
			LastTransitionTime: metav1.NewTime(metav1.Now().UTC()),
		})
	}
	return statuses
}

// UpdateExternalProviderStatus updates the status of a specific external provider.
func UpdateExternalProviderStatus(status *llamav1alpha1.LlamaStackDistributionStatus, providerID string, phase llamav1alpha1.ExternalProviderPhase, message string) {
	for i := range status.ExternalProviderStatus {
		if status.ExternalProviderStatus[i].ProviderID == providerID {
			status.ExternalProviderStatus[i].Phase = phase
			status.ExternalProviderStatus[i].Message = message
			status.ExternalProviderStatus[i].LastTransitionTime = metav1.NewTime(metav1.Now().UTC())
			return
		}
	}
}

// GetExternalProviderStatus returns the status of a specific external provider.
func GetExternalProviderStatus(status *llamav1alpha1.LlamaStackDistributionStatus, providerID string) *llamav1alpha1.ExternalProviderStatus {
	for i := range status.ExternalProviderStatus {
		if status.ExternalProviderStatus[i].ProviderID == providerID {
			return &status.ExternalProviderStatus[i]
		}
	}
	return nil
}

// AllExternalProvidersReady returns true if all external providers are in Ready phase.
func AllExternalProvidersReady(status *llamav1alpha1.LlamaStackDistributionStatus) bool {
	for _, ps := range status.ExternalProviderStatus {
		if ps.Phase != llamav1alpha1.ExternalProviderPhaseReady {
			return false
		}
	}
	return true
}

// AnyExternalProviderFailed returns true if any external provider is in Failed phase.
func AnyExternalProviderFailed(status *llamav1alpha1.LlamaStackDistributionStatus) bool {
	for _, ps := range status.ExternalProviderStatus {
		if ps.Phase == llamav1alpha1.ExternalProviderPhaseFailed {
			return true
		}
	}
	return false
}
