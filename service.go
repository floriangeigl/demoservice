package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type errorAnomalyConfig struct {
	ResponseCode int
	Count        int
}

type slowdownAnomalyConfig struct {
	SlowdownMillis int
	Count          int
}

type crashAnomalyConfig struct {
	Code int
}

type resourceAnomalyConfig struct {
	Severity int
	Count    int
}

type callee struct {
	Adr   string // URL address to call
	Count int    // number of calls per minute
}

type config struct {
	ErrorConfig    errorAnomalyConfig
	SlowdownConfig slowdownAnomalyConfig
	CrashConfig    crashAnomalyConfig
	ResourceConfig resourceAnomalyConfig
	Callees        []callee
	Balanced       bool
	Proxy          bool
}

var conf config
var reqcount int

func readEnvConfig() {

	conf.ErrorConfig.ResponseCode, _ = strconv.Atoi(os.Getenv("ErrorConfig_ResponseCode"))
	conf.ErrorConfig.Count, _ = strconv.Atoi(os.Getenv("ErrorConfig_Count"))

	conf.SlowdownConfig.SlowdownMillis, _ = strconv.Atoi(os.Getenv("SlowdownConfig_SlowdownMillis"))
	conf.SlowdownConfig.Count, _ = strconv.Atoi(os.Getenv("SlowdownConfig_Count"))

	conf.CrashConfig.Code, _ = strconv.Atoi(os.Getenv("CrashConfig_Code"))

	conf.ResourceConfig.Severity, _ = strconv.Atoi(os.Getenv("ResourceConfig_Severity"))
	conf.ResourceConfig.Count, _ = strconv.Atoi(os.Getenv("ResourceConfig_Count"))

	var callee_adr = strings.Split(os.Getenv("Callees_Adr"), ",")
	var callee_count = strings.Split(os.Getenv("Callees_Count"), ",")

	if len(callee_adr) > 0 {
		callee_array := make([]callee, len(callee_adr))	

		for i := range callee_adr {
			callee_array[i].Adr = callee_adr[i]
			callee_array[i].Count, _ = strconv.Atoi(callee_count[i])
		}
		conf.Callees = callee_array
	}
	// os.Getenv(service.callee.Balanced)
	// os.Getenv(service.callee.Proxy)
}

func receiveConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		w.WriteHeader(http.StatusNoContent)
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		//fmt.Printf(string(body))
		err = json.Unmarshal(body, &conf)
		if err != nil {
			fmt.Printf("config payload wrong")
			log.Printf("%s config payload is wrong", os.Args[0])
			panic(err)
		}
		log.Printf("%s received a new service config deployment",  os.Args[0])
	default:
		fmt.Fprintf(w, "sorry, only POST method is supported.")
	}
	defer r.Body.Close()
}

func handleIcon(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "favicon.ico")
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()
}


