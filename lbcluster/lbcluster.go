package lbcluster

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"lb-experts/golbd/log"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"lb-experts/golbd/lbhost"
)

// WorstValue worst possible load
const WorstValue int = 99999

// TIMEOUT snmp timeout
const TIMEOUT int = 10

// OID snmp object to get
const OID string = ".1.3.6.1.4.1.96.255.1"

// LBCluster struct of an lbcluster alias
type LBCluster struct {
	ClusterName           string
	LoadBalancingUsername string
	LoadBalancingPassword string
	HostMetricTable       map[string]Node
	Parameters            Params
	TimeOfLastEvaluation  time.Time
	CurrentBestIps        []net.IP
	PreviousBestIpsDns    []net.IP
	CurrentIndex          int
	Slog                  *log.Log
}

// Params of the alias
type Params struct {
	Behaviour       string
	BestHosts       int
	External        bool
	Metric          string
	PollingInterval int
	Statistics      string
	Ttl             int
}

// Shuffle pseudo-randomizes the order of elements.
// n is the number of elements. Shuffle panics if n < 0.
// swap swaps the elements with indexes i and j.
func Shuffle(n int, swap func(i, j int)) {
	if n < 0 {
		panic("invalid argument to Shuffle")
	}

	// Fisher-Yates shuffle: https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle
	// Shuffle really ought not be called with n that doesn't fit in 32 bits.
	// Not only will it take a very long time, but with 2³¹! possible permutations,
	// there's no way that any PRNG can have a big enough internal state to
	// generate even a minuscule percentage of the possible permutations.
	// Nevertheless, the right API signature accepts an int n, so handle it as best we can.
	i := n - 1
	for ; i > 1<<31-1-1; i-- {
		j := int(rand.Int63n(int64(i + 1)))
		swap(i, j)
	}
	for ; i > 0; i-- {
		j := int(rand.Int31n(int32(i + 1)))
		swap(i, j)
	}
}

// Node Struct to keep the ips and load of a node for an alias
type Node struct {
	Load int
	IPs  []net.IP
}

// NodeList struct for the list
type NodeList []Node

