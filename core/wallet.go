package core

import (
	"encoding/hex"
	"encoding/json"
	"os"

	"github.com/fxamacker/cbor/v2"
)

const (
	KeyHashSize     = 28
	KeySize         = 32
	KeyExtendedSize = 128
)

type Wallet struct {
	VerificationKey      []byte `json:"vkey"`
	SigningKey           []byte `json:"skey"`
	StakeVerificationKey []byte `json:"vstake"`
	StakeSigningKey      []byte `json:"sstake"`
}

var _ ITxSigner = (*Wallet)(nil)

func NewWallet(signingKey, stakeSigningKey []byte) *Wallet {
	getVerificationKey := func(signingKey []byte) []byte {
		if len(signingKey) >= 96 {
			return signingKey[64:96]
		}

		return GetVerificationKeyFromSigningKey(signingKey)
	}

	signingKey = PadKeyToSize(signingKey)
	stakeVerificationKey := []byte(nil)

	if len(stakeSigningKey) > 0 {
		stakeSigningKey = PadKeyToSize(stakeSigningKey)
		stakeVerificationKey = getVerificationKey(stakeSigningKey)
	} else {
		stakeSigningKey = nil
	}

	return &Wallet{
		SigningKey:           signingKey,
		VerificationKey:      getVerificationKey(signingKey),
		StakeSigningKey:      stakeSigningKey,
		StakeVerificationKey: stakeVerificationKey,
	}
}

// GenerateWallet generates wallet
func GenerateWallet(isStake bool) (*Wallet, error) {
	var stakeSigningKey, stakeVerificationKey []byte

	signingKey, verificationKey, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	if isStake {
		stakeSigningKey, stakeVerificationKey, err = GenerateKeyPair()
		if err != nil {
			return nil, err
		}

		stakeSigningKey, stakeVerificationKey = PadKeyToSize(stakeSigningKey), PadKeyToSize(stakeVerificationKey)
	}

	return &Wallet{
		SigningKey:           PadKeyToSize(signingKey),
		VerificationKey:      PadKeyToSize(verificationKey),
		StakeSigningKey:      stakeSigningKey,
		StakeVerificationKey: stakeVerificationKey,
	}, nil
}

func (w Wallet) CreateTxWitness(txHash []byte) ([]byte, error) {
	signature, err := SignMessage(w.SigningKey, w.VerificationKey, txHash)
	if err != nil {
		return nil, err
	}

	return cbor.Marshal([][]byte{w.VerificationKey, signature})
}

func (w Wallet) GetPaymentKeys() ([]byte, []byte) {
	return w.SigningKey, w.VerificationKey
}

type Key struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Hex         string `json:"cborHex"`
}

func NewKey(filePath string) (Key, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return Key{}, err
	}

	var key Key

	if err := json.Unmarshal(bytes, &key); err != nil {
		return Key{}, err
	}

	return key, nil
}

func NewKeyFromBytes(keyType string, desc string, bytes []byte) (Key, error) {
	cborBytes, err := cbor.Marshal(PadKeyToSize(bytes))
	if err != nil {
		return Key{}, err
	}

	return Key{
		Type:        keyType,
		Description: desc,
		Hex:         hex.EncodeToString(cborBytes),
	}, nil
}

func (k Key) GetKeyBytes() ([]byte, error) {
	return GetKeyBytes(k.Hex)
}

func (k Key) WriteToFile(filePath string) error {
	bytes, err := json.Marshal(k)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, bytes, FilePermission); err != nil {
		return err
	}

	return nil
}
