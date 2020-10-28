package simulation

import (
	"context"
	"fmt"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"

	toolsv1 "github.com/allinbits/runsim-operator/api/v1"
)

func (r *SimulationReconciler) CreateJob(ctx context.Context, sim *toolsv1.Simulation, seed int) (*batchv1.Job, error) {
	job := getJobSpec(sim, seed)

	if err := ctrl.SetControllerReference(sim, job, r.Scheme); err != nil {
		return nil, err
	}

	if err := r.Create(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

func (r *SimulationReconciler) GetJob(ctx context.Context, sim *toolsv1.Simulation, seed int) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Namespace: sim.Namespace, Name: getJobName(sim, seed)}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func getJobName(sim *toolsv1.Simulation, seed int) string {
	return fmt.Sprintf("%s-%d", sim.Name, seed)
}

func updateJobStatus(sim *toolsv1.Simulation, job *batchv1.Job) error {
	if sim.Status.JobStatus == nil {
		sim.Status.JobStatus = make([]toolsv1.JobStatus, 0)
	}

	// Get the simulation status
	var status toolsv1.SimStatus
	switch {
	case job.Status.Succeeded > 0:
		status = toolsv1.SimulationSucceed
	case job.Status.Failed > 0:
		status = toolsv1.SimulationFailed
	default:
		status = toolsv1.SimulationRunning
	}

	jobsExists := false
	for i, j := range sim.Status.JobStatus {
		if j.Name == job.Name {
			jobsExists = true
			sim.Status.JobStatus[i].Status = status
		}
	}

	if !jobsExists {
		seed, err := strconv.Atoi(job.Annotations[SeedAnnotation])
		if err != nil {
			return err
		}
		sim.Status.JobStatus = append(sim.Status.JobStatus, toolsv1.JobStatus{
			Name:   job.Name,
			Seed:   seed,
			Status: status,
		})
	}

	return nil
}

func getJobSpec(sim *toolsv1.Simulation, seed int) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getJobName(sim, seed),
			Namespace: sim.Namespace,
			Labels: map[string]string{
				NameLabelKey: sim.Name,
			},
			Annotations: map[string]string{
				SeedAnnotation: strconv.Itoa(seed),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: pointer.Int32Ptr(0),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						// Volume where app repository will be cloned to
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						// Volume to hold go directory
						{
							Name: "go",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					InitContainers: []corev1.Container{
						// Init container for cloning repository
						{
							Name:  "clone-repo",
							Image: "alpine/git",
							Args: []string{
								"clone", "--single-branch",
								"--depth", "1",
								"--branch", sim.Spec.Target.Version,
								sim.Spec.Target.Repo, "/workspace",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/workspace",
								},
							},
						},
						// Download go dependencies
						{
							Name:  "go-mod",
							Image: "golang",
							Args:  []string{"bash", "-c", "cd /workspace && go mod download"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/workspace",
								},
								{
									Name:      "go",
									MountPath: "/go",
								},
							},
						},
					},
					Containers: []corev1.Container{
						// Main container performing the simulation
						{
							Name:  "simulation",
							Image: "golang",
							Args:  []string{"bash", "-c", fmt.Sprintf("cd /workspace && %s", getSimulationCmd(sim, seed))},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/workspace",
								},
								{
									Name:      "go",
									MountPath: "/go",
								},
							},
							Resources: sim.Spec.Config.Resources,
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	if sim.Spec.Config.Genesis != nil && sim.Spec.Config.Genesis.FromConfigMap != nil {
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "genesis",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: sim.Spec.Config.Genesis.FromConfigMap.Name,
					},
				},
			},
		})
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "genesis",
			ReadOnly:  true,
			MountPath: genesisMountPath,
		})
	}

	return job
}

func getSimulationCmd(sim *toolsv1.Simulation, seed int) string {
	cmd := fmt.Sprintf("go test %s -run %s -Enabled=true -NumBlocks=%d -Verbose=true -Commit=true -Seed=%d -Period=%d -v -timeout %s",
		sim.Spec.Target.Package, sim.Spec.Config.Test, sim.Spec.Config.Blocks, seed, sim.Spec.Config.Period, sim.Spec.Config.Timeout)
	if sim.Spec.Config.Genesis != nil && sim.Spec.Config.Genesis.FromConfigMap != nil {
		cmd += fmt.Sprintf(" -Genesis=%s/%s", genesisMountPath, sim.Spec.Config.Genesis.FromConfigMap.Key)
	}
	return cmd
}
