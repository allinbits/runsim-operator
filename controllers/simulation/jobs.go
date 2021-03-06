package simulation

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"

	toolsv1 "github.com/allinbits/runsim-operator/api/v1"
)

func (r *SimulationReconciler) CreateJob(ctx context.Context, sim *toolsv1.Simulation, seed string) (*batchv1.Job, error) {
	job := getJobSpec(sim, seed)

	if r.opts.ImagePullSecret != "" {
		job.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{
			Name: r.opts.ImagePullSecret,
		}}
	}

	if err := ctrl.SetControllerReference(sim, job, r.scheme); err != nil {
		return nil, err
	}

	if err := r.Create(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

func (r *SimulationReconciler) MaybeDeleteJob(ctx context.Context, sim *toolsv1.Simulation, seed string) error {
	err := r.Delete(ctx, getJobSpec(sim, seed))
	if err != nil && errors.IsNotFound(err) {
		return nil
	}
	return err
}

func (r *SimulationReconciler) GetJob(ctx context.Context, sim *toolsv1.Simulation, seed string) (*batchv1.Job, error) {
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

func (r *SimulationReconciler) getJobPods(job *batchv1.Job) ([]*corev1.Pod, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"controller-uid": string(job.ObjectMeta.UID)}}
	podList, err := r.clientset.CoreV1().Pods(job.Namespace).List(metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	})
	if err != nil {
		return nil, err
	}
	pods := make([]*corev1.Pod, len(podList.Items))
	for i, pod := range podList.Items {
		pods[i] = &pod
	}
	return pods, err
}

func (r *SimulationReconciler) removeSafeToEvictAnnotation(job *batchv1.Job) error {
	pods, err := r.getJobPods(job)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		if _, ok := pod.Annotations[CASafeToEvictAnnotation]; ok {
			delete(pod.Annotations, CASafeToEvictAnnotation)
			if _, err := r.clientset.CoreV1().Pods(job.Namespace).Update(pod); err != nil {
				return err
			}
		}
	}
	return nil
}

func getJobName(sim *toolsv1.Simulation, seed string) string {
	return fmt.Sprintf("%s-%s", sim.Name, seed)
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
	case job.Status.Active > 0:
		status = toolsv1.SimulationRunning
	default:
		status = toolsv1.SimulationPending
	}

	jobsExists := false
	for i, j := range sim.Status.JobStatus {
		if j.Name == job.Name {
			jobsExists = true
			sim.Status.JobStatus[i].Status = status
		}
	}

	if !jobsExists {
		sim.Status.JobStatus = append(sim.Status.JobStatus, toolsv1.JobStatus{
			Name:   job.Name,
			Seed:   job.Annotations[SeedAnnotation],
			Status: status,
		})
	}

	return nil
}

func removeJobFromStatus(sim *toolsv1.Simulation, jobName string) {
	for i, j := range sim.Status.JobStatus {
		if j.Name == jobName {
			sim.Status.JobStatus = append(sim.Status.JobStatus[:i], sim.Status.JobStatus[i+1:]...)
			return
		}
	}
}

func getJobSpec(sim *toolsv1.Simulation, seed string) *batchv1.Job {
	simCommand := "trap \"[ -p /workspace/.tmp/params ] && echo '' > /workspace/.tmp/params; " +
		"[ -p /workspace/.tmp/state ] && echo '' > /workspace/.tmp/state\" EXIT; cd /workspace; " +
		getSimulationCmd(sim, seed)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getJobName(sim, seed),
			Namespace: sim.Namespace,
			Labels: map[string]string{
				NameLabelKey: sim.Name,
			},
			Annotations: map[string]string{
				SeedAnnotation: seed,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: pointer.Int32Ptr(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						CASafeToEvictAnnotation: "false",
					},
				},
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
						// Initialize required fifos
						{
							Name:  "init-fifos",
							Image: "busybox",
							Args: []string{
								"sh", "-c",
								"mkdir -p /workspace/.tmp && mkfifo /workspace/.tmp/state && mkfifo /workspace/.tmp/params",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/workspace",
								},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
					Containers: []corev1.Container{
						// Main container performing the simulation
						{
							Name:  simulationContainerName,
							Image: "golang",
							Args:  []string{"bash", "-c", simCommand},
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
						{
							Name:  stateContainerName,
							Image: "busybox",
							Args:  []string{"sh", "-c", "cat /workspace/.tmp/state && rm /workspace/.tmp/state"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/workspace",
								},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						{
							Name:  paramsContainerName,
							Image: "busybox",
							Args:  []string{"sh", "-c", "cat /workspace/.tmp/params && rm /workspace/.tmp/params"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/workspace",
								},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	if sim.Spec.Config.Genesis != nil && sim.Spec.Config.Genesis.FromURL != "" {
		job.Spec.Template.Spec.InitContainers = append(job.Spec.Template.Spec.InitContainers, corev1.Container{
			Name:  "download-genesis",
			Image: "busybox",
			Args: []string{
				"sh", "-c",
				fmt.Sprintf("wget %s --no-check-certificate -O /workspace/.tmp/genesis.json", sim.Spec.Config.Genesis.FromURL),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "data",
					MountPath: "/workspace",
				},
			},
		})
	} else if sim.Spec.Config.Genesis != nil && sim.Spec.Config.Genesis.FromConfigMap != nil {
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

func getSimulationCmd(sim *toolsv1.Simulation, seed string) string {
	cmd := fmt.Sprintf("go test %s ", sim.Spec.Target.Package)

	if sim.Spec.Config.Benchmark {
		cmd += fmt.Sprintf("-bench=%s -run=nothing ", sim.Spec.Config.Test)
	} else {
		cmd += fmt.Sprintf("-run=%s ", sim.Spec.Config.Test)
	}

	cmd += fmt.Sprintf("-Enabled=true -NumBlocks=%d -Verbose=true -Commit=true -BlockSize=%d"+
		" -Seed=%s -Period=%d -v -timeout %s -ExportParamsPath /workspace/.tmp/params -ExportStatePath /workspace/.tmp/state",
		sim.Spec.Config.Blocks, sim.Spec.Config.BlockSize, seed, sim.Spec.Config.Period, sim.Spec.Config.Timeout)
	if sim.Spec.Config.Genesis != nil && sim.Spec.Config.Genesis.FromURL != "" {
		cmd += " -Genesis=/workspace/.tmp/genesis.json"
	} else if sim.Spec.Config.Genesis != nil && sim.Spec.Config.Genesis.FromConfigMap != nil {
		cmd += fmt.Sprintf(" -Genesis=%s/%s", genesisMountPath, sim.Spec.Config.Genesis.FromConfigMap.Key)
	}
	return cmd
}
