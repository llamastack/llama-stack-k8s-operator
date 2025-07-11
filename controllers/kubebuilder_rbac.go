package controllers

// LlamaStackDistribution CRD permissions
//+kubebuilder:rbac:groups=llamastack.io,resources=llamastackdistributions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=llamastack.io,resources=llamastackdistributions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=llamastack.io,resources=llamastackdistributions/finalizers,verbs=update

// Deployment permissions - controller creates and manages deployments
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Service permissions - controller creates and manages services
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// PVC permissions - controller creates PVCs (immutable after creation, no update/patch needed)
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create

// ConfigMap permissions - controller reads user configmaps and manages operator config configmaps
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch

// NetworkPolicy permissions - controller creates and manages network policies
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
