package main

type healthStatus struct {
	OpenTransactions []transaction `json:"failed_transactions"`
	CheckingPeriod   string        `json:"event_reader_checking_period"`
	LastTimeCheck    string        `json:"event_reader_checking_time"`
	Successful       bool          `json:"event_reader_was_reachable"`
}

type transaction struct {
	TransactionID string `json:"transaction_id"`
	UUID          string `json:"uuid"`
	LastModified  string `json:"start_time"`
}

type transactions []transaction
