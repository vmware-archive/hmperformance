package simulator

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/yagnats"
	"math/rand"
)

type Simulator struct {
	MaxDEACapacity int

	NumberOfNewAppsStartingPerHB int
	FractionOfAppsThatAreCrashed float64

	HeartbeatIntervalInSeconds int
	AdvertiseIntervalInSeconds int

	Apps             []*appfixture.AppFixture
	AppInstanceState map[string]models.InstanceState
	SecondsElapsed   int

	DEAs                 map[string][]*appfixture.AppFixture
	DEAHeartbeatSchedule map[string]int
	DEAAdvertiseSchedule map[string]int

	randomizer         *rand.Rand
	messageBus         yagnats.NATSClient
	desiredStateServer desiredstateserver.DesiredStateServerInterface
}

func New(numberOfApps int, initialDEACount int, randomizer *rand.Rand, messageBus yagnats.NATSClient, desiredStateServer desiredstateserver.DesiredStateServerInterface) *Simulator {
	simulator := &Simulator{
		MaxDEACapacity:               100,
		NumberOfNewAppsStartingPerHB: 10,
		FractionOfAppsThatAreCrashed: 0.1,
		HeartbeatIntervalInSeconds:   10,
		AdvertiseIntervalInSeconds:   5,

		randomizer:         randomizer,
		messageBus:         messageBus,
		desiredStateServer: desiredStateServer,

		Apps:                 []*appfixture.AppFixture{},
		AppInstanceState:     map[string]models.InstanceState{},
		DEAs:                 map[string][]*appfixture.AppFixture{},
		DEAHeartbeatSchedule: map[string]int{},
		DEAAdvertiseSchedule: map[string]int{},
	}

	simulator.prepare(numberOfApps, initialDEACount)
	return simulator
}

func (s *Simulator) TickOneSecond() {
	for dea, schedule := range s.DEAAdvertiseSchedule {
		if schedule == s.SecondsElapsed%s.AdvertiseIntervalInSeconds {
			s.emitAdvertiseFor(dea)
		}
	}

	for dea, schedule := range s.DEAHeartbeatSchedule {
		if schedule == s.SecondsElapsed%s.HeartbeatIntervalInSeconds {
			s.emitHeartbeatFor(dea)
		}
	}

	s.SecondsElapsed++

	if s.SecondsElapsed%s.HeartbeatIntervalInSeconds == 0 {
		for i := 0; i < s.NumberOfNewAppsStartingPerHB; i++ {
			s.buildNewApp()
		}
		s.populateDesiredState()
	}
}

func (s *Simulator) prepare(numberOfApps, initialDEACount int) {
	for i := 0; i < initialDEACount; i++ {
		s.buildDEA()
	}
	for i := 0; i < numberOfApps; i++ {
		s.buildNewApp()
	}
	s.populateDesiredState()
}

func (s *Simulator) populateDesiredState() {
	desiredStates := make([]models.DesiredAppState, len(s.Apps))
	for i, app := range s.Apps {
		desiredStates[i] = app.DesiredState(1)
	}
	s.desiredStateServer.SetDesiredState(desiredStates)
}

func (s *Simulator) emitAdvertiseFor(dea string) {
	s.messageBus.Publish("dea.advertise", []byte(fmt.Sprintf(`{"dea":"%s"}`, dea)))
}

func (s *Simulator) emitHeartbeatFor(dea string) {
	hb := models.Heartbeat{
		DeaGuid:            dea,
		InstanceHeartbeats: make([]models.InstanceHeartbeat, len(s.DEAs[dea])),
	}

	for i, app := range s.DEAs[dea] {
		hb.InstanceHeartbeats[i] = app.InstanceAtIndex(0).Heartbeat()
		hb.InstanceHeartbeats[i].State = s.AppInstanceState[app.AppGuid]
	}

	s.messageBus.Publish("dea.heartbeat", hb.ToJSON())
}

func (s *Simulator) buildDEA() string {
	guid := models.Guid()
	s.DEAs[guid] = make([]*appfixture.AppFixture, 0)
	s.DEAHeartbeatSchedule[guid] = s.randomizer.Intn(s.HeartbeatIntervalInSeconds)
	s.DEAAdvertiseSchedule[guid] = s.randomizer.Intn(s.AdvertiseIntervalInSeconds)
	return guid
}

func (s *Simulator) buildNewApp() {
	app := appfixture.NewAppFixture()
	if s.randomizer.Float64() < s.FractionOfAppsThatAreCrashed {
		s.AppInstanceState[app.AppGuid] = models.InstanceStateCrashed
	} else {
		s.AppInstanceState[app.AppGuid] = models.InstanceStateRunning
	}

	winner := ""
	lowball := 1000000
	for dea, apps := range s.DEAs {
		if len(apps)+1 > s.MaxDEACapacity {
			continue
		}
		if len(apps) < lowball {
			lowball = len(apps)
			winner = dea
		}
	}

	if winner == "" {
		winner = s.buildDEA()
	}

	app.DeaGuid = winner
	s.DEAs[winner] = append(s.DEAs[winner], &app)
	s.Apps = append(s.Apps, &app)
}
