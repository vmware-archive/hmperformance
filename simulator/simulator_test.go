package simulator_test

import (
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hmperformance/simulator"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"

	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"math/rand"
	"sort"
)

var _ = Describe("Simulator", func() {
	var (
		sim                *Simulator
		messageBus         *fakeyagnats.FakeYagnats
		desiredStateServer *desiredstateserver.DesiredStateServer
		randomizer         *rand.Rand
	)

	BeforeEach(func() {
		desiredStateServer = desiredstateserver.NewDesiredStateServer()
		messageBus = fakeyagnats.New()
		randomizer = rand.New(rand.NewSource(time.Now().Unix()))
		sim = New(100, 8, randomizer, messageBus, desiredStateServer)
	})

	Describe("Preparing the simulator", func() {
		It("should generate the requested number of apps", func() {
			Ω(sim.Apps).Should(HaveLen(100))
		})

		It("should report no time has elapsed yet", func() {
			Ω(sim.SecondsElapsed).Should(Equal(0))
		})

		It("should populate the desired state server", func() {
			Ω(desiredStateServer.Apps).Should(HaveLen(100))
			for _, desired := range desiredStateServer.Apps {
				Ω(desired.NumberOfInstances).Should(Equal(1))
				Ω(desired.State).Should(Equal(models.AppStateStarted))
				Ω(desired.PackageState).Should(Equal(models.AppPackageStateStaged))
			}
		})

		Context("when the # of requested apps fit within the initial DEA count", func() {
			It("should generate the requested number of DEAs", func() {
				Ω(sim.DEAs).Should(HaveLen(8))
			})

			It("should spread the apps evenly among the DEAs", func() {
				deas := make(map[string]int)
				for _, app := range sim.Apps {
					deas[app.DeaGuid] += 1
				}
				Ω(deas).Should(HaveLen(8))
				for dea, appCount := range deas {
					Ω(appCount).Should(BeNumerically("~", 13, 1))
					Ω(sim.DEAs[dea]).Should(HaveLen(appCount))
				}
			})
		})

		Context("when the # of requested apps need more DEAs to fit", func() {
			It("should fill up the DEAs then generate new ones as needed", func() {
				sim = New(250, 2, randomizer, messageBus, desiredStateServer)
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

	Describe("Ticking the simulator", func() {
		BeforeEach(func() {
			sim.NumberOfNewAppsStartingPerHB = 0 //turn this off to make these tests simpler
		})

		It("should emit a heartbeat for each DEA by every heartbeat interval", func() {
			Ω(sim.HeartbeatIntervalInSeconds).Should(Equal(10))
			for i := 0; i < sim.HeartbeatIntervalInSeconds; i++ {
				sim.TickOneSecond()
			}
			Ω(messageBus.PublishedMessages["dea.heartbeat"]).Should(HaveLen(8))

			heartbeats := map[string]models.Heartbeat{}
			for _, message := range messageBus.PublishedMessages["dea.heartbeat"] {
				heartbeat, err := models.NewHeartbeatFromJSON(message.Payload)
				Ω(err).ShouldNot(HaveOccured())
				heartbeats[heartbeat.DeaGuid] = heartbeat
			}

			for dea, apps := range sim.DEAs {
				Ω(heartbeats[dea].InstanceHeartbeats).Should(HaveLen(len(apps)))
			}
		})

		It("should emit an advertise for each DEA by every advertise interval", func() {
			Ω(sim.AdvertiseIntervalInSeconds).Should(Equal(5))
			for i := 0; i < sim.AdvertiseIntervalInSeconds; i++ {
				sim.TickOneSecond()
			}
			Ω(messageBus.PublishedMessages["dea.advertise"]).Should(HaveLen(8))
		})

		It("should report a consistent portion of apps as crashed", func() {
			Ω(sim.FractionOfAppsThatAreCrashed).Should(Equal(0.1))

			for i := 0; i < sim.HeartbeatIntervalInSeconds; i++ {
				sim.TickOneSecond()
			}

			appStates := map[models.InstanceState]int{}
			total := 0
			for _, message := range messageBus.PublishedMessages["dea.heartbeat"] {
				heartbeat, err := models.NewHeartbeatFromJSON(message.Payload)
				Ω(err).ShouldNot(HaveOccured())
				for _, instanceHeartbeat := range heartbeat.InstanceHeartbeats {
					appStates[instanceHeartbeat.State]++
					total++
				}
			}

			Ω(total).Should(Equal(100))
			Ω(appStates[models.InstanceStateCrashed]).Should(BeNumerically("~", 10, 8), "This may fail.. non-deterministally, but it should fail *very* infrequently.")
			Ω(appStates[models.InstanceStateCrashed] + appStates[models.InstanceStateRunning]).Should(Equal(total))
		})
	})

	Describe("Adding new apps with time", func() {
		It("should add apps at the specified rate", func() {
			Ω(sim.NumberOfNewAppsStartingPerHB).Should(Equal(10))
			for i := 0; i < 12; i++ {
				messageBus.Reset()
				for j := 0; j < sim.HeartbeatIntervalInSeconds; j++ {
					sim.TickOneSecond()
				}
			}

			numApps := 100 + 12*10

			Ω(sim.Apps).Should(HaveLen(numApps))
			Ω(sim.DEAs).Should(HaveLen(8), "Shouldn't have needed any additional DEAs")
			Ω(desiredStateServer.Apps).Should(HaveLen(numApps))
		})
	})

	Describe("Scenarios", func() {
		Describe("Removing an entire DEA", func() {

		})
	})
})
