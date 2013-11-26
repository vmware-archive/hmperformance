package simulator_test

import (
	. "github.com/cloudfoundry/hmperformance/simulator"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"sort"
)

var _ = Describe("Simulator", func() {
	var (
		sim                *Simulator
		messageBus         *fakeyagnats.FakeYagnats
		desiredStateServer *desiredstateserver.DesiredStateServer
	)

	BeforeEach(func() {
		desiredStateServer = desiredstateserver.NewDesiredStateServer()
		messageBus = fakeyagnats.New()
		sim = New(100, 8, messageBus, desiredStateServer)
	})

	Describe("Preparing the simulator", func() {
		It("should generate the requested number of apps", func() {
			Ω(sim.Apps).Should(HaveLen(100))
		})

		It("should report no time has elapsed yet", func() {
			Ω(sim.SecondsElapsed).Should(Equal(0))
		})

		Context("when the # of requested apps fit within the initial DEA count", func() {
			It("should spread the apps evenly among the DEAs", func() {
				deas := make(map[string]int)
				for _, app := range sim.Apps {
					deas[app.DeaGuid] += 1
				}
				Ω(deas).Should(HaveLen(8))
				for _, appCount := range deas {
					Ω(appCount).Should(BeNumerically("~", 13, 1))
				}
			})
		})

		Context("when the # of requested apps need more DEAs to fit", func() {
			It("should fill up the DEAs then generate new ones as needed", func() {
				sim = New(250, 2, messageBus, desiredStateServer)
				deas := make(map[string]int)
				for _, app := range sim.Apps {
					deas[app.DeaGuid] += 1
				}
				Ω(deas).Should(HaveLen(3))

				appCounts := []int{}
				for _, appCount := range deas {
					appCounts = append(appCounts, appCount)
				}

				sort.Ints(appCounts)
				Ω(appCounts).Should(Equal([]int{50, 100, 100}))
			})
		})
	})
})
