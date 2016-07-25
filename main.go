package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
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

var verbose = false
var dateMatchRangeDays int

func (t Transaction) StringNoFollow() string {
	return fmt.Sprintf(
		"[%s: '%s', '%s', %s]",
		t.Timestamp.Format("2006-01-02"),
		t.Description,
		t.Details,
		strconv.FormatFloat(float64(t.Amount)/100.0, 'f', 2, 64))
}

func main() {
	if len(os.Args) != 3 {
		printArgsFatal()
	}

	bankPath := os.Args[1]
	budgetPath := os.Args[2]
	log.Printf("comparing bank statement %s to budget entries %s", bankPath, budgetPath)

	dateMatchRangeDays = 5

	bankRecords, err := readFile(bankPath)
	if err != nil {
		log.Fatalf("failed to read %s: %+v", bankPath, err)
	}
	bankTransactions, err := parseBankTransactions(bankRecords)
	if err != nil {
		log.Fatalf("failed to parse transactions for %s: %+v", bankPath, err)
	}

	budgetRecords, err := readFile(budgetPath)
	if err != nil {
		log.Fatalf("failed to read %s: %+v", budgetPath, err)
	}
	budgetTransactions, err := parseBudgetTransactions(budgetRecords)
	if err != nil {
		log.Fatalf("failed to parse transactions for %s: %+v", budgetPath, err)
	}

	missingTransactions, err := compareTransactions(bankTransactions, budgetTransactions)
	if err != nil {
		log.Fatalf("failed to compare transactions for %s and %s: %+v", bankPath, budgetPath, err)
	}

	if len(missingTransactions) == 0 {
		log.Printf("There are no missing transactions.  Good job budgeter!")
	} else {
		log.Printf("There are %d missing transactions:", len(missingTransactions))
		for _, t := range missingTransactions {
			log.Printf("%+v", t)
		}
	}
}

func readFile(p string) ([][]string, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %+v", p, err)
	}
	defer f.Close()

	records := [][]string{}
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1

	for {
		fields, err := r.Read()
		if err != nil {
			if err == io.EOF {
				return records, nil
			}

			return nil, fmt.Errorf("Read error: %+v", err)
		}

		records = append(records, fields)
	}
}

func parseBankTransactions(bankRecords [][]string) ([]Transaction, error) {
	// find starting index of useful records
	start := -1
	for i, record := range bankRecords {
		if len(record) > 3 && record[0] == "Date" && record[1] == "Description" && record[2] == "Amount" {
			// found the headers that precedes the useful records.  there's still 1 more useless record,
			// so the good starting point is actually 1 greater than just the next index.
			start = i + 2
		}
	}

	if start < 0 {
		return nil, errors.New("failed to find start of useful records")
	}

	transactions := []Transaction{}

	for i := start; i < len(bankRecords); i++ {
		transaction, err := parseTransaction(bankRecords[i], 0, 1, 2, -1)
		if err != nil {
			log.Printf("invalid record, skipping: %+v", err)
			continue
		}

		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

func parseBudgetTransactions(budgetRecords [][]string) ([]Transaction, error) {
	transactions := []Transaction{}

	for i := 1; i < len(budgetRecords); i++ {
		transaction, err := parseTransaction(budgetRecords[i], 0, 2, 4, 3)
		if err != nil {
			log.Printf("invalid record, skipping: %+v", err)
			continue
		}

		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

func parseTransaction(record []string, timestampIndex, descriptionIndex, amountIndex, detailsIndex int) (Transaction, error) {
	refTime := "01/02/2006"
	t, err := time.Parse(refTime, record[timestampIndex])
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid timestamp: %+v, %+v", err, record)
	}

	a, err := strconv.ParseFloat(strings.Replace(record[amountIndex], ",", "", -1), 64)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid amount: %+v, %+v", err, record)
	}

	var d string
	if detailsIndex > 0 {
		d = record[detailsIndex]
	}

	transaction := Transaction{
		Timestamp:   t,
		Description: record[descriptionIndex],
		Details:     d,
		Amount:      (int)(a * 100),
	}

	return transaction, nil
}

func compareTransactions(bankTransactions, budgetTransactions []Transaction) ([]Transaction, error) {
	missingTransactions := []Transaction{}

	for bankIndex := 0; bankIndex < len(bankTransactions); bankIndex++ {
		bankT := &(bankTransactions[bankIndex])

		potentialMatches := []*Transaction{}
		for budgetIndex := 0; budgetIndex < len(budgetTransactions); budgetIndex++ {
			budgetT := &(budgetTransactions[budgetIndex])
			if bankT.Amount == budgetT.Amount {
				if budgetT.Matching != nil {
					// this budget entry has already been matched, it can't be matched again
					continue
				}

				// the amount matches and this budget entry hasn't already been matched yet.  add to the list
				// of potential matches so we can later on pick the closest match by date
				potentialMatches = append(potentialMatches, budgetT)
			}
		}

		var closest *Transaction
		closestDuration := 99999.0
		if len(potentialMatches) > 0 {
			if verbose && len(potentialMatches) > 1 {
				log.Printf("bank item %s has %d potential matches: %+v", bankT.StringNoFollow(), len(potentialMatches), potentialMatches)
			}

			for i := 0; i < len(potentialMatches); i++ {
				pm := potentialMatches[i]
				d := bankT.Timestamp.Sub(pm.Timestamp).Hours()

				// for the best match, the delta between bank statement item and budget app item should always
				// be 0 or positive.  The budget app entry is always from the date the transaction happened, while
				// the bank item takes a while to clear.  Bank should always be later than budget app.
				if d >= 0 && d < closestDuration {
					closestDuration = d
					closest = pm
				}
			}

			// verify the date of the closest matching budget transaction is close enough in time
			// (don't match transactions with the same amount but from very different dates)
			if closest != nil &&
				closest.Timestamp.Before(bankT.Timestamp.AddDate(0, 0, dateMatchRangeDays)) &&
				closest.Timestamp.After(bankT.Timestamp.AddDate(0, 0, -1*dateMatchRangeDays)) {

				bankT.Matching = closest
				closest.Matching = bankT
			}
		}

		if verbose && len(potentialMatches) > 1 {
			log.Printf("bank item %s matched with %s", bankT.StringNoFollow(), bankT.Matching)
		}

		if bankT.Matching == nil {
			missingTransactions = append(missingTransactions, *bankT)
		}
	}

	if verbose {
		log.Printf("****************** start bank transactions: ******************")
		for _, bt := range bankTransactions {
			log.Printf("%s", bt)
		}
		log.Printf("****************** end bank transactions *********************")
	}

	return missingTransactions, nil
}

func printArgsFatal() {
	log.Fatal("budget-verifier <bankPath> <budgetPath>")
}
