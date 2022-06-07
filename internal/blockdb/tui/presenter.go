package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

var _ list.DefaultItem = testCasePresenter{}

type testCasePresenter struct {
	blockdb.TestCaseResult
}

func (p testCasePresenter) FilterValue() string {
	return strings.Join(append([]string{p.Name, p.GitSha}, p.Chains...), " ")
}

func (p testCasePresenter) Title() string {
	return fmt.Sprintf("%s (git: %s)", p.Name, p.GitSha)
}

func (p testCasePresenter) Description() string {
	t := formatTime(p.CreatedAt)
	chains := strings.Join(p.Chains, " + ")
	return fmt.Sprintf("%s | %s", t, chains)
}

func testCasesToItems(cases []blockdb.TestCaseResult) []list.Item {
	items := make([]list.Item, len(cases))
	for i := range cases {
		items[i] = testCasePresenter{cases[i]}
	}
	return items
}

var _ list.DefaultItem = chainPresenter{}

type chainPresenter struct {
	blockdb.ChainResult
}

func (p chainPresenter) FilterValue() string { return p.ChainID }
func (p chainPresenter) Title() string       { return p.ChainID }

func (p chainPresenter) Description() string {
	return fmt.Sprintf("Height %d", p.Height)
}

func chainsToItems(chains []blockdb.ChainResult) []list.Item {
	items := make([]list.Item, len(chains))
	for i := range chains {
		items[i] = chainPresenter{chains[i]}
	}
	return items
}
