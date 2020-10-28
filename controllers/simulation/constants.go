package simulation

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DefaultRepo                = "https://github.com/cosmos/cosmos-sdk"
	DefaultVersion             = "master"
	DefaultPackage             = "./simapp"
	DefaultTest                = "TestFullAppSimulation"
	DefaultBlocks              = 100
	DefaultPeriod              = 1
	DefaultTimeout             = "24h"
	DefaultGenesisConfigMapKey = "genesis.json"

	genesisMountPath = "/config"

	SeedAnnotation = "tools.cosmos.network/simulation-seed"
	NameLabelKey   = "simulation"
)

var (
	DefaultSeeds = []int{
		1, 2, 4, 7, 32, 123, 124, 582, 1893, 2989,
		3012, 4728, 37827, 981928, 87821, 891823782,
		989182, 89182391, 11, 22, 44, 77, 99, 2020,
		3232, 123123, 124124, 582582, 18931893,
		29892989, 30123012, 47284728, 7601778, 8090485,
		977367484, 491163361, 424254581, 673398983,
	}

	DefaultResources = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("650m"),
			corev1.ResourceMemory: resource.MustParse("350Mi"),
		},
	}
)
