package core

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	AdaTokenPolicyID = "ada"
	AdaTokenName     = "lovelace"
)

type TokenAmount struct {
	PolicyID string `json:"pid"`
	Name     string `json:"nam"` // name must not be hex encoded
	Amount   uint64 `json:"val"`
}

func NewTokenAmount(policyID string, name string, amount uint64) TokenAmount {
	return TokenAmount{
		PolicyID: policyID,
		Name:     name,
		Amount:   amount,
	}
}

func NewTokenAmountWithFullName(name string, amount uint64, isNameEncoded bool) (TokenAmount, error) {
	parts := strings.Split(name, ".")
	if len(parts) != 2 {
		return TokenAmount{}, fmt.Errorf("invalid full token name: %s", name)
	}

	if !isNameEncoded {
		name = parts[1]
	} else {
		decodedName, err := hex.DecodeString(parts[1])
		if err != nil {
			return TokenAmount{}, fmt.Errorf("invalid full token name: %s", name)
		}

		name = string(decodedName)
	}

	return TokenAmount{
		PolicyID: parts[0],
		Name:     name,
		Amount:   amount,
	}, nil
}

func (tt TokenAmount) TokenName() string {
	return fmt.Sprintf("%s.%s", tt.PolicyID, hex.EncodeToString([]byte(tt.Name)))
}

func (tt TokenAmount) String() string {
	return fmt.Sprintf("%d %s.%s", tt.Amount, tt.PolicyID, hex.EncodeToString([]byte(tt.Name)))
}

type Utxo struct {
	Hash   string        `json:"hsh"`
	Index  uint32        `json:"ind"`
	Amount uint64        `json:"amount"`
	Tokens []TokenAmount `json:"tokens,omitempty"`
}

type QueryTipData struct {
	Block           uint64 `json:"block"`
	Epoch           uint64 `json:"epoch"`
	Era             string `json:"era"`
	Hash            string `json:"hash"`
	Slot            uint64 `json:"slot"`
	SlotInEpoch     uint64 `json:"slotInEpoch"`
	SlotsToEpochEnd uint64 `json:"slotsToEpochEnd"`
	SyncProgress    string `json:"syncProgress"`
}

type ITxSubmitter interface {
	// SubmitTx submits transaction - txSigned should be cbor serialized signed transaction
	SubmitTx(ctx context.Context, txSigned []byte) error
}

type ITxRetriever interface {
	GetTxByHash(ctx context.Context, hash string) (map[string]interface{}, error)
}

type ITxDataRetriever interface {
	GetTip(ctx context.Context) (QueryTipData, error)
	GetProtocolParameters(ctx context.Context) ([]byte, error)
}

type IUTxORetriever interface {
	GetUtxos(ctx context.Context, addr string) ([]Utxo, error)
}

type ITxProvider interface {
	ITxSubmitter
	ITxDataRetriever
	IUTxORetriever
	Dispose()
}

type ITxSigner interface {
	CreateTxWitness(txHash []byte) ([]byte, error)
	GetPaymentKeys() ([]byte, []byte)
}

type IPolicyScript interface {
	GetPolicyScriptJSON() ([]byte, error)
	GetCount() int
}
