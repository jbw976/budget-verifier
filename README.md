# budget-verifier

`budget-verifier` is a simple Go (golang) application that compares financial transaction entries found in bank statements against those found in budget apps.
The idea is to identify any charges to your bank account or credit card that you didn't expect or forgot to enter into your budget app.

## Instructions

First download the most recent statements from your bank and budget app.
To run `budget-verifier` to compare the two, simply invoke it along with the path to your downloaded bank statement and your downloaded budget app statement:

```console
budget-verifier <bankPath> <budgetPath>
	    filter-file: ./filter.json
```

Any entries from your bank statement that are not found in your budget app will be printed to the console.

### Filters

It is possible to use a filter file to instruct `budget-verifier` to ignore certain entries from your bank statement.
This can be useful for recurring consistent value payments (e.g. car payment) that you may not bother entering into your budget app because it is a fixed value (instead of variable) each month.

Filters have a regular expression and a value range that they will match against.
If both match, then the entry from the bank statement that matches the filter will be ignored.
Note that expenses are negative values and credits are positive.

For filter examples, see [filter.json.example](filter.json.example).

## Supported Formats

The following formats are currently supported:

**Bank Statements**

* [Bank of America checking/savings account](https://www.bankofamerica.com/)
* [Bank of America credit card](https://www.bankofamerica.com/)

**Budget App**

* [Goodbudget](https://goodbudget.com/)