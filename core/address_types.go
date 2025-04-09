package core

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"

	"github.com/akamensky/base58"
	"github.com/fxamacker/cbor/v2"
	"github.com/igorcrevar/go-cardano-tx/core/bech32"
)

type CardanoAddressPayload struct {
	Payload  [KeyHashSize]byte
	IsScript bool
}

func (sc CardanoAddressPayload) String() string {
	return hex.EncodeToString(sc.Payload[:])
}

type StakePointer struct {
	Slot      uint64
	TxIndex   uint64
	CertIndex uint64
}

type cardanoAddressParser interface {
	GetAddressType() CardanoAddressType
	IsValid(bytes []byte) error
	ToString(bytes []byte) string
	ToCardanoAddressInfo(bytes []byte) CardanoAddressInfo
	FromCardanoAddressInfo(a CardanoAddressInfo) []byte
}

var addressParsers = []cardanoAddressParser{
	&cardanoBaseAddressParser{},
	&cardanoPointerAddressParser{},
	&cardanoEnterpriseAddressParser{},
	&cardanoRewardAddressParser{},
	&cardanoByronAddressParser{},
}

// cardanoBaseAddressParser BaseAddress
// 0000: keyhash28,keyhash28
// 0001: scripthash28,keyhash28
// 0010: keyhash28,scripthash28
// 0011: scripthash28,scripthash28
type cardanoBaseAddressParser struct{}

func (addrParser cardanoBaseAddressParser) GetAddressType() CardanoAddressType {
	return BaseAddress
}

func (addrParser cardanoBaseAddressParser) IsValid(bytes []byte) error {
	if len(bytes) < 1+KeyHashSize*2 {
		return fmt.Errorf("%w: expect %d got %d", ErrInvalidAddressData, 1+KeyHashSize*2, len(bytes))
	}

	return nil
}

func (addrParser cardanoBaseAddressParser) ToString(bytes []byte) string {
	str, _ := bech32.EncodeFromBase256(CardanoNetworkType(bytes[0]&0x0F).GetPrefix(), bytes)

	return str
}

func (addrParser cardanoBaseAddressParser) ToCardanoAddressInfo(bytes []byte) CardanoAddressInfo {
	header, data := (bytes[0]&0xF0)>>4, bytes[1:]

	return CardanoAddressInfo{
		AddressType: BaseAddress,
		Network:     CardanoNetworkType(bytes[0] & 0x0F),
		Payment: &CardanoAddressPayload{
			Payload:  [KeyHashSize]byte(data[:KeyHashSize]),
			IsScript: header&1 > 0,
		},
		Stake: &CardanoAddressPayload{
			Payload:  [KeyHashSize]byte(data[KeyHashSize : KeyHashSize*2]),
			IsScript: header&2 > 0,
		},
		Extra: data[2*KeyHashSize:],
	}
}

func (addrParser cardanoBaseAddressParser) FromCardanoAddressInfo(a CardanoAddressInfo) []byte {
	bytes := make([]byte, KeyHashSize*2+1+len(a.Extra))
	bytes[0] = (toByte(a.Payment.IsScript) << 4) | (toByte(a.Stake.IsScript) << 5) | (byte(a.Network) & 0xf)

	copy(bytes[1:KeyHashSize+1], a.Payment.Payload[:])
	copy(bytes[KeyHashSize+1:], a.Stake.Payload[:])
	copy(bytes[KeyHashSize*2+1:], a.Extra)

	return bytes
}

// cardanoPointerAddressParser
// 0100: keyhash28, 3 variable length uint
// 0101: scripthash28, 3 variable length uint
type cardanoPointerAddressParser struct{}

func (addrParser cardanoPointerAddressParser) GetAddressType() CardanoAddressType {
	return PointerAddress
}

func (addrParser cardanoPointerAddressParser) IsValid(bytes []byte) error {
	if len(bytes) < 1+KeyHashSize+1+1+1 { // header + payment + at least one byte for all three pointer parts
		return fmt.Errorf("%w: expect at least %d got %d", ErrInvalidAddressData, 1+KeyHashSize+1+1+1, len(bytes))
	}

	_, err := addrParser.getStakePointer(bytes[1+KeyHashSize:])

	return err
}

func (addrParser cardanoPointerAddressParser) ToString(bytes []byte) string {
	str, _ := bech32.EncodeFromBase256(CardanoNetworkType(bytes[0]&0x0F).GetPrefix(), bytes)

	return str
}

func (addrParser cardanoPointerAddressParser) ToCardanoAddressInfo(bytes []byte) CardanoAddressInfo {
	header, data := (bytes[0]&0xF0)>>4, bytes[1:]
	pointer, _ := addrParser.getStakePointer(data[KeyHashSize:])

	return CardanoAddressInfo{
		AddressType: PointerAddress,
		Network:     CardanoNetworkType(bytes[0] & 0x0F),
		Payment: &CardanoAddressPayload{
			Payload:  [KeyHashSize]byte(data[:KeyHashSize]),
			IsScript: header&1 > 0,
		},
		StakePointer: pointer,
	}
}

