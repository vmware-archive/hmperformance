package simulator

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/yagnats"
)

type Simulator struct {
	MaxDEACapacity int

	FractionOfNewAppsStartingPerHB float64
	FractionOfAppsStoppingPerHB    float64
	FractionOfAppsThatAreCrashed   float64

	HeartbeatIntervalInSeconds int
	AdvertiseIntervalInSeconds int

	Apps           []*appfixture.AppFixture
	SecondsElapsed int

	deas map[string][]*appfixture.AppFixture

	messageBus         yagnats.NATSClient
	desiredStateServer desiredstateserver.DesiredStateServerInterface
}

func New(numberOfApps int, initialDEACount int, messageBus yagnats.NATSClient, desiredStateServer desiredstateserver.DesiredStateServerInterface) *Simulator {
	simulator := &Simulator{
		MaxDEACapacity:                 100,
		FractionOfNewAppsStartingPerHB: 0.01,
		FractionOfAppsStoppingPerHB:    0.01,
		FractionOfAppsThatAreCrashed:   0.1,
		HeartbeatIntervalInSeconds:     10,
		AdvertiseIntervalInSeconds:     5,
		messageBus:                     messageBus,
		desiredStateServer:             desiredStateServer,
		Apps:                           []*appfixture.AppFixture{},
		deas:                           map[string][]*appfixture.AppFixture{},
	}

	simulator.prepare(numberOfApps, initialDEACount)
	return simulator
}

func (s *Simulator) TickOneSecond() {

}

func (s *Simulator) prepare(numberOfApps, initialDEACount int) {
	for i := 0; i < initialDEACount; i++ {
		s.deas[models.Guid()] = make([]*appfixture.AppFixture, 0)
	}
	for i := 0; i < numberOfApps; i++ {
		s.buildNewApp()
	}
}

func (s *Simulator) buildNewApp() {
	app := appfixture.NewAppFixture()
	winner := models.Guid()
	lowball := 1000000
	for dea, apps := range s.deas {
		if len(apps)+1 > s.MaxDEACapacity {
			continue
		}
		if len(apps) < lowball {
			lowball = len(apps)
			winner = dea
		}
	}

	app.DeaGuid = winner
	s.deas[winner] = append(s.deas[winner], &app)
	s.Apps = append(s.Apps, &app)
}
