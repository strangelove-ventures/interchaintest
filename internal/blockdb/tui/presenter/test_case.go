package presenter

import (
	"strconv"

	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

// TestCase presents a blockdb.TestCaseResult.
type TestCase struct {
	Result blockdb.TestCaseResult
}

func (p TestCase) ID() string      { return strconv.FormatInt(p.Result.ID, 10) }
func (p TestCase) Date() string    { return FormatTime(p.Result.CreatedAt) }
func (p TestCase) Name() string    { return p.Result.Name }
func (p TestCase) GitSha() string  { return p.Result.GitSha }
func (p TestCase) ChainID() string { return p.Result.ChainID }

func (p TestCase) Height() string {
	if !p.Result.ChainHeight.Valid {
		return ""
	}
	return strconv.FormatInt(p.Result.ChainHeight.Int64, 10)
}

func (p TestCase) TxTotal() string {
	if !p.Result.TxTotal.Valid {
		return ""
	}
	return strconv.FormatInt(p.Result.TxTotal.Int64, 10)
}