func (addrParser cardanoPointerAddressParser) FromCardanoAddressInfo(a CardanoAddressInfo) []byte {
	variableEncode := func(num uint64) []byte {
		var output []byte

		output = append(output, byte(num)&0x7F)
		num /= 128

		for num > 0 {
			output = append(output, byte(num)&0x7F|0x80)
			num /= 128
		}

		for i, j := 0, len(output)-1; i < j; i, j = i+1, j-1 {
			output[i], output[j] = output[j], output[i]
		}

		return output
	}

	buf := make([]byte, 0, KeyHashSize+1+3)

	header := 0b0100_0000 | (toByte(a.Payment.IsScript) << 4) | (byte(a.Network) & 0xf)
	buf = append(buf, header)
	buf = append(buf, a.Payment.Payload[:]...)
	buf = append(buf, variableEncode(a.StakePointer.Slot)...)
	buf = append(buf, variableEncode(a.StakePointer.TxIndex)...)

	return append(append(buf, variableEncode(a.StakePointer.CertIndex)...), a.Extra...)
}

func (addrParser cardanoPointerAddressParser) getStakePointer(raw []byte) (*StakePointer, error) {
	readOne := func(raw []byte) (result uint64, bytesReadCnt int, err error) {
		for _, rbyte := range raw {
			result = (result << 7) | uint64(rbyte&0x7F)
			bytesReadCnt++

			if (rbyte & 0x80) == 0 {
				return result, bytesReadCnt, nil
			}
		}

		return 0, 0, ErrInvalidAddressData
	}

	slot, bytesReadCnt, err := readOne(raw)
	if err != nil {
		return nil, err
	}

	txIndex, bytesReadCnt2, err := readOne(raw[bytesReadCnt:])
	if err != nil {
		return nil, err
	}

	certIndex, bytesReadCnt3, err := readOne(raw[bytesReadCnt+bytesReadCnt2:])
	if err != nil {
		return nil, err
	}

	if bytesReadCnt+bytesReadCnt2+bytesReadCnt3 != len(raw) {
		return nil, ErrInvalidAddressData
	}

	return &StakePointer{
		Slot:      slot,
		TxIndex:   txIndex,
		CertIndex: certIndex,
	}, nil
}

// cardanoEnterpriseAddressParser EnterpriseAddress
// 0110: keyhash28
// 0111: scripthash28
type cardanoEnterpriseAddressParser struct{}

func (addrParser cardanoEnterpriseAddressParser) GetAddressType() CardanoAddressType {
	return EnterpriseAddress
}

func (addrParser cardanoEnterpriseAddressParser) IsValid(bytes []byte) error {
	if len(bytes) != KeyHashSize+1 {
		return fmt.Errorf("%w: expect %d got %d", ErrInvalidAddressData, 1+KeyHashSize, len(bytes))
	}

	return nil
}

func (addrParser cardanoEnterpriseAddressParser) ToString(bytes []byte) string {
	str, _ := bech32.EncodeFromBase256(CardanoNetworkType(bytes[0]&0x0F).GetPrefix(), bytes)

	return str
}

func (addrParser cardanoEnterpriseAddressParser) ToCardanoAddressInfo(bytes []byte) CardanoAddressInfo {
	header, data := (bytes[0]&0xF0)>>4, bytes[1:]

	return CardanoAddressInfo{
		AddressType: EnterpriseAddress,
		Network:     CardanoNetworkType(bytes[0] & 0x0F),
		Payment: &CardanoAddressPayload{
			Payload:  [KeyHashSize]byte(data[:KeyHashSize]),
			IsScript: header&1 > 0,
		},
		Extra: data[KeyHashSize:],
	}
}

func (addrParser cardanoEnterpriseAddressParser) FromCardanoAddressInfo(a CardanoAddressInfo) []byte {
	bytes := make([]byte, KeyHashSize+1+len(a.Extra))
	bytes[0] = 0b01100000 | (toByte(a.Payment.IsScript) << 4) | (byte(a.Network) & 0xf)

	copy(bytes[1:], a.Payment.Payload[:])
	copy(bytes[1+KeyHashSize:], a.Extra)

	return bytes
}

// cardanoRewardAddressParser RewardAddress
// 0110: keyhash28
// 0111: scripthash28
type cardanoRewardAddressParser struct{}

func (addrParser cardanoRewardAddressParser) GetAddressType() CardanoAddressType {
	return RewardAddress
}

func (addrParser cardanoRewardAddressParser) IsValid(bytes []byte) error {
	if len(bytes) != KeyHashSize+1 {
		return fmt.Errorf("%w: expect %d got %d", ErrInvalidAddressData, 1+KeyHashSize, len(bytes))
	}

	return nil
}

