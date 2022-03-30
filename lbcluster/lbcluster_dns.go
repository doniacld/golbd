package lbcluster

import (
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"

	"lb-experts/golbd/log"
)

// Logger struct for the Logger interface
type Logger interface {
	Info(s string) error
	Warning(s string) error
	Debug(s string) error
	Error(s string) error
}

// WriteToLog puts something in the log file
func (lbc *LBCluster) WriteToLog(level string, input string) error {
	msg := fmt.Sprintf("cluster: %s, %s", lbc.ClusterName, input)

	switch level {
	case log.LevelInfo:
		return lbc.Slog.Info(msg)
	case log.LevelDebug:
		return lbc.Slog.Debug(msg)
	case log.LevelWarning:
		return lbc.Slog.Warning(msg)
	case log.LevelError:
		return lbc.Slog.Error(msg)
	default:
		return lbc.Slog.Error(fmt.Sprintf("unsupported level %s, assuming error %s", input, msg))
	}
}

// RefreshDNS This is the only public function here. It retrieves the current ips behind the dns,
// and then updates it with the new best ips (if they are different).
func (lbc *LBCluster) RefreshDNS(dnsManager, keyPrefix, internalKey, externalKey string) {
	err := lbc.GetStateDNS(dnsManager)
	if err != nil {
		lbc.WriteToLog(log.LevelWarning, fmt.Sprintf("GetStateDNS Error: %v", err.Error()))
	}

	pbiDNS := lbc.concatenateIps(lbc.PreviousBestIpsDns)
	cbi := lbc.concatenateIps(lbc.CurrentBestIps)
	if pbiDNS == cbi {
		lbc.WriteToLog(log.LevelInfo, fmt.Sprintf("DNS not update keyName %v cbh == pbhDns == %v", keyPrefix, cbi))
		return
	}

	lbc.WriteToLog(log.LevelInfo, fmt.Sprintf("Updating the DNS with %v (previous state was %v)", cbi, pbiDNS))

	err = lbc.updateDNS(keyPrefix+"internal.", internalKey, dnsManager)
	if err != nil {
		lbc.WriteToLog(log.LevelWarning, fmt.Sprintf("Internal updateDNS Error: %v", err.Error()))
	}
	if lbc.externallyVisible() {
		err = lbc.updateDNS(keyPrefix+"external.", externalKey, dnsManager)
		if err != nil {
			lbc.WriteToLog(log.LevelWarning, fmt.Sprintf("External updateDNS Error: %v", err.Error()))
		}
	}
}

func (lbc *LBCluster) externallyVisible() bool {
	return lbc.Parameters.External
}

const (
	defaultTTL = 60
)

func (lbc *LBCluster) updateDNS(keyName, tsigKey, dnsManager string) error {
	ttl := defaultTTL
	if lbc.Parameters.Ttl > defaultTTL {
		ttl = lbc.Parameters.Ttl
	}

	m := new(dns.Msg)
	m.SetUpdate(lbc.ClusterName + ".")
	m.Id = 1234
	rrRemoveA, _ := dns.NewRR(fmt.Sprintf("%s. %d IN A 127.0.0.1", lbc.ClusterName, ttl))
	rrRemoveAAAA, _ := dns.NewRR(fmt.Sprintf("%s. %d IN AAAA ::1", lbc.ClusterName, ttl))
	m.RemoveRRset([]dns.RR{rrRemoveA})
	m.RemoveRRset([]dns.RR{rrRemoveAAAA})

	for _, ip := range lbc.CurrentBestIps {
		var rrInsert dns.RR
		if ip.To4() != nil {
			rrInsert, _ = dns.NewRR(fmt.Sprintf("%s. %d IN A %s", lbc.ClusterName, ttl, ip.String()))
		} else if ip.To16() != nil {
			rrInsert, _ = dns.NewRR(fmt.Sprintf("%s. %d IN IN AAAA %s", lbc.ClusterName, ttl, ip.String()))
		}
		m.Insert([]dns.RR{rrInsert})
	}
	lbc.WriteToLog(log.LevelInfo, fmt.Sprintf("WE WOULD UPDATE THE DNS WITH THE IPS %v", m))
	c := new(dns.Client)
	m.SetTsig(keyName, dns.HmacMD5, 300, time.Now().Unix())
	c.TsigSecret = map[string]string{keyName: tsigKey}
	_, _, err := c.Exchange(m, dnsManager+":53")
	if err != nil {
		lbc.WriteToLog(log.LevelError, fmt.Sprintf("DNS update failed with (%v)", err))
		return err
	}
	lbc.WriteToLog(log.LevelInfo, fmt.Sprintf("DNS update with keyName %v", keyName))

	return nil
}

func (lbc *LBCluster) getIpsFromDNS(m *dns.Msg, dnsManager string, dnsType uint16, ips *[]net.IP) error {
	m.SetQuestion(lbc.ClusterName+".", dnsType)
	in, err := dns.Exchange(m, dnsManager+":53")
	if err != nil {
		lbc.WriteToLog(log.LevelError, fmt.Sprintf("Error getting the ipv4 state of dns: %v", err))
		return err
	}
	for _, a := range in.Answer {
		if t, ok := a.(*dns.A); ok {
			lbc.Slog.Debug(fmt.Sprintf("From %v, got ipv4 %v", t, t.A))
			*ips = append(*ips, t.A)
		} else if t, ok := a.(*dns.AAAA); ok {
			lbc.Slog.Debug(fmt.Sprintf("From %v, got ipv6 %v", t, t.AAAA))
			*ips = append(*ips, t.AAAA)
		}
	}
	return nil
}

func (lbc *LBCluster) GetStateDNS(dnsManager string) error {
	m := new(dns.Msg)
	var ips []net.IP
	m.SetEdns0(4096, false)
	lbc.WriteToLog(log.LevelDebug, "Getting the ips from the DNS")
	err := lbc.getIpsFromDNS(m, dnsManager, dns.TypeA, &ips)

	if err != nil {
		return err
	}
	err = lbc.getIpsFromDNS(m, dnsManager, dns.TypeAAAA, &ips)
	if err != nil {
		return err
	}

	lbc.WriteToLog(log.LevelInfo, fmt.Sprintf("Let's keep the list of ips : %v", ips))
	lbc.PreviousBestIpsDns = ips

	return nil
}
