package main

import (
	"flag"
	"fmt"
	"log/syslog"
	"math/rand"
	"os"
	"time"

	"gitlab.cern.ch/lb-experts/golbd/lbcluster"
	"gitlab.cern.ch/lb-experts/golbd/lbconfig"
)

var (
	// Version number
	// This should be overwritten with `go build -ldflags "-X main.Version='HELLO_THERE'"`
	Version = "head"
	// Release number
	// It should also be overwritten
	Release = "no_release"

	versionFlag    = flag.Bool("version", false, "print lbd version and exit")
	debugFlag      = flag.Bool("debug", false, "set lbd in debug mode")
	startFlag      = flag.Bool("start", false, "start lbd")
	stopFlag       = flag.Bool("stop", false, "stop lbd")
	updateFlag     = flag.Bool("update", false, "update lbd config")
	configFileFlag = flag.String("config", "./load-balancing.conf", "specify configuration file path")
	logFileFlag    = flag.String("log", "./lbd.log", "specify log file path")
	stdoutFlag     = flag.Bool("stdout", false, "send log to stdtout")
)

func main() {
	flag.Parse()
	if *versionFlag {
		fmt.Printf("This is a proof of concept golbd version: %s-%s \n", Version, Release)
		os.Exit(0)
	}
	rand.Seed(time.Now().UTC().UnixNano())
	syslog, e := syslog.New(syslog.LOG_NOTICE, "lbd")

	if e != nil {
		fmt.Printf("Error getting a syslog instance %v\nThe service will only write to the logfile %v\n\n", e, *logFileFlag)
	}
	log := lbcluster.Log{SyslogWriter: syslog, Stdout: *stdoutFlag, DebugFlag: *debugFlag, ToFilePath: *logFileFlag}

	log.Info("Starting lbd")

	//	var sig_hup, sig_term bool
	// installSignalHandler(&sig_hup, &sig_term, &lg)

	config, lbclusters, err := lbconfig.LoadConfig(*configFileFlag, &log)
	if err != nil {
		log.Warning("loadConfig Error: ")
		log.Warning(err.Error())
		os.Exit(1)
	}
	log.Info("Clusters loaded")

	doneChan := make(chan int)
	go watchFile(*configFileFlag, doneChan)
	go sleep(10, doneChan)

	for {
		myValue := <-doneChan
		if myValue == 1 {
			log.Info("Config Changed")
			config, lbclusters, err = lbconfig.LoadConfig(*configFileFlag, &log)
			if err != nil {
				log.Error(fmt.Sprintf("Error getting the clusters (something wrong in %v", configFileFlag))
			}
		} else if myValue == 2 {
			checkAliases(config, log, lbclusters)
		} else {
			log.Error("Got an unexpected value")
		}
	}
	log.Error("The lbd is not supposed to stop")

}
