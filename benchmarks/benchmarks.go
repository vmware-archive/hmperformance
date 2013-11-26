package main

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/workerpool"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/natsrunner"
	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"
	"github.com/cloudfoundry/hmperformance/simulator"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"time"
)

var nats *natsrunner.NATSRunner
var etcd *storerunner.ETCDClusterRunner
var cmdsToStop []*exec.Cmd
var store storepackage.Store

const collectorFetchInterval = 30

func main() {
	cmdsToStop = []*exec.Cmd{}

	registerSignalHandler()
	nats = natsrunner.NewNATSRunner(4222)
	nats.Start()

	fakeCC := desiredstateserver.NewDesiredStateServer()
	go fakeCC.SpinUp(6001)

	etcd = storerunner.NewETCDClusterRunner(4001, 1)
	etcd.Start()
	adapter := storeadapter.NewETCDStoreAdapter(etcd.NodeURLS(), workerpool.NewWorkerPool(30))
	adapter.Connect()
	conf, _ := config.FromFile("./config.json")
	store = storepackage.NewStore(conf, adapter, fakelogger.NewFakeLogger())

	r := rand.New(rand.NewSource(time.Now().Unix()))
	num, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(err)
	}

	sim := simulator.New(num, 10, r, nats.MessageBus, fakeCC)
	//start all the HM components (make them pipe to stdout)
	start("listen", false)
	start("fetch_desired", true)
	start("analyze", true)
	start("send", true)
	start("serve_metrics", false)
	start("serve_api", false)
	time.Sleep(time.Second)
	go Tick(sim)
	go Fetch()
	select {}
}

func Tick(sim *simulator.Simulator) {
	startTime := time.Now()
	for {
		t := time.Now()
		sim.TickOneSecond()
		fmt.Printf("TICKING SIMULATOR TOOK: %s\n", time.Since(t))
		time.Sleep(time.Second)
		err := store.VerifyFreshness(time.Now())
		if err == nil {
			fmt.Printf("\n\n~~~~ STORE IS FRESH: %s\n\n", time.Since(startTime))
		} else {
			fmt.Printf("\n\n~~~~ STORE IS NOT FRESH: %s (%s)\n\n", time.Since(startTime), err.Error())
		}
		i++
	}
}

func Fetch() {
	for {
		t := time.Now()
		ip, err := localip.LocalIP()
		if err != nil {
			panic(err)
		}
		url := "http://metrics_server_user:canHazMetrics@" + ip + ":7879/varz"
		resp, err := http.Get(url)
		if err != nil {
			panic(err)
		}
		fmt.Printf("FETCHING METRICS TOOK: %s", time.Since(t))
		resp.Body.Close()
		time.Sleep(collectorFetchInterval * time.Second)
	}
}

func registerSignalHandler() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		select {
		case <-c:
			nats.Stop()
			etcd.Stop()
			for _, cmd := range cmdsToStop {
				cmd.Process.Signal(os.Interrupt)
				cmd.Wait()
			}
			os.Exit(0)
		}
	}()
}

func start(command string, polling bool) {
	args := []string{command, "--config=./config.json"}
	if polling {
		args = append(args, "--poll")
	}

	cmd := exec.Command("hm9000", args...)
	outPipe, _ := cmd.StdoutPipe()
	errPipe, _ := cmd.StderrPipe()
	go func() { io.Copy(os.Stdout, outPipe) }()
	go func() { io.Copy(os.Stdout, errPipe) }()

	cmd.Start()
	cmdsToStop = append(cmdsToStop, cmd)
}