func sayHello(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	reqcount++
	fmt.Fprintf(&buf, "it's the %d call\n", reqcount)
	fmt.Fprintf(&buf, "what I did:\n")
	// first call all callees we have in the config with the multiplicity given
	failures := false

	for ci, element := range conf.Callees {
		if !conf.Balanced || reqcount%len(conf.Callees) == ci {
			for i := 0; i < element.Count; i++ {
				req, err := http.NewRequest("GET", element.Adr, nil)
				if err != nil {
					// os.Args[0] to get the current exe name
					log.Printf("%s error reading request.", os.Args[0])
					os.Exit(1)
				}
				if conf.Proxy {
					log.Printf("%s dt header: %s ", os.Args[0], r.Header.Get("X-Dynatrace"))
					log.Printf("%s RemoteAddr: %s ", os.Args[0], r.RemoteAddr)
					req.Header.Set("X-Dynatrace", r.Header.Get("X-Dynatrace"))
					req.Header.Set("x-forwarded-for", r.RemoteAddr)
					req.Header.Set("forwarded", r.RemoteAddr)
				}
				req.Header.Set("Cache-Control", "no-cache")

				client := &http.Client{Timeout: time.Second * 10}

				resp, err := client.Do(req)
				if err != nil {
					log.Printf("%s error reading response.", os.Args[0])
					os.Exit(1)
				} else {
					if resp.StatusCode != 200 {
						log.Printf("%s got a bad return", os.Args[0])
						failures = true
					}
				}
				defer resp.Body.Close()
			}
			fmt.Fprintf(&buf, "called %s %d times\n", element.Adr, element.Count)
		}
	}
	// then check if we should crash the process
	if conf.CrashConfig.Code != 0 {
		log.Printf("%s cashed.", os.Args[0])
		panic("a problem")
		//os.Exit(conf.CrashConfig.Code)
	}
	// then check if we should add a delay
	if conf.SlowdownConfig.SlowdownMillis != 0 && conf.SlowdownConfig.Count > 0 {
		time.Sleep(time.Duration(conf.SlowdownConfig.SlowdownMillis) * time.Millisecond)
		conf.SlowdownConfig.Count = conf.SlowdownConfig.Count - 1
		fmt.Fprintf(&buf, "sleeped for %d millis\n", conf.SlowdownConfig.SlowdownMillis)
	}
	// then check if we should increase resource consumption
	if conf.ResourceConfig.Severity != 0 && conf.ResourceConfig.Count > 0 {
		for c := 0; c <= conf.ResourceConfig.Severity; c++ {
			m1 := [100][100]int{}
			for i := 0; i < 100; i++ {
				for j := 0; j < 100; j++ {
					m1[i][j] = rand.Int()
				}
			}
		}
		fmt.Fprintf(&buf, "allocated %d 100x100 matrices with random values\n", conf.ResourceConfig.Severity)
		conf.ResourceConfig.Count = conf.ResourceConfig.Count - 1
		log.Printf("%s high resource consumption service call", os.Args[0])
	}
	// then check if the should return an error response code
	if failures || (conf.ErrorConfig.ResponseCode != 0 && conf.ErrorConfig.Count > 0) {
		if conf.ErrorConfig.ResponseCode == 400 {
			w.WriteHeader(http.StatusForbidden)
			promHttpResponses.With(prometheus.Labels{"code":"400"}).Inc()
			promHttpFailures.Inc()
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			promHttpResponses.With(prometheus.Labels{"code":"500"}).Inc()
			promHttpFailures.Inc()
		}
		conf.ErrorConfig.Count = conf.ErrorConfig.Count - 1
		
	} else {
		message := r.URL.Path
		message = strings.TrimPrefix(message, "/")
		message = "finally returned " + message
		w.Write([]byte(message))
		promHttpResponses.With(prometheus.Labels{"code":"200"}).Inc()
	}
	defer r.Body.Close()
}

var (
	promHttpResponses = promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "demoservice_http_responses",
			Help: "The total number of http responses.",
		},
		[]string{"code"},
	)
	promHttpFailures = promauto.NewCounter(prometheus.CounterOpts{
			Name: "demoservice_http_failures",
			Help: "The total number of http failures.",
		},
	)
)

func main() {
	port := 8080
	if len(os.Args) > 1 {
		arg := os.Args[1]
		fmt.Printf("Start demo service at port: %s\n", arg)
		i1, err := strconv.Atoi(arg)
		if err == nil {
			port = i1
		}
	} else {
		fmt.Printf("Start demo service at default port: %d\n", port)
	}
	readEnvConfig()

	http.HandleFunc("/", sayHello)
	http.HandleFunc("/favicon.ico", handleIcon)
	http.HandleFunc("/config", receiveConfig)
	http.Handle("/metrics", promhttp.Handler()) 
	http.HandleFunc("/healthz", healthz)
	log.Printf("Start demo service at default port: %d\n", port)
	if err := http.ListenAndServe(":"+strconv.Itoa(port), nil); err != nil {
		panic(err)
	}
}
