package controllers

//+kubebuilder:rbac:groups=llamastack.io,resources=llamastackdistributions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=llamastack.io,resources=llamastackdistributions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=llamastack.io,resources=llamastackdistributions/finalizers,verbs=update

// +kubebuilder:rbac:groups="core",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups="core",resources=pods/log,verbs=*
// +kubebuilder:rbac:groups="core",resources=pods/exec,verbs=*
// +kubebuilder:rbac:groups="core",resources=pods,verbs=*

// +kubebuilder:rbac:groups="core",resources=persistentvolumes,verbs=*
// +kubebuilder:rbac:groups="core",resources=persistentvolumeclaims,verbs=*

// +kubebuilder:rbac:groups="apps",resources=deployments/finalizers,verbs=*
// +kubebuilder:rbac:groups="core",resources=deployments,verbs=*
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=*

// +kubebuilder:rbac:groups="core",resources=services/finalizers,verbs=create;delete;list;update;watch;patch;get
// +kubebuilder:rbac:groups="core",resources=services,verbs=get;create;watch;update;patch;list;delete
// +kubebuilder:rbac:groups="core",resources=services,verbs=*

// +kubebuilder:rbac:groups="core",resources=configmaps,verbs=*

// +kubebuilder:rbac:groups="networking.k8s.io",resources=networkpolicies,verbs=*