func (p NodeList) Len() int           { return len(p) }
func (p NodeList) Less(i, j int) bool { return p[i].Load < p[j].Load }
func (p NodeList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// TimeToRefresh Checks if the cluster needs refreshing
func (lbc *LBCluster) TimeToRefresh() bool {
	return lbc.TimeOfLastEvaluation.Add(time.Duration(lbc.Parameters.PollingInterval) * time.Second).Before(time.Now())
}

// GetListHosts Get the hosts for an alias
func (lbc *LBCluster) GetListHosts(current_list map[string]lbhost.LBHost) {
	lbc.WriteToLog(log.LevelDebug, "Getting the list of hosts for the alias")
	for host := range lbc.HostMetricTable {
		myHost, ok := current_list[host]
		if ok {
			myHost.ClusterName = myHost.ClusterName + "," + lbc.ClusterName
		} else {
			myHost = lbhost.LBHost{
				ClusterName:           lbc.ClusterName,
				HostName:              host,
				LoadBalancingUsername: lbc.LoadBalancingUsername,
				LoadBalancingPassword: lbc.LoadBalancingPassword,
				LogFile:               lbc.Slog.ToFilePath,
				DebugFlag:             lbc.Slog.DebugFlag,
			}
		}
		current_list[host] = myHost
	}
}

func (lbc *LBCluster) concatenateNodes(myNodes []Node) string {
	nodes := make([]string, 0, len(myNodes))
	for _, node := range myNodes {
		nodes = append(nodes, lbc.concatenateIps(node.IPs))
	}
	return strings.Join(nodes, " ")
}

func (lbc *LBCluster) concatenateIps(myIps []net.IP) string {
	ipString := make([]string, 0, len(myIps))

	for _, ip := range myIps {
		ipString = append(ipString, ip.String())
	}

	sort.Strings(ipString)
	return strings.Join(ipString, " ")
}

// FindBestHosts Looks for the best hosts for a cluster
func (lbc *LBCluster) FindBestHosts(hostsToCheck map[string]lbhost.LBHost) bool {

	lbc.EvaluateHosts(hostsToCheck)
	allMetrics := make(map[string]bool)
	allMetrics["minimum"] = true
	allMetrics["cmsfrontier"] = true
	allMetrics["minino"] = true

	_, ok := allMetrics[lbc.Parameters.Metric]
	if !ok {
		lbc.WriteToLog(log.LevelError, "wrong parameter(metric) in definition of cluster "+lbc.Parameters.Metric)
		return false
	}
	lbc.TimeOfLastEvaluation = time.Now()
	if !lbc.ApplyMetric(hostsToCheck) {
		return false
	}
	nodes := lbc.concatenateIps(lbc.CurrentBestIps)
	if len(lbc.CurrentBestIps) == 0 {
		nodes = "NONE"
	}
	lbc.WriteToLog(log.LevelInfo, "best hosts are: "+nodes)
	return true
}

// ApplyMetric This is the core of the lbcluster: based on the metrics, select the best hosts
func (lbc *LBCluster) ApplyMetric(hostsToCheck map[string]lbhost.LBHost) bool {
	lbc.WriteToLog(log.LevelInfo, "Got metric = "+lbc.Parameters.Metric)
	pl := make(NodeList, len(lbc.HostMetricTable))
	i := 0
	for _, v := range lbc.HostMetricTable {
		pl[i] = v
		i++
	}
	//Let's shuffle the hosts before sorting them, in case some hosts have the same value
	Shuffle(len(pl), func(i, j int) { pl[i], pl[j] = pl[j], pl[i] })
	sort.Sort(pl)
	lbc.WriteToLog(log.LevelDebug, fmt.Sprintf("%v", pl))
	var sortedHostList []Node
	var usefulHostList []Node
	for _, v := range pl {
		if (v.Load > 0) && (v.Load <= WorstValue) {
			usefulHostList = append(usefulHostList, v)
		}
		sortedHostList = append(sortedHostList, v)
	}
	lbc.WriteToLog(log.LevelDebug, fmt.Sprintf("%v", usefulHostList))
	usefulHosts := len(usefulHostList)
	listLength := len(pl)
	max := lbc.Parameters.BestHosts
	if max == -1 {
		max = listLength
	}
	if max > listLength {
		lbc.WriteToLog(log.LevelWarning, fmt.Sprintf("impossible to return %v hosts from the list of %v hosts (%v). Check the configuration of cluster. Returning %v hosts.",
			max, listLength, lbc.concatenateNodes(sortedHostList), listLength))
		max = listLength
	}
	lbc.CurrentBestIps = []net.IP{}
	if listLength == 0 {
		lbc.WriteToLog(log.LevelError, "cluster has no hosts defined ! Check the configuration.")
	} else if usefulHosts == 0 {

		if lbc.Parameters.Metric == "minimum" {
			lbc.WriteToLog(log.LevelWarning, fmt.Sprintf("no usable hosts found for cluster! Returning random %v hosts.", max))
			//Get hosts with all IPs even when not OK for SNMP
			lbc.ReEvaluateHostsForMinimum(hostsToCheck)
			i := 0
			for _, v := range lbc.HostMetricTable {
				pl[i] = v
				i++
			}
			//Let's shuffle the hosts
			Shuffle(len(pl), func(i, j int) { pl[i], pl[j] = pl[j], pl[i] })
			for i := 0; i < max; i++ {
				lbc.CurrentBestIps = append(lbc.CurrentBestIps, pl[i].IPs...)
			}
			lbc.WriteToLog(log.LevelWarning, fmt.Sprintf("We have put random hosts behind the alias: %v", lbc.CurrentBestIps))

		} else if (lbc.Parameters.Metric == "minino") || (lbc.Parameters.Metric == "cmsweb") {
			lbc.WriteToLog(log.LevelWarning, "no usable hosts found for cluster! Returning no hosts.")
		} else if lbc.Parameters.Metric == "cmsfrontier" {
			lbc.WriteToLog(log.LevelWarning, "no usable hosts found for cluster! Skipping the DNS update")
			return false
		}
	} else {
		if usefulHosts < max {
			lbc.WriteToLog(log.LevelWarning, fmt.Sprintf("only %v useable hosts found in cluster", usefulHosts))
			max = usefulHosts
		}
		for i := 0; i < max; i++ {
			lbc.CurrentBestIps = append(lbc.CurrentBestIps, usefulHostList[i].IPs...)
		}
	}

	return true
}

// NewTimeoutClient checks the timeout
// The following functions are for the roger state and its timeout
func NewTimeoutClient(connectTimeout time.Duration, readWriteTimeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: timeoutDialer(connectTimeout, readWriteTimeout),
		},
	}
}

