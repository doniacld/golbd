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

type TransportResult struct {
	Transport string
	IP        net.IP
	// TODO maybe move this into a struct response
	// TODO Do not see the point of the Reponse int or string?!
	ResponseInt    int
	ResponseString string
	ResponseError  string
}

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

// TODO could we use generics?
func (tr *TransportResult) setPdu(pdu interface{}) {
	//var pduInteger int
	switch t := pdu.(type) {
	case int:
		tr.ResponseInt = pdu.(int)
	case string:
		tr.ResponseString = pdu.(string)
	default:
		tr.ResponseError = fmt.Sprintf("The node returned an unexpected type %s in %v", t, pdu)
	}
}

func (h *LBHost) Log(level string, msg string) error {
	var err error
	if level == log.LevelDebug && !h.DebugFlag {
		//The debug messages should not appear
		return nil
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
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, msg)

	return err
}

func (h *LBHost) GetLoadForAlias(clusterName string) int {
	load := -200
	for _, transport := range h.HostTransports {
		pduInteger := transport.ResponseInt

		// TODO move this regexp compilation as a const
		re := regexp.MustCompile(clusterName + "=([0-9]+)")
		submatch := re.FindStringSubmatch(transport.ResponseString)

		if submatch != nil {
			// TODO handle the error
			pduInteger, _ = strconv.Atoi(submatch[1])
		}

		if (pduInteger > 0 && pduInteger < load) || (load < 0) {
			load = pduInteger
		}
		h.Log(log.LevelDebug, fmt.Sprintf("Possible load is %v", pduInteger))

	}
	h.Log(log.LevelDebug, fmt.Sprintf("THE LOAD IS %v, ", load))

	return load
}

func (h *LBHost) GetWorkingIps() ([]net.IP, error) {
	var myIps []net.IP
	for _, myTransport := range h.HostTransports {
		if (myTransport.ResponseInt > 0) && (myTransport.ResponseError == "") {
			myIps = append(myIps, myTransport.IP)
		}

	}
	h.Log(log.LevelInfo, fmt.Sprintf("The ips for this host are %v", myIps))
	return myIps, nil
}

func (h *LBHost) GetAllIps() ([]net.IP, error) {
	var myIps []net.IP
	for _, myTransport := range h.HostTransports {
		myIps = append(myIps, myTransport.IP)
	}
	h.Log(log.LevelInfo, fmt.Sprintf("All ips for this host are %v", myIps))
	return myIps, nil
}

func (h *LBHost) GetIps() ([]net.IP, error) {
	var ips []net.IP
	var err error

	re := regexp.MustCompile(".*no such host")

	net.DefaultResolver.StrictErrors = true

	for i := 0; i < 3; i++ {
		h.Log(log.LevelInfo, "Getting the ip addresses")
		ips, err = net.LookupIP(h.HostName)
		if err == nil {
			return ips, nil
		}
		h.Log(log.LevelWarning, fmt.Sprintf("LookupIP: %v has incorrect or missing IP address (%v) ", h.HostName, err))
		submatch := re.FindStringSubmatch(err.Error())
		if submatch != nil {
			h.Log(log.LevelInfo, "There is no need to retry this error")
			return nil, err
		}
	}

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
