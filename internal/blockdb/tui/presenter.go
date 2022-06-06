package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

type testCasePresenter struct {
	blockdb.TestCaseResult
}

func (i testCasePresenter) FilterValue() string {
	return i.Name + i.GitSha + strings.Join(i.Chains, " ")
}

func (i testCasePresenter) Title() string {
	return fmt.Sprintf("%s (git: %s)", i.Name, i.GitSha)
}

func (i testCasePresenter) Description() string {
	var (
		t      = formatTime(i.CreatedAt)
		chains = strings.Join(i.Chains, ", ")
	)
	return fmt.Sprintf("%s [%s]", t, chains)
}

func testCasesToItems(cases []blockdb.TestCaseResult) []list.DefaultItem {
	items := make([]list.DefaultItem, len(cases))
	for i := range cases {
		items[i] = testCasePresenter{cases[i]}
	}
	return items
}
