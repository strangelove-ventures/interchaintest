package tui

import (
	"strconv"
	"time"

	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

func formatTime(t time.Time) string {
	return t.Format("01-02 03:04PM MST")
}

type testCasePresenter struct {
	tc blockdb.TestCaseResult
}

func (p testCasePresenter) Date() string    { return formatTime(p.tc.CreatedAt) }
func (p testCasePresenter) Name() string    { return p.tc.Name }
func (p testCasePresenter) GitSha() string  { return p.tc.GitSha }
func (p testCasePresenter) ChainID() string { return p.tc.ChainID }

func (p testCasePresenter) Height() string {
	if !p.tc.ChainHeight.Valid {
		return ""
	}
	return strconv.FormatInt(p.tc.ChainHeight.Int64, 10)
}

func (p testCasePresenter) TxTotal() string {
	if !p.tc.TxTotal.Valid {
		return ""
	}
	return strconv.FormatInt(p.tc.TxTotal.Int64, 10)
}
