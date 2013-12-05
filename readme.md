#Performance-related test for HM9000

##To Run The Benchmark

### 1. Install HM9000 and fetch the performance repo

```bash

git clone https://github.com/cloudfoundry/hm-workspace #get the hm9000 go workspace
cd hm-workspace
git submodule update --init #get hm9000 + dependencies

export GOPATH=$PWD
export PATH=$PATH:$GOPATH/bin

go install ./src/github.com/cloudfoundry/hm9000 #install hm9000

cd ./src/github.com/cloudfoundry
git clone https://github.com/pivotal-cf-experimental/hmperformance
cd hmperformance/benchmarks

```

### 2. Start ETCD (cluster) and point the benchmarks at the cluster

- Do whatever you need to do to get etcd v2-rc1 running (either single node or multi-node)
- Edit `$GOPATH/src/github.com/cloudfoundry/hmperformance/benchmarks/config.json and set the "store_urls" entry to an array of ETCD urls (one entry in the array for each node in your cluster)

### 3. Run the benchmark:

```bash

cd $GOPATH/src/github.com/cloudfoundry/hmperformance/benchmarks
go run benchmarks.go NUMBER_OF_APPS

```

Where NUMBER_OF_APPS is an integer number of apps to simulate (clustered etcd begins to break past 200000-30000 apps).

**IMPORTANT: when you rerun the benchmark you must *manuall* nuke the etcd database (don't just restart etcd -- delete its contents)**

### Interpreting the benchmarks

By default the benchmark runs in non-verbose mode and all you see is:

```
TICKING SIMULATOR TOOK: 3.89321ms
~~~~ STORE IS NOT FRESH: 1.005864507s (Actual state is not fresh)
TICKING SIMULATOR TOOK: 8.093903ms
~~~~ STORE IS NOT FRESH: 2.023198996s (Actual state is not fresh)
TICKING SIMULATOR TOOK: 2.847576ms
~~~~ STORE IS NOT FRESH: 3.031770736s (Actual state is not fresh)
TICKING SIMULATOR TOOK: 2.261067ms
~~~~ STORE IS NOT FRESH: 4.035181163s (Actual state is not fresh)
TICKING SIMULATOR TOOK: 7.226324ms
~~~~ STORE IS NOT FRESH: 5.049524367s (Actual state is not fresh)
...
TICKING SIMULATOR TOOK: 3.865164ms
~~~~ STORE IS NOT FRESH: 29.173568156s (Actual state is not fresh)
TICKING SIMULATOR TOOK: 5.243196ms
~~~~ STORE IS FRESH: 30.179737838s
...
```

The main thing the benchmark measures is how long it takes for the ETCD store to be considered "fresh" (the store is fresh if all the running apps have been written to the store succesfully).  This can never be faster than 30 seconds so if the store becomes fresh by 30 seconds the run is considered succesful.

As one increases the number of apps, it becomes harder and harder for the store to attain freshness.  Past 30,000 apps the store becomes fresh *eventually* (after 30 seconds) and stays fresh.  This is good.  Past 100,000 apps the store never becomes fresh.

In particular, we've seen clustered ETCD (on m1.mediums on AWS) fail to attain and maintain freshness for values as low as 20,000-30,000 apps and output looks like this:

```
TICKING SIMULATOR TOOK: 84.359299ms
~~~~ STORE IS NOT FRESH: 1m36.589709933s (Actual state is not fresh)
TICKING SIMULATOR TOOK: 134.765048ms
~~~~ STORE IS NOT FRESH: 1m37.725547732s (0:  ())
TICKING SIMULATOR TOOK: 94.408203ms
~~~~ STORE IS NOT FRESH: 1m38.821741106s (0:  ())
TICKING SIMULATOR TOOK: 123.709909ms
~~~~ STORE IS NOT FRESH: 1m39.947215637s (0:  ())
TICKING SIMULATOR TOOK: 81.027412ms
~~~~ STORE IS NOT FRESH: 1m41.029270728s (0:  ())
```

The (0: ()) is the error coming from the `go-etcd` client library and seems to correlate with an election being held to pick a new etcd leader (?).  This doesn't always happen consistently, but when it does it tends to never recover.

Aside: If you edit `benchmarks.go` you can get *much* more output by setting `const verbose = true` instead of `false`.  This makes the output harder to interpret to the untrained eye ;)
