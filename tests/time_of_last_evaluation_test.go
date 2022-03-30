package main_test

import (
	"testing"
	"time"
)

func TestTimeOfLastEvaluation(t *testing.T) {

	c := getTestCluster("test01.cern.ch")

	c.TimeOfLastEvaluation = time.Now().Add(time.Duration(-c.Parameters.PollingInterval+2) * time.Second)
	if c.TimeToRefresh() {
		t.Errorf("e.Time_of_last_evaluation: got\n%v\nexpected\n%v", true, false)
	}
	c.TimeOfLastEvaluation = time.Now().Add(time.Duration(-c.Parameters.PollingInterval-2) * time.Second)
	if !c.TimeToRefresh() {
		t.Errorf("e.Time_of_last_evaluation: got\n%v\nexpected\n%v", false, true)
	}
}
