package simulation

import (
	"bufio"
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	toolsv1 "github.com/allinbits/runsim-operator/api/v1"
)

func (r *SimulationReconciler) backupJobLogs(ctx context.Context, sim *toolsv1.Simulation, job *batchv1.Job) error {
	log := r.log.WithValues("simulations", sim.Name, "job", job.Name)

	// Ignore if job has not finished yet
	if job.Status.Succeeded == 0 && job.Status.Failed == 0 {
		return nil
	}

	// Check if logs were already backed up
	if _, ok := job.Annotations[LogBackupAnnotation]; ok {
		return nil
	}

	// Grab list of pods for the job
	var jobPods corev1.PodList
	if err := r.Client.List(ctx,
		&jobPods,
		client.InNamespace(sim.Namespace),
		client.MatchingLabels(job.Spec.Selector.MatchLabels),
	); err != nil {
		return err
	}

	if len(jobPods.Items) == 0 {
		return fmt.Errorf("job %q has no pods", job.Name)
	}
	pod := jobPods.Items[0]

	for _, container := range []string{simulationContainerName, stateContainerName, paramsContainerName} {
		log.WithValues("container", container).Info("backing up logs")

		logs, err := r.clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: container}).Stream()
		if err != nil {
			return err
		}

		reader := bufio.NewReader(logs)
		_, err = r.minio.PutObject(ctx,
			r.opts.LogsBucketName,
			fmt.Sprintf("%s/%s/%s.log", sim.Name, job.Annotations[SeedAnnotation], container),
			reader,
			-1,
			minio.PutObjectOptions{ContentType: "text/plain"},
		)
		_ = logs.Close()
		if err != nil {
			return err
		}
	}

	job.Annotations[LogBackupAnnotation] = "true"
	return r.Update(ctx, job)
}
