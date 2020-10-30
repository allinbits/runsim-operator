package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SimStatus string

const (
	SimulationRunning SimStatus = "Running"
	SimulationFailed  SimStatus = "Failed"
	SimulationSucceed SimStatus = "Succeed"
	SimulationPending SimStatus = "Pending"
)

// SimulationSpec defines the desired state of Simulation
type SimulationSpec struct {
	// Specifies the target package to run simulations for
	// +optional
	Target TargetSpec `json:"target,omitempty"`

	// Specifies simulation parameters
	// +optional
	Config ConfigSpec `json:"config,omitempty"`
}

// ConfigSpec specifies the target package to run simulations for
type ConfigSpec struct {
	// The name of the test to run.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:default=TestFullAppSimulation
	Test string `json:"test,omitempty"`

	// For how many blocks the simulation should run.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=100
	Blocks int `json:"blocks,omitempty"`

	// Block period.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	Period int `json:"period,omitempty"`

	// Timeout at which the simulations will fail if they run longer than it.
	// +optional
	// +kubebuilder:validation:Pattern=\d+(s|m|h)
	// +kubebuilder:default="24h"
	Timeout string `json:"timeout,omitempty"`

	// Seeds to run simulations for.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:default={1,2,4,7,32,123,124,582,1893,2989,3012,4728,37827,981928,87821,891823782,989182,89182391,11,22,44,77,99,2020,3232,123123,124124,582582,18931893,29892989,30123012,47284728,7601778,8090485,977367484,491163361,424254581,673398983}
	Seeds []int `json:"seeds,omitempty"`

	// Resources describes the desired compute resource requirements for each simulation job.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Genesis specifies the genesis to be provided to the simulation.
	// +optional
	Genesis *GenesisSpec `json:"genesis,omitempty"`
}

// GenesisSpec specifies the genesis to be provided to the simulation.
type GenesisSpec struct {
	// Allows specifying a genesis from a configmap.
	// +optional
	FromConfigMap *FromConfigMapConfig `json:"fromConfigMap,omitempty"`

	// Allows specifying a genesis from a URL
	// +optional
	FromURL string `json:"fromUrl,omitempty"`
}

type FromConfigMapConfig struct {
	// Name of the configmap.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key specifies the key in configmap containing the genesis file.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:default="genesis.json"
	Key string `json:"key,omitempty"`
}

// TargetSpec specifies simulation parameters
type TargetSpec struct {
	// The repository that contains the package to run simulations for.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:default="https://github.com/cosmos/cosmos-sdk"
	Repo string `json:"repo,omitempty"`

	// The repository that contains the package to run simulations for.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:default=master
	Version string `json:"version,omitempty"`

	// The package to run simulations for.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:default=simapp
	Package string `json:"package,omitempty"`
}

// SimulationStatus defines the observed state of Simulation
type SimulationStatus struct {
	// Global simulations status.
	// +optional
	Status SimStatus `json:"status"`

	// The number of jobs running.
	Running *int `json:"running"`

	// The number of jobs that completed successfully.
	Succeeded *int `json:"succeeded"`

	// The number of jobs that failed.
	Failed *int `json:"failed"`

	// The number of jobs that is pending.
	Pending *int `json:"pending"`

	// Per job simulation status.
	// +optional
	JobStatus []JobStatus `json:"jobStatus"`
}

// JobStatus indicates the simulation status per job.
type JobStatus struct {
	// The name of the job running the simulation.
	Name string `json:"name"`

	// The seed being run by the simulation.
	Seed int `json:"seed"`

	// The status of this job's simulation.
	Status SimStatus `json:"status"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Running",type=integer,JSONPath=`.status.running`
// +kubebuilder:printcolumn:name="Succeeded",type=integer,JSONPath=`.status.succeeded`
// +kubebuilder:printcolumn:name="Failed",type=integer,JSONPath=`.status.failed`
// +kubebuilder:printcolumn:name="Pending",type=integer,JSONPath=`.status.pending`

// Simulation is the Schema for the simulations API
type Simulation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SimulationSpec   `json:"spec,omitempty"`
	Status SimulationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SimulationList contains a list of Simulation
type SimulationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Simulation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Simulation{}, &SimulationList{})
}
