package simulation

import (
	"context"
	"reflect"

	toolsv1 "github.com/allinbits/runsim-operator/api/v1"
)

func (r *SimulationReconciler) ReconcileSimulation(ctx context.Context, sim *toolsv1.Simulation) error {
	log := r.Log.WithValues("simulations", sim.Name)

	for _, seed := range sim.Spec.Config.Seeds {
		// Get the job if it already exists
		job, err := r.GetJob(ctx, sim, seed)
		if err != nil {
			return err
		}

		// Create the job if it does not exist
		if job == nil {
			log.Info("creating job", "seed", seed)
			if job, err = r.CreateJob(ctx, sim, seed); err != nil {
				return err
			}
		}

		if err := updateJobStatus(sim, job); err != nil {
			return err
		}
	}

	log.Info("updating status")
	updateGlobalStatus(sim)
	return r.Status().Update(ctx, sim)
}

func (r *SimulationReconciler) setSimulationDefaults(sim *toolsv1.Simulation) bool {
	old := sim.DeepCopy()

	if sim.Spec.Target.Repo == "" {
		sim.Spec.Target.Repo = DefaultRepo
	}

	if sim.Spec.Target.Version == "" {
		sim.Spec.Target.Version = DefaultVersion
	}

	if sim.Spec.Target.Package == "" {
		sim.Spec.Target.Package = DefaultPackage
	}

	if sim.Spec.Config.Test == "" {
		sim.Spec.Config.Test = DefaultTest
	}

	if sim.Spec.Config.Blocks == 0 {
		sim.Spec.Config.Blocks = DefaultBlocks
	}

	if sim.Spec.Config.Period == 0 {
		sim.Spec.Config.Period = DefaultPeriod
	}

	if sim.Spec.Config.Timeout == "" {
		sim.Spec.Config.Timeout = DefaultTimeout
	}

	if len(sim.Spec.Config.Seeds) == 0 {
		sim.Spec.Config.Seeds = DefaultSeeds
	}

	if sim.Spec.Config.Resources.Limits == nil && sim.Spec.Config.Resources.Requests == nil {
		sim.Spec.Config.Resources = DefaultResources
	}

	if sim.Spec.Config.Genesis != nil &&
		sim.Spec.Config.Genesis.FromConfigMap != nil &&
		sim.Spec.Config.Genesis.FromConfigMap.Key == "" {
		sim.Spec.Config.Genesis.FromConfigMap.Key = DefaultGenesisConfigMapKey
	}

	return !reflect.DeepEqual(old, sim)
}

func updateGlobalStatus(sim *toolsv1.Simulation) {
	var running, failed, succeeded int

	for _, job := range sim.Status.JobStatus {
		switch job.Status {
		case toolsv1.SimulationRunning:
			running += 1
		case toolsv1.SimulationSucceed:
			succeeded += 1
		case toolsv1.SimulationFailed:
			failed += 1
		}
	}

	sim.Status.Succeeded = &succeeded
	sim.Status.Failed = &failed
	sim.Status.Running = &running

	switch {
	case succeeded == len(sim.Status.JobStatus):
		sim.Status.Status = toolsv1.SimulationSucceed
	case failed > 0:
		sim.Status.Status = toolsv1.SimulationFailed
	default:
		sim.Status.Status = toolsv1.SimulationRunning
	}

}
