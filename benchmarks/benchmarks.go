package main

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/hm9000/testhelpers/natsrunner"
	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"
	"github.com/cloudfoundry/hmperformance/simulator"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"time"
)

var nats *natsrunner.NATSRunner
var etcd *storerunner.ETCDClusterRunner
var cmdsToStop []*exec.Cmd

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

	r := rand.New(rand.NewSource(time.Now().Unix()))
	sim := simulator.New(100000, 10, r, nats.MessageBus, fakeCC)

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
	for {
		t := time.Now()
		sim.TickOneSecond()
		fmt.Printf("TICKING SIMULATOR TOOK: %s\n", time.Since(t))
		time.Sleep(time.Second)
	}
}

func Fetch() {
	for {
		t := time.Now()
		url := "http://metrics_server_user:canHazMetrics@10.80.130.83:7879/varz"
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
