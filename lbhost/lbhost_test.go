package lbhost_test

import (
	"net"
	"testing"

	"lb-experts/golbd/lbhost"
)

func TestGetLoadForAlias(t *testing.T) {
	tt := map[string]struct {
		host         lbhost.LBHost
		clusterName  string
		expectedLoad int
	}{
		"lxplus132.cern.ch": {
			host:         getHost("lxplus132.cern.ch", 7, ""),
			clusterName:  "lxplus132.cern.ch",
			expectedLoad: 7,
		},
		"lxplus132.cern.ch - load 0": {
			host:         getHost("lxplus132.cern.ch", 0, "blabla.cern.ch=179,blablabla2.cern.ch=4"),
			clusterName:  "lxplus132.cern.ch",
			expectedLoad: 0,
		},
		"blabla.cern.ch": {
			host:         getHost("lxplus132.cern.ch", 179, "blabla.cern.ch=179,blablabla2.cern.ch=4"),
			clusterName:  "blabla.cern.ch",
			expectedLoad: 179,
		},
		"blablabla2.cern.ch": {
			host:         getHost("lxplus132.cern.ch", 4, "blabla.cern.ch=179,blablabla2.cern.ch=4"),
			clusterName:  "blablabla2.cern.ch",
			expectedLoad: 4,
		},
		"toto132.lxplus.cern.ch - 42": {
			host:         getHost("toto132.lxplus.cern.ch", 42, ""),
			clusterName:  "toto132.lxplus.cern.ch",
			expectedLoad: 42,
		},
		"toto132.lxplus.cern.ch - load 0": {
			host:         getHost("toto132.lxplus.cern.ch", 0, "blabla.subdo.cern.ch=179,blablabla2.subdo.cern.ch=4"),
			clusterName:  "toto.subdo.cern.ch",
			expectedLoad: 0,
		},
		"toto.subdo.cern.ch": {
			host:         getHost("toto132.lxplus.cern.ch", 42, ""),
			clusterName:  "toto.subdo.cern.ch",
			expectedLoad: 42,
		},
		"blabla.subdo.cern.ch - 179": {
			host:         getHost("toto132.lxplus.cern.ch", 0, "blabla.subdo.cern.ch=179,blablabla2.subdo.cern.ch=4"),
			clusterName:  "blabla.subdo.cern.ch",
			expectedLoad: 179,
		},
		"blablabla2.subdo.cern.ch": {
			host:         getHost("toto132.lxplus.cern.ch", 0, "blabla.subdo.cern.ch=179,blablabla2.subdo.cern.ch=4"),
			clusterName:  "blablabla2.subdo.cern.ch",
			expectedLoad: 4,
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			underTest := tc.host.GetLoadForAlias(tc.clusterName)
			if underTest != tc.expectedLoad {
				t.Errorf(" got %v, expected %v\n", underTest, tc.expectedLoad)
			}
		})
	}
}

func getHost(hostname string, responseInt int, responseString string) lbhost.LBHost {
	return lbhost.LBHost{ClusterName: "test01.cern.ch",
		HostName: hostname,
		HostTransports: []lbhost.TransportResult{{
			Transport:      "udp",
			ResponseInt:    responseInt,
			ResponseString: responseString,
			IP:             net.ParseIP("188.184.108.98"),
			ResponseError:  "",
		}},
		LoadBalancingUsername: "loadbalancing",
		LoadBalancingPassword: "XXXX",
		LogFile:               "",
		DebugFlag:             false,
	}
}
