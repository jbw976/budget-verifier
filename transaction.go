package main

import (
	"fmt"
	"time"
)

type Transaction struct {
	Timestamp   time.Time
	Description string
	Details     string
	Amount      int // amount in cents, can be negative or positive
	Matching    *Transaction
}

func (t Transaction) String() string {
	var matchingStr string
	if t.Matching != nil {
		matchingStr = t.Matching.StringNoFollow()
	} else {
		matchingStr = "<nil>"
	}

	return fmt.Sprintf("[%s (matching: %s)]", t.StringNoFollow(), matchingStr)
}

func (t Transaction) StringNoFollow() string {
	return fmt.Sprintf(
		"[%s: '%s', '%s', %s]",
		t.Timestamp.Format("2006-01-02"),
		t.Description,
		t.Details,
		printAmount(t.Amount))
}
