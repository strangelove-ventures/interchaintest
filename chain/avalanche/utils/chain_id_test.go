package utils

import (
	"testing"
)

func TestNetworkID(t *testing.T) {
	inputs := []struct {
		str string
		obj ChainID
		err error
	}{
		{
			str: "somestring",
			err: ErrBadChainID,
		},
		{
			str: "network-123",
			obj: ChainID{"network", 123},
		},
	}
	for i, input := range inputs {
		netId, err := ParseChainID(input.str)
		if input.err != nil && (err == nil || input.err != err) {
			t.Errorf("[%d] error > want: %s, got: %s", i, input.err, err)
		}
		if input.err == nil && (netId == nil || netId.Name != input.obj.Name || netId.Number != input.obj.Number) {
			t.Errorf("[%d] result > want: %#v, got: %#v", i, input.obj, netId)
		}
	}
}
