package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func printArgsFatal() {
	log.Fatalf(`budget-verifier <bankPath> <budgetPath>
	    filter-file: ./%s`, FilterFileName)
}

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
var dateMatchRangeDays = 7

func (t Transaction) StringNoFollow() string {
	return fmt.Sprintf(
		"[%s: '%s', '%s', %s]",
		t.Timestamp.Format("2006-01-02"),
		t.Description,
		t.Details,
		printAmount(t.Amount))
}

func (f Filter) String() string {
	return fmt.Sprintf("[filter:'%s', min:%s, max:%s]", f.FilterRegex, printAmount(f.MinAmount), printAmount(f.MaxAmount))
}

func main() {
	if len(os.Args) != 3 {
		printArgsFatal()
	}

	bankPath := os.Args[1]
	budgetPath := os.Args[2]
	log.Printf("comparing bank statement %s to budget entries %s", bankPath, budgetPath)

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

	workingDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatalf("failed to get working directory: %+v", err)
	}
	filterPath := filepath.Join(workingDir, FilterFileName)
	filters, err := loadFilters(filterPath)
	if err != nil {
		log.Fatalf("failed to load filters from %s: %+v", filterPath, err)
	}

	missingTransactions, err := compareTransactions(bankTransactions, budgetTransactions, filters)
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

func compareTransactions(bankTransactions, budgetTransactions []Transaction, filters []Filter) ([]Transaction, error) {
	missingTransactions := []Transaction{}

	for bankIndex := 0; bankIndex < len(bankTransactions); bankIndex++ {
		bankT := &(bankTransactions[bankIndex])

		if isFiltered(bankT, filters) {
			if verbose {
				log.Printf("filtered %s", bankT.StringNoFollow())
			}
			continue
		}

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

func printAmount(amount int) string {
	return strconv.FormatFloat(float64(amount)/100.0, 'f', 2, 64)
}
