package lbhost

import (
	"fmt"
	"net"
)

// TransportResult defines the result of SNMP request
type TransportResult struct {
	Transport string
	IP        net.IP
	// TODO maybe move this into a struct response
	// TODO Do not see the point of the Reponse int or string?!
	ResponseInt    int
	ResponseString string
	ResponseError  string
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
