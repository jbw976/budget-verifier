package main

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

type parseIndices struct {
	timestamp   int
	description int
	amount      int
	details     int
}

func parseBankTransactions(bankRecords [][]string) ([]Transaction, error) {
	// find starting index of useful records and determine the parse indices
	start := -1
	var pi parseIndices
	for i, record := range bankRecords {
		if len(record) > 3 && record[0] == "Date" && record[1] == "Description" && record[2] == "Amount" {
			// Bank of America debit format
			// found the headers that precedes the useful records.  there's still 1 more useless record,
			// so the good starting point is actually 1 greater than just the next index.
			start = i + 2
			pi = parseIndices{timestamp: 0, description: 1, amount: 2, details: -1}
		} else if len(record) > 3 && record[0] == "Posted Date" && record[1] == "Reference Number" && record[2] == "Payee" {
			// Bank of America credit format
			start = i + 1
			pi = parseIndices{timestamp: 0, description: 2, amount: 4, details: -1}
		} else if len(record) > 4 && record[0] == "Transaction Date" && record[1] == "Post Date" && record[3] == "Category" {
			// Chase credit format
			start = i + 1
			pi = parseIndices{timestamp: 0, description: 2, amount: 5, details: -1}
		}
	}

	if start < 0 {
		return nil, errors.New("failed to find start of useful records")
	}

	transactions := []Transaction{}

	for i := start; i < len(bankRecords); i++ {
		transaction, err := parseTransaction(bankRecords[i], pi)
		if err != nil {
			log.Printf("invalid bank record, skipping: %+v", err)
			continue
		}

		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

func parseBudgetTransactions(budgetRecords [][]string) ([]Transaction, error) {
	pi := parseIndices{timestamp: 0, description: 2, amount: 5, details: 3}

	transactions := []Transaction{}

	for i := 1; i < len(budgetRecords); i++ {
		transaction, err := parseTransaction(budgetRecords[i], pi)
		if err != nil {
			log.Printf("invalid budget record, skipping: %+v", err)
			continue
		}

		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

func parseTransaction(record []string, pi parseIndices) (Transaction, error) {
	refTime := "01/02/2006"
	t, err := time.Parse(refTime, record[pi.timestamp])
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid timestamp: %+v, %+v", err, record)
	}

	a, err := strconv.ParseFloat(strings.Replace(record[pi.amount], ",", "", -1), 64)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid amount: %+v, %+v", err, record)
	}

	var d string
	if pi.details > 0 {
		d = record[pi.details]
	}

	transaction := Transaction{
		Timestamp:   t,
		Description: record[pi.description],
		Details:     d,
		Amount:      (int)(math.Round(a * 100.0)),
	}

	return transaction, nil
}
