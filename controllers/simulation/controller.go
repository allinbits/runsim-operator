package simulation

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	toolsv1 "github.com/allinbits/runsim-operator/api/v1"
)

// SimulationReconciler reconciles a Simulation object
type SimulationReconciler struct {
	client.Client
	log       logr.Logger
	scheme    *runtime.Scheme
	clientset *kubernetes.Clientset
	minio     *minio.Client
	opts      *Options
}

func SetupSimulationReconciler(mgr ctrl.Manager, opts ...Option) error {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}

	r := SimulationReconciler{
		Client:    mgr.GetClient(),
		log:       ctrl.Log.WithName("controllers").WithName("Simulations"),
		scheme:    mgr.GetScheme(),
		clientset: clientset,
		opts:      options,
	}

	if options.LogBackupEnabled {
		r.minio, err = minio.New(options.MinioEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(options.S3AccessKeyId, options.S3SecretAccessKey, ""),
			Secure: true,
		})
		if err != nil {
			return err
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&toolsv1.Simulation{}).
		Watches(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{OwnerType: &toolsv1.Simulation{}}).
		Complete(&r)
}

// +kubebuilder:rbac:groups=tools.cosmos.network,resources=simulations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tools.cosmos.network,resources=simulations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get;list;watch

func (r *SimulationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	var sim toolsv1.Simulation
	if err := r.Get(ctx, req.NamespacedName, &sim); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return ctrl.Result{}, nil
		}
		r.log.WithValues("simulation", req.NamespacedName).Error(err, "unable to fetch Simulation resource")
		return ctrl.Result{
			RequeueAfter: time.Second * 30,
		}, err
	}

	if r.setSimulationDefaults(&sim) {
		return ctrl.Result{Requeue: true}, r.Update(ctx, &sim)
	}

	r.log.WithValues("simulation", sim.Name).Info("reconciling")
	return ctrl.Result{}, r.ReconcileSimulation(ctx, &sim)
}
