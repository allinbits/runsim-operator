package simulation

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	toolsv1 "github.com/allinbits/runsim-operator/api/v1"
)

var _ = Describe("Simulation controller", func() {

	const (
		SimName      = "test-simulation"
		SimNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating Simulation with 4 seeds", func() {
		It("Should create 4 jobs", func() {
			By("By creating a new Simulation")
			ctx := context.Background()
			sim := &toolsv1.Simulation{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "tools.cosmos.network/v1",
					Kind:       "Simulation",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      SimName,
					Namespace: SimNamespace,
				},
				Spec: toolsv1.SimulationSpec{
					Target: toolsv1.TargetSpec{
						Repo:    "https://github.com/cosmos/cosmos-sdk",
						Version: "master",
						Package: "./simapp",
					},
					Config: toolsv1.ConfigSpec{
						Test:    "TestFullAppSimulation",
						Blocks:  100,
						Period:  1,
						Timeout: "24h",
						Seeds:   []int{1, 2, 4, 7},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sim)).Should(Succeed())

			simulationLookupKey := types.NamespacedName{Name: SimName, Namespace: SimNamespace}
			createdSimulation := &toolsv1.Simulation{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, simulationLookupKey, createdSimulation)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("By checking the Simulation has 4 active Jobs")
			Consistently(func() (int, error) {
				err := k8sClient.Get(ctx, simulationLookupKey, createdSimulation)
				if err != nil {
					return -1, err
				}
				return len(createdSimulation.Status.JobStatus), nil
			}, duration, interval).Should(Equal(4))
		})
	})

})
