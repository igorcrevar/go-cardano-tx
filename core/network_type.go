package core

import "strings"

type CardanoNetworkType byte

const (
	TestNetProtocolMagic = uint(1097911063)
	PreviewProtocolMagic = uint(2)
	PreProdProtocolMagic = uint(1)
	MainNetProtocolMagic = uint(764824073)

	MainNetNetwork CardanoNetworkType = 1
	TestNetNetwork CardanoNetworkType = 0
)

func (n CardanoNetworkType) GetPrefix() string {
	switch n {
	case MainNetNetwork:
		return "addr"
	case TestNetNetwork:
		return "addr_test"
	default:
		return "" // not handled but dont raise an error
	}
}

func (n CardanoNetworkType) GetStakePrefix() string {
	switch n {
	case MainNetNetwork:
		return "stake"
	case TestNetNetwork:
		return "stake_test"
	default:
		return "" // not handled but dont raise an error
	}
}

func (n CardanoNetworkType) IsMainNet() bool {
	return n == MainNetNetwork
}

func IsAddressWithValidPrefix(addr string) bool {
	return strings.HasPrefix(addr, "addr") || strings.HasPrefix(addr, "stake")
}
