package intro

import (
	"fmt"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v8/internal/dockerutil"
	"github.com/stretchr/testify/require"
)

func TestIntroContract(t *testing.T) {
	contract, err := dockerutil.CompileCwContract("contract")
	require.NoError(t, err)
	fmt.Println("Contract: ", contract)
}