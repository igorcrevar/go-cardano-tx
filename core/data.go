package core

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	AdaTokenName = "lovelace"
)

type Token struct {
	PolicyID string `json:"pid"`
	Name     string `json:"nam"` // name must plain name and not be hex encoded
}

func NewToken(policyID string, name string) Token {
	return Token{
		PolicyID: policyID,
		Name:     name,
	}
}

func NewTokenWithFullName(name string, isNameEncoded bool) (Token, error) {
	parts := strings.Split(name, ".")
	if len(parts) != 2 {
		return Token{}, fmt.Errorf("invalid full token name: %s", name)
	}

	if !isNameEncoded {
		return Token{
			PolicyID: parts[0],
			Name:     parts[1],
		}, nil
	}

	decodedName, err := hex.DecodeString(parts[1])
	if err != nil {
		return Token{}, fmt.Errorf("invalid full token name: %s", name)
	}

	return Token{
		PolicyID: parts[0],
		Name:     string(decodedName),
	}, nil
}

func (tt Token) String() string {
	return fmt.Sprintf("%s.%s", tt.PolicyID, hex.EncodeToString([]byte(tt.Name)))
}

type TokenAmount struct {
	Token
	Amount uint64 `json:"val"`
}

func NewTokenAmount(token Token, amount uint64) TokenAmount {
	return TokenAmount{
		Token:  token,
		Amount: amount,
	}
}

func (tt TokenAmount) TokenName() string {
	return tt.Token.String()
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

func (utxo Utxo) GetTokenAmount(tokenName string) uint64 {
	if tokenName == AdaTokenName {
		return utxo.Amount
	}

	for _, token := range utxo.Tokens {
		if token.TokenName() == tokenName {
			return token.Amount
		}
	}

	return 0
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
