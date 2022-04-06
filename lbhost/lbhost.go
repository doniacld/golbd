package lbhost

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/reguero/go-snmplib"
	"lb-experts/golbd/log"
)

const (
	creationSNMPTimeout int    = 10
	OID                 string = ".1.3.6.1.4.1.96.255.1"
)

type LBHost struct {
	ClusterName           string
	HostName              string
	HostTransports        []TransportResult
	LoadBalancingUsername string
	LoadBalancingPassword string
	LogFile               string
	logMu                 sync.Mutex // TODO why this is not exposed
	DebugFlag             bool
}

func (h *LBHost) SnmpReq() {
	h.findTransports()

	for i, hostTransport := range h.HostTransports {
		hostTransport.ResponseInt = 100000 // TODO why ?
		transport := hostTransport.Transport
		nodeIp := hostTransport.IP.String()

		// There is no need to put square brackets around the ipv6 addresses.
		h.Log(log.LevelDebug, fmt.Sprintf("checking the host %s with %s", nodeIp, transport))
		h.HostTransports[i] = h.newSNMP(hostTransport)
	}

	h.Log(log.LevelDebug, "All the ips have been tested")

	// TODO is it for debugging?
	/*for _, my_transport := range self.Host_transports {
		self.Write_to_log(lbcluster.LevelInfo, fmt.Sprintf("%v", my_transport))
	}*/
}

func (h *LBHost) newSNMP(tr TransportResult) TransportResult {
	transport := tr.Transport
	nodeIP := tr.IP.String()

	snmp, err := snmplib.NewSNMPv3(nodeIP, h.LoadBalancingUsername, snmplib.SnmpMD5, h.LoadBalancingPassword, snmplib.SnmpNOPRIV, h.LoadBalancingPassword,
		time.Duration(creationSNMPTimeout)*time.Second, 2)
	if err != nil {
		// Failed to create snmpgo.SNMP object
		tr.ResponseError = fmt.Sprintf("contacted node: error creating the snmp object: %v", err)
	} else {
		// TODO defer in a for loop
		defer snmp.Close()
		err = snmp.Discover()

		if err != nil {
			tr.ResponseError = fmt.Sprintf("contacted node: error in the snmp discovery: %v", err)
		} else {
			oid, err := snmplib.ParseOid(OID)
			if err != nil {
				// Failed to parse Oids
				tr.ResponseError = fmt.Sprintf("contacted node: Error parsing the OID %v", err)
			} else {
				pdu, err := snmp.GetV3(oid)
				if err != nil {
					tr.ResponseError = fmt.Sprintf("contacted node: The getv3 gave the following error: %v ", err)
				} else {
					h.Log(log.LevelInfo, fmt.Sprintf("contacted node: transport: %v ip: %v - reply was %v", transport, nodeIP, pdu))

					tr.setPdu(pdu)
				}
			}
		}
	}
	return tr
}

// Log logs in the given file the message depending on the level
func (h *LBHost) Log(level string, msg string) {
	if level == log.LevelDebug && !h.DebugFlag {
		// The debug messages should not appear
		return
	}
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}

	timestamp := time.Now().Format(time.StampMilli)
	msg = fmt.Sprintf("%s lbd[%d]: %s: cluster: %s node: %s %s", timestamp, os.Getpid(), level, h.ClusterName, h.HostName, msg)

	h.logMu.Lock()
	defer h.logMu.Unlock()

	f, err := os.OpenFile(h.LogFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0640)
	if err != nil {
		fmt.Printf("error while opening file %s: %s\n", h.LogFile, err.Error())
		return
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, msg)
	if err != nil {
		fmt.Printf("error while writing in file %s: %s\n", f.Name(), err.Error())
		return
	}
}

// GetLoadForAlias retrieves the load from the alias of the DNS
// e.g.: lxplus.cern.ch=179 ; load=179
func (h *LBHost) GetLoadForAlias(clusterName string) int {
	// TODO why the load is equal to this magic number?
	load := -200
	for _, transport := range h.HostTransports {
		pduInteger := transport.ResponseInt

		re := regexp.MustCompile(clusterName + "=([0-9]+)")
		submatch := re.FindStringSubmatch(transport.ResponseString)

		var err error
		if submatch != nil {
			pduInteger, err = strconv.Atoi(submatch[1])
			if err != nil {
				h.Log(log.LevelDebug, fmt.Sprintf("error while converting %v to int", submatch[1]))
			}
		}

		if (pduInteger > 0 && pduInteger < load) || (load < 0) {
			load = pduInteger
		}
		h.Log(log.LevelDebug, fmt.Sprintf("possible load is %v", pduInteger))
	}

	h.Log(log.LevelDebug, fmt.Sprintf("the load is %v, ", load))

	return load
}

// GetWorkingIps returns valid ips which means they have a response and no error
func (h *LBHost) GetWorkingIps() []net.IP {
	ips := make([]net.IP, 0)
	for _, ht := range h.HostTransports {
		if (ht.ResponseInt > 0) && (ht.ResponseError == "") {
			ips = append(ips, ht.IP)
		}
	}
	h.Log(log.LevelInfo, fmt.Sprintf("The ips for this host are %v", ips))

	return ips
}

// GetAllIps retrieves all the IPs from the host transports
func (h *LBHost) GetAllIps() []net.IP {
	ips := make([]net.IP, 0)
	for _, ht := range h.HostTransports {
		ips = append(ips, ht.IP)
	}
	h.Log(log.LevelInfo, fmt.Sprintf("All ips for this host are %v", ips))

	return ips
}

var (
	// TODO Not sure if it has a real interest
	noHostPattern = regexp.MustCompile(".*no such host")
)

// GetIps returns a list of available IPs
// TODO not sure it should still be exposed since it is not used anymore in lbcluster
func (h *LBHost) GetIps() ([]net.IP, error) {
	ips := make([]net.IP, 0)
	var err error

	net.DefaultResolver.StrictErrors = true

	// TODO Why 3 ? It is the number of retries
	// TODO Should be as configuration but at least a constant
	// Lookup IPs
	for i := 0; i < 3; i++ {
		h.Log(log.LevelInfo, "Getting the ip addresses")
		ips, err = net.LookupIP(h.HostName)
		if err == nil {
			return ips, nil
		}

		h.Log(log.LevelWarning, fmt.Sprintf("LookupIP: %s has incorrect or missing IP address (%s) ", h.HostName, err.Error()))

		submatch := noHostPattern.FindStringSubmatch(err.Error())
		if submatch != nil {
			h.Log(log.LevelInfo, "There is no need to retry this error")
			return nil, err
		}
	}

	// Handle the failure
	h.Log(log.LevelError, "After several retries, we couldn't get the ips!. Let's try with partial results")
	net.DefaultResolver.StrictErrors = false

	ips, err = net.LookupIP(h.HostName)
	if err != nil {
		h.Log(log.LevelError, fmt.Sprintf("It didn't work :(. This node will be ignored during this evaluation: %v", err))
	}

	return ips, err
}

func (h *LBHost) findTransports() {
	h.Log(log.LevelDebug, "Let's find the ips behind this host")

	ips, _ := h.GetIps()
	for _, ip := range ips {
		transport := "udp"
		// If there is an IPv6 address use udp6 transport
		if ip.To4() == nil {
			transport = "udp6"
		}
		h.HostTransports = append(h.HostTransports, TransportResult{Transport: transport,
			ResponseInt: 100000, ResponseString: "", IP: ip,
			ResponseError: ""})
	}
}
