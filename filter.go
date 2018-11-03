package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

type JSONDate time.Time

type Filter struct {
	FilterRegex string   `json:"regex"`
	MinAmount   int      `json:"min"` // amount in cents, can be negative or positive
	MaxAmount   int      `json:"max"` // amount in cents, can be negative or positive
	Date        JSONDate `json:"date,omitempty"`
}

func (f Filter) String() string {
	return fmt.Sprintf("[filter:'%s', min:%s, max:%s]", f.FilterRegex, printAmount(f.MinAmount), printAmount(f.MaxAmount))
}

func loadFilters(filterPath string) ([]Filter, error) {
	buf, err := ioutil.ReadFile(filterPath)
	if err != nil {
		if os.IsNotExist(err) {
			// the filter file doesn't exist, return an empty list of filters
			return []Filter{}, nil
		}

		return nil, fmt.Errorf("failed to read filter file: %+v", err)
	}

	var filters []Filter
	err = json.Unmarshal(buf, &filters)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal filter file: %+v", err)
	}

	if verbose {
		log.Printf("filters: %+v", filters)
	} else {
		log.Printf("found %d filters", len(filters))
	}

	return filters, nil
}

func isFiltered(t *Transaction, filters []Filter) bool {
	for _, f := range filters {
		match, _ := regexp.MatchString("(?i)"+f.FilterRegex, t.Description)
		if match && t.Amount >= f.MinAmount && t.Amount <= f.MaxAmount {
			// the regex matches and the amount matches, let's also factor in the date if the filter has one
			if time.Time(f.Date).IsZero() {
				// date isn't specified on the filter, don't factor it in and just call this a match
				return true
			}

			// check the filter's date to further verify this is a match
			return t.Timestamp.Before(time.Time(f.Date).AddDate(0, 0, 1)) &&
				t.Timestamp.After(time.Time(f.Date).AddDate(0, 0, -1))
		}
	}

	return false
}

func (j *JSONDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*j = JSONDate(t)
	return nil
}

func (j *JSONDate) MarshalJSON() ([]byte, error) {
	return json.Marshal(j)
}