func timeoutDialer(cTimeout time.Duration, rwTimeout time.Duration) func(net, addr string) (c net.Conn, err error) {
	return func(netw, addr string) (net.Conn, error) {
		conn, err := net.DialTimeout(netw, addr, cTimeout)
		if err != nil {
			return nil, err
		}
		conn.SetDeadline(time.Now().Add(rwTimeout))
		return conn, nil
	}
}

func (lbc *LBCluster) checkRogerState(host string) string {
	logmessage := ""

	connectTimeout := 10 * time.Second
	readWriteTimeout := 20 * time.Second
	httpClient := NewTimeoutClient(connectTimeout, readWriteTimeout)
	response, err := httpClient.Get("http://woger-direct.cern.ch:9098/roger/v1/state/" + host)
	if err != nil {
		logmessage = logmessage + fmt.Sprintf("%s", err)
	} else {
		defer response.Body.Close()
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			logmessage = logmessage + fmt.Sprintf("%s", err)
		}
		var dat map[string]interface{}
		if err := json.Unmarshal([]byte(contents), &dat); err != nil {
			logmessage = logmessage + " - " + fmt.Sprintf("%s", host)
			logmessage = logmessage + " - " + fmt.Sprintf("%v", response.Body)
			logmessage = logmessage + " - " + fmt.Sprintf("%v", err)
		}
		if str, ok := dat["appstate"].(string); ok {
			if str != "production" {
				return fmt.Sprintf("node: %s - %s - setting reply -99", host, str)
			}
		} else {
			logmessage = logmessage + fmt.Sprintf("dat[\"appstate\"] not a string for node %s", host)
		}
	}
	return logmessage

}

// EvaluateHosts gets the load from the all the nodes
func (lbc *LBCluster) EvaluateHosts(hostsToCheck map[string]lbhost.LBHost) {

	for currenthost := range lbc.HostMetricTable {
		host := hostsToCheck[currenthost]
		ips := host.GetWorkingIps()
		// TODO since GetWorkingIps does not return an error anymore
		// always call GetIps, do something with the error
		ips, _ = host.GetIps()
		lbc.HostMetricTable[currenthost] = Node{host.GetLoadForAlias(lbc.ClusterName), ips}
		lbc.WriteToLog(log.LevelDebug, fmt.Sprintf("node: %s It has a load of %d", currenthost, lbc.HostMetricTable[currenthost].Load))
	}
}

//ReEvaluateHostsForMinimum gets the load from the all the nodes for Minimum metric policy
func (lbc *LBCluster) ReEvaluateHostsForMinimum(hostsToCheck map[string]lbhost.LBHost) {
	for currenthost := range lbc.HostMetricTable {
		host := hostsToCheck[currenthost]
		ips := host.GetAllIps()
		// TODO since GetWorkingIps does not return an error anymore
		// always call GetIps, do something with the error
		ips, _ = host.GetIps()

		lbc.HostMetricTable[currenthost] = Node{host.GetLoadForAlias(lbc.ClusterName), ips}
		lbc.WriteToLog(log.LevelDebug, fmt.Sprintf("node: %s It has a load of %d", currenthost, lbc.HostMetricTable[currenthost].Load))
	}
}
