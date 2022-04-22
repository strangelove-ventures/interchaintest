package dockerutil

import (
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHandleNodeJobError(t *testing.T) {
	err := HandleNodeJobError(0, "", "", nil)
	require.NoError(t, err)

	err = HandleNodeJobError(1, "", "", nil)
	require.Error(t, err)

	err = HandleNodeJobError(0, "", "", errors.New("oops"))
	require.Error(t, err)
}
