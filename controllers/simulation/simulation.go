package simulation

import (
	"context"
	"fmt"
	"reflect"

	toolsv1 "github.com/allinbits/runsim-operator/api/v1"
	"github.com/allinbits/runsim-operator/internal/genesis"
)

func (r *SimulationReconciler) ReconcileSimulation(ctx context.Context, sim *toolsv1.Simulation) error {
	log := r.log.WithValues("simulations", sim.Name)

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

		if r.opts.LogBackupEnabled {
			if err := r.backupJobLogs(ctx, sim, job); err != nil {
				return err
			}
		}

		if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
			if err := r.removeSafeToEvictAnnotation(job); err != nil {
				return err
			}
		}
	}

	// Delete jobs removed from spec
	for _, s := range sim.Status.JobStatus {
		if !contains(sim.Spec.Config.Seeds, s.Seed) {
			if err := r.MaybeDeleteJob(ctx, sim, s.Seed); err != nil {
				return err
			}
			removeJobFromStatus(sim, s.Name)
		}
	}

	log.Info("updating status")
	updateGlobalStatus(sim)
	if err := updateGenesisStatus(sim); err != nil {
		return fmt.Errorf("could not retrieve information from genesis: %v", err)
	}
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

	if sim.Spec.Config.BlockSize == 0 {
		sim.Spec.Config.BlockSize = DefaultBlockSize
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
	var running, failed, succeeded, pending int

	for _, job := range sim.Status.JobStatus {
		switch job.Status {
		case toolsv1.SimulationRunning:
			running += 1
		case toolsv1.SimulationSucceed:
			succeeded += 1
		case toolsv1.SimulationFailed:
			failed += 1
		case toolsv1.SimulationPending:
			pending += 1
		}
	}

	sim.Status.Succeeded = &succeeded
	sim.Status.Failed = &failed
	sim.Status.Running = &running
	sim.Status.Pending = &pending

	switch {
	case succeeded == len(sim.Status.JobStatus):
		sim.Status.Status = toolsv1.SimulationSucceed
	case failed > 0:
		sim.Status.Status = toolsv1.SimulationFailed
	case pending == len(sim.Status.JobStatus):
		sim.Status.Status = toolsv1.SimulationPending
	default:
		sim.Status.Status = toolsv1.SimulationRunning
	}

}

func updateGenesisStatus(sim *toolsv1.Simulation) error {
	if sim.Spec.Config.Genesis != nil && sim.Spec.Config.Genesis.FromURL != "" {
		if sim.Status.Genesis == nil {
			chainId, hash, err := genesis.GetChainIdAndHashFromRemote(sim.Spec.Config.Genesis.FromURL)
			if err != nil {
				return err
			}
			sim.Status.Genesis = &toolsv1.GenesisInfo{
				ChainId: chainId,
				Sha256:  hash,
			}
		}
	}
	return nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
