package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:  "budget-verifier",
	RunE: runVerify,
}

var (
	bankPath           = ""
	budgetPath         = ""
	filterPath         = "filter.json"
	verbose            = false
	dateMatchRangeDays = 7
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enables or disables verbose output")
	rootCmd.PersistentFlags().StringVar(&bankPath, "bank", "", "path to downloaded bank statement")
	rootCmd.PersistentFlags().StringVar(&budgetPath, "budget", "", "path to downloaded budget app statement")
	rootCmd.PersistentFlags().StringVar(&filterPath, "filter", "filter.json", "path to filter JSON file")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("budget-verify error: %+v\n", err)
	}
}

func runVerify(cmd *cobra.Command, args []string) error {
	if bankPath == "" || budgetPath == "" {
		return fmt.Errorf("both bank path and budget path must be specified")
	}

	log.Printf("comparing bank statement %s to budget entries %s", bankPath, budgetPath)

	// read the bank statement file and parse the transactions
	bankRecords, err := readFile(bankPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %+v", bankPath, err)
	}
	bankTransactions, err := parseBankTransactions(bankRecords)
	if err != nil {
		return fmt.Errorf("failed to parse transactions for %s: %+v", bankPath, err)
	}

	// read the budget app file and parse the transactions
	budgetRecords, err := readFile(budgetPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %+v", budgetPath, err)
	}
	budgetTransactions, err := parseBudgetTransactions(budgetRecords)
	if err != nil {
		return fmt.Errorf("failed to parse transactions for %s: %+v", budgetPath, err)
	}

	// load filters from the filter file in the working directory
	workingDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return fmt.Errorf("failed to get working directory: %+v", err)
	}
	filterPath := filepath.Join(workingDir, filterPath)
	filters, err := loadFilters(filterPath)
	if err != nil {
		return fmt.Errorf("failed to load filters from %s: %+v", filterPath, err)
	}

	// compare bank vs. budget transactions to find missing ones
	missingTransactions, err := compareTransactions(bankTransactions, budgetTransactions, filters)
	if err != nil {
		return fmt.Errorf("failed to compare transactions for %s and %s: %+v", bankPath, budgetPath, err)
	}

	if len(missingTransactions) == 0 {
		log.Printf("There are no missing transactions.  Good job budgeter!")
	} else {
		log.Printf("There are %d missing transactions:", len(missingTransactions))
		for _, t := range missingTransactions {
			log.Printf("%+v", t)
		}
	}

	return nil
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