func (addrParser cardanoRewardAddressParser) ToString(bytes []byte) string {
	str, _ := bech32.EncodeFromBase256(CardanoNetworkType(bytes[0]&0x0F).GetStakePrefix(), bytes)

	return str
}

func (addrParser cardanoRewardAddressParser) ToCardanoAddressInfo(bytes []byte) CardanoAddressInfo {
	header, data := (bytes[0]&0xF0)>>4, bytes[1:]

	return CardanoAddressInfo{
		AddressType: RewardAddress,
		Network:     CardanoNetworkType(bytes[0] & 0x0F),
		Stake: &CardanoAddressPayload{
			Payload:  [KeyHashSize]byte(data[:KeyHashSize]),
			IsScript: header&1 > 0,
		},
		Extra: data[KeyHashSize:],
	}
}

func (addrParser cardanoRewardAddressParser) FromCardanoAddressInfo(a CardanoAddressInfo) []byte {
	bytes := make([]byte, KeyHashSize+1+len(a.Extra))
	bytes[0] = 0b1110_0000 | (toByte(a.Stake.IsScript) << 4) | (byte(a.Network) & 0xf)

	copy(bytes[1:], a.Stake.Payload[:])
	copy(bytes[1+KeyHashSize:], a.Extra)

	return bytes
}

// cardanoByronAddressParser ByronAddress
// 1000: NOT IMPLEMENTED
type cardanoByronAddressParser struct{}

func (addrParser cardanoByronAddressParser) GetAddressType() CardanoAddressType {
	return ByronAddress
}

func (addrParser cardanoByronAddressParser) IsValid(bytes []byte) error {
	var rawAddr struct {
		_        struct{} `cbor:",toarray"`
		Tag      cbor.Tag
		Checksum uint32
	}

	if err := cbor.Unmarshal(bytes, &rawAddr); err != nil {
		return errors.Join(ErrInvalidAddressData, err)
	}

	rawTag, ok := rawAddr.Tag.Content.([]byte)
	if !ok || rawAddr.Tag.Number != 24 {
		return ErrInvalidAddressData
	}

	cheksum := crc32.ChecksumIEEE(rawTag)
	if rawAddr.Checksum != cheksum {
		return ErrInvalidAddressData
	}

	return nil
}

func (addrParser cardanoByronAddressParser) ToString(bytes []byte) string {
	return base58.Encode(bytes)
}

func (addrParser cardanoByronAddressParser) ToCardanoAddressInfo(bytes []byte) CardanoAddressInfo {
	const cborMetaDataNumber = 24

	var rawAddr struct {
		_        struct{} `cbor:",toarray"`
		Tag      cbor.Tag
		Checksum uint32
	}

	if err := cbor.Unmarshal(bytes, &rawAddr); err != nil {
		return CardanoAddressInfo{
			AddressType: UnsupportedAddress,
		}
	}

	rawTag, ok := rawAddr.Tag.Content.([]byte)
	if !ok || rawAddr.Tag.Number != cborMetaDataNumber {
		return CardanoAddressInfo{
			AddressType: UnsupportedAddress,
		}
	}

	var byron struct {
		_      struct{} `cbor:",toarray"`
		Hashed []byte
		Attrs  struct {
			Payload []byte `cbor:"1,keyasint,omitempty"`
			Network []byte `cbor:"2,keyasint,omitempty"`
		}
		Tag uint
	}

	if err := cbor.Unmarshal(rawTag, &byron); err != nil {
		return CardanoAddressInfo{
			AddressType: UnsupportedAddress,
		}
	}

	netInfo := byron.Attrs.Network

	network := MainNetNetwork
	if len(netInfo) == 1 && (netInfo[0] == byte(PreProdProtocolMagic) || netInfo[0] == byte(PreviewProtocolMagic)) ||
		len(netInfo) == 5 && binary.BigEndian.Uint32(netInfo[1:]) != uint32(MainNetProtocolMagic) {
		network = TestNetNetwork
	}

	return CardanoAddressInfo{
		AddressType: ByronAddress,
		Network:     network,
		Payment: &CardanoAddressPayload{
			Payload:  [28]byte(byron.Hashed),
			IsScript: false,
		},
		Extra: bytes,
	}
}

func (addrParser cardanoByronAddressParser) FromCardanoAddressInfo(a CardanoAddressInfo) []byte {
	return a.Extra
}

func toByte(b bool) byte {
	if !b {
		return 0
	}

	return 1
}

func getAddressParser(addressType CardanoAddressType) (cardanoAddressParser, error) {
	for _, parser := range addressParsers {
		if parser.GetAddressType() == addressType {
			return parser, nil
		}
	}

	return nil, ErrInvalidAddressData
}
