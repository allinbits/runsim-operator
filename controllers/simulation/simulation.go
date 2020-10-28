package simulation

import (
	"context"

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
