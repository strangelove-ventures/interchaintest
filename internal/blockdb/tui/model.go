package tui

import (
	"time"

	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

type Model struct {
	schemaVersion string
	schemaDate    time.Time
	testCases     []blockdb.TestCaseResult
}

func NewModel(schemaVersion string, schemaDate time.Time, testCases []blockdb.TestCaseResult) *Model {
	return &Model{
		schemaVersion: schemaVersion,
		schemaDate:    schemaDate,
		testCases:     testCases,
	}
}
