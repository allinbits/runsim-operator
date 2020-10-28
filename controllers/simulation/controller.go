package simulation

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	toolsv1 "github.com/allinbits/runsim-operator/api/v1"
)

// SimulationReconciler reconciles a Simulation object
type SimulationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *SimulationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&toolsv1.Simulation{}).
		Watches(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{OwnerType: &toolsv1.Simulation{}}).
		Complete(r)
}

// +kubebuilder:rbac:groups=tools.cosmos.network,resources=simulations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tools.cosmos.network,resources=simulations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

func (r *SimulationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	var sim toolsv1.Simulation
	if err := r.Get(ctx, req.NamespacedName, &sim); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return ctrl.Result{}, nil
		}
		r.Log.WithValues("simulation", req.NamespacedName).Error(err, "unable to fetch Simulation resource")
		return ctrl.Result{
			RequeueAfter: time.Second * 30,
		}, err
	}

	r.Log.WithValues("simulation", sim.Name).Info("reconciling")
	return ctrl.Result{}, r.ReconcileSimulation(ctx, &sim)
}
