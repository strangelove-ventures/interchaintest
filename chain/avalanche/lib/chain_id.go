package lib

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ChainID struct {
	Name   string
	Number uint32
}

var (
	FormatChainID = regexp.MustCompile(`^([a-zA-Z]+)-(\d+)$`)
	ErrBadChainID = errors.New("networkID has bad format")
)

func (cid ChainID) String() string {
	return fmt.Sprintf("%d", cid.Number)
}

func ParseChainID(str string) (*ChainID, error) {
	if !FormatChainID.Match([]byte(str)) {
		return nil, ErrBadChainID
	}
	raw := strings.Split(str, "-")
	num, err := strconv.ParseUint(raw[1], 10, 0)
	if err != nil {
		return nil, ErrBadChainID
	}
	return &ChainID{
		Name:   raw[0],
		Number: uint32(num),
	}, nil
}
