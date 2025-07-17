package controllers

// LlamaStackDistribution CRD permissions
//+kubebuilder:rbac:groups=llamastack.io,resources=llamastackdistributions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=llamastack.io,resources=llamastackdistributions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=llamastack.io,resources=llamastackdistributions/finalizers,verbs=update

// Deployment permissions - controller creates and manages deployments
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Service permissions - controller creates and manages services
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// ServiceAccount permissions - controller creates and manages service accounts for PVC permissions
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch

//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=use
//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,resourceNames=anyuid,verbs=use

//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create

// ConfigMap permissions - controller reads user configmaps and manages operator config configmaps
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch

// NetworkPolicy permissions - controller creates and manages network policies
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
