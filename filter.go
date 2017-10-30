package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
)

const (
	FilterFileName = `filter.json`
)

type Filter struct {
	FilterRegex string `json:"regex"`
	MinAmount   int    `json:"min"` // amount in cents, can be negative or positive
	MaxAmount   int    `json:"max"` // amount in cents, can be negative or positive
}

func loadFilters(filterPath string) ([]Filter, error) {
	buf, err := ioutil.ReadFile(filterPath)
	if err != nil {
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
	for i := range filters {
		match, _ := regexp.MatchString("(?i)"+filters[i].FilterRegex, t.Description)
		if match && t.Amount >= filters[i].MinAmount && t.Amount <= filters[i].MaxAmount {
			return true
		}
	}

	return false
}
