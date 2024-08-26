package lib

import (
	"errors"
	"fmt"
	"strconv"
)

type ChainID struct {
	Name   string
	Number uint32
}

var (
	ErrBadChainID = errors.New("networkID has bad format")
)

func (cid ChainID) String() string {
	return fmt.Sprintf("%d", cid.Number)
}

func ParseChainID(str string) (*ChainID, error) {
	num, err := strconv.ParseUint(str, 10, 0)
	if err != nil {
		return nil, ErrBadChainID
	}
	return &ChainID{
		Name:   "neto",
		Number: uint32(num),
	}, nil
}
