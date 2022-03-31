package main_test

import (
	"net"
	"reflect"
	"testing"

	"lb-experts/golbd/lbcluster"
	"lb-experts/golbd/lbhost"
	"lb-experts/golbd/log"
)

func TestGetListHostsOne(t *testing.T) {
	c := getTestCluster("test01.cern.ch")

	expected := map[string]lbhost.LBHost{
		"lxplus041.cern.ch": lbhost.LBHost{ClusterName: c.ClusterName,
			HostName:              "lxplus041.cern.ch",
			LoadBalancingUsername: c.LoadBalancingUsername,
			LoadBalancingPassword: c.LoadBalancingPassword,
			LogFile:               c.Slog.ToFilePath,
			DebugFlag:             c.Slog.DebugFlag,
		},
		"monit-kafkax-17be060b0d.cern.ch": lbhost.LBHost{ClusterName: c.ClusterName,
			HostName:              "monit-kafkax-17be060b0d.cern.ch",
			LoadBalancingUsername: c.LoadBalancingUsername,
			LoadBalancingPassword: c.LoadBalancingPassword,
			LogFile:               c.Slog.ToFilePath,
			DebugFlag:             c.Slog.DebugFlag,
		},
		"lxplus132.cern.ch": lbhost.LBHost{ClusterName: c.ClusterName,
			HostName:              "lxplus132.cern.ch",
			LoadBalancingUsername: c.LoadBalancingUsername,
			LoadBalancingPassword: c.LoadBalancingPassword,
			LogFile:               c.Slog.ToFilePath,
			DebugFlag:             c.Slog.DebugFlag,
		},
		"lxplus133.subdo.cern.ch": lbhost.LBHost{ClusterName: c.ClusterName,
			HostName:              "lxplus133.subdo.cern.ch",
			LoadBalancingUsername: c.LoadBalancingUsername,
			LoadBalancingPassword: c.LoadBalancingPassword,
			LogFile:               c.Slog.ToFilePath,
			DebugFlag:             c.Slog.DebugFlag,
		},
		"lxplus130.cern.ch": lbhost.LBHost{ClusterName: c.ClusterName,
			HostName:              "lxplus130.cern.ch",
			LoadBalancingUsername: c.LoadBalancingUsername,
			LoadBalancingPassword: c.LoadBalancingPassword,
			LogFile:               c.Slog.ToFilePath,
			DebugFlag:             c.Slog.DebugFlag,
		},
	}

	hosts_to_check := make(map[string]lbhost.LBHost)
	c.GetListHosts(hosts_to_check)
	if !reflect.DeepEqual(hosts_to_check, expected) {
		t.Errorf("e.Get_list_hosts: got\n%v\nexpected\n%v", hosts_to_check, expected)
	}
}

func TestGetListHostsTwo(t *testing.T) {
	lg := log.Log{Stdout: true, DebugFlag: false}

	clusters := []lbcluster.LBCluster{
		{ClusterName: "test01.cern.ch",
			LoadBalancingUsername: "loadbalancing",
			LoadBalancingPassword: "zzz123",
			HostMetricTable:       map[string]lbcluster.Node{"lxplus142.cern.ch": lbcluster.Node{}, "lxplus177.cern.ch": lbcluster.Node{}},
			Parameters:            lbcluster.Params{Behaviour: "mindless", BestHosts: 2, External: true, Metric: "cmsfrontier", PollingInterval: 6, Statistics: "long"},
			//Time_of_last_evaluation time.Time
			CurrentBestIPs: []net.IP{},

			PreviousBestIPsDNS: []net.IP{},
			Slog:               &lg,
			CurrentIndex:       0},
		lbcluster.LBCluster{ClusterName: "test02.cern.ch",
			LoadBalancingUsername: "loadbalancing",
			LoadBalancingPassword: "zzz123",
			HostMetricTable:       map[string]lbcluster.Node{"lxplus013.cern.ch": lbcluster.Node{}, "lxplus177.cern.ch": lbcluster.Node{}, "lxplus025.cern.ch": lbcluster.Node{}},
			Parameters:            lbcluster.Params{Behaviour: "mindless", BestHosts: 10, External: false, Metric: "cmsfrontier", PollingInterval: 6, Statistics: "long"},
			//Time_of_last_evaluation time.Time
			CurrentBestIPs:     []net.IP{},
			PreviousBestIPsDNS: []net.IP{},
			Slog:               &lg,
			CurrentIndex:       0}}

	expected := map[string]lbhost.LBHost{
		"lxplus142.cern.ch": lbhost.LBHost{ClusterName: "test01.cern.ch",
			HostName:              "lxplus142.cern.ch",
			LoadBalancingUsername: "loadbalancing",
			LoadBalancingPassword: "zzz123",
			LogFile:               "",
			DebugFlag:             false,
		},
		"lxplus177.cern.ch": lbhost.LBHost{ClusterName: "test01.cern.ch,test02.cern.ch",
			HostName:              "lxplus177.cern.ch",
			LoadBalancingUsername: "loadbalancing",
			LoadBalancingPassword: "zzz123",
			LogFile:               "",
			DebugFlag:             false,
		},
		"lxplus013.cern.ch": lbhost.LBHost{ClusterName: "test02.cern.ch",
			HostName:              "lxplus013.cern.ch",
			LoadBalancingUsername: "loadbalancing",
			LoadBalancingPassword: "zzz123",
			LogFile:               "",
			DebugFlag:             false,
		},
		"lxplus025.cern.ch": lbhost.LBHost{ClusterName: "test02.cern.ch",
			HostName:              "lxplus025.cern.ch",
			LoadBalancingUsername: "loadbalancing",
			LoadBalancingPassword: "zzz123",
			LogFile:               "",
			DebugFlag:             false,
		},
	}

	hosts_to_check := make(map[string]lbhost.LBHost)
	for _, c := range clusters {
		c.GetListHosts(hosts_to_check)
	}
	if !reflect.DeepEqual(hosts_to_check, expected) {
		t.Errorf("e.Get_list_hosts: got\n%v\nexpected\n%v", hosts_to_check, expected)
	}
}
