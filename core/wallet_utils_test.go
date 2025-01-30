package core

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyWitness(t *testing.T) {
	t.Parallel()

	var (
		txHash         = "7e8b59e41d2ba71888272a14cff401268fa01dceb19014f5dda7763334b8f221"
		signingKey, _  = hex.DecodeString("1217236ac24d8ac12684b308cf9468f68ef5283096896dc1c5c3caf8351e2847")
		witnessCbor, _ = hex.DecodeString("8258203e9d3a6f792c9820ab4423e41256e4b6e2ae1f456318f9d936fc70e0eafdc76f58402992d7fbc6fb155b7cc83223c80bf9b0ddbfe24ff260600897a06e8050f6596a76defeea6a86048605f8f7c27ef53da318aa02838532ea1876aac876b2491a01")
		txHashBytes, _ = hex.DecodeString(txHash)
	)

	verificationKey := GetVerificationKeyFromSigningKey(signingKey)

	signature, vKeyWitness, err := TxWitnessRaw(witnessCbor).GetSignatureAndVKey()
	require.NoError(t, err)

	require.NoError(t, err)
	assert.Equal(t, verificationKey, vKeyWitness)

	signedMessageCbor, err := SignMessage(signingKey, verificationKey, txHashBytes)
	require.NoError(t, err)
	assert.Equal(t, signature, signedMessageCbor)

	dummySignature, err := SignMessage(signingKey, verificationKey, append([]byte{255}, txHash[1:]...))
	require.NoError(t, err)

	dummyWitness, err := cbor.Marshal([][]byte{verificationKey, dummySignature})
	require.NoError(t, err)

	assert.NoError(t, VerifyWitness(txHash, witnessCbor))
	assert.ErrorIs(t, VerifyWitness(strings.Replace(txHash, "7e", "7f", 1), witnessCbor), ErrInvalidSignature)
	assert.ErrorIs(t, VerifyWitness(txHash, dummyWitness), ErrInvalidSignature)
}

func TestVerifyMessage(t *testing.T) {
	t.Parallel()

	const msg = "Hello World!"

	priv, pub, err := GenerateKeyPair()
	require.NoError(t, err)

	signature, err := SignMessage(priv, pub, []byte(msg))
	require.NoError(t, err)

	require.NoError(t, VerifyMessage([]byte(msg), pub, signature))
	require.ErrorIs(t, VerifyMessage([]byte("invalid msg"), pub, signature), ErrInvalidSignature)
}

func TestKeyHash(t *testing.T) {
	t.Parallel()

	baseDirectory, err := os.MkdirTemp("", "key-hash-test")
	require.NoError(t, err)

	defer os.RemoveAll(baseDirectory)

	const accountsNumber = 20

	cliUtils := NewCliUtils(ResolveCardanoCliBinary(TestNetNetwork))

	for i := 0; i < accountsNumber; i++ {
		wallet, err := GenerateWallet(true)
		require.NoError(t, err)

		keyHash, err := GetKeyHash(wallet.VerificationKey)
		require.NoError(t, err)

		keyHashCli, err := cliUtils.GetKeyHash(wallet.VerificationKey)
		require.NoError(t, err)

		assert.Equal(t, keyHashCli, keyHash)
	}
}

func TestWalletExtended(t *testing.T) {
	t.Parallel()

	type walletContainer struct {
		Wallet      *Wallet `json:"wallet"`
		WalletStake *Wallet `json:"walletStake"`
	}

	wallet, err := GenerateWallet(false)
	require.NoError(t, err)
	require.Nil(t, wallet.StakeSigningKey)
	require.Nil(t, wallet.StakeVerificationKey)
	require.Len(t, wallet.SigningKey, KeySize)
	require.Len(t, wallet.VerificationKey, KeySize)

	walletStake, err := GenerateWallet(true)
	require.NoError(t, err)
	require.Len(t, walletStake.SigningKey, KeySize)
	require.Len(t, walletStake.VerificationKey, KeySize)
	require.Len(t, walletStake.StakeSigningKey, KeySize)
	require.Len(t, walletStake.StakeVerificationKey, KeySize)

	wc := &walletContainer{
		Wallet:      wallet,
		WalletStake: walletStake,
	}

	bytes, err := json.Marshal(wc)
	require.NoError(t, err)

	var wc2 *walletContainer

	require.NoError(t, json.Unmarshal(bytes, &wc2))

	require.Equal(t, wc, wc2)

	const (
		msg1 = "message number 1"
		msg2 = "some other message"
		msg3 = "third one"
	)

	signature, err := SignMessage(wc.Wallet.SigningKey, wc.Wallet.VerificationKey, []byte(msg1))
	require.NoError(t, err)

	signature2, err := SignMessage(wc.WalletStake.StakeSigningKey,
		wc.WalletStake.StakeVerificationKey, []byte(msg2))
	require.NoError(t, err)

	signature3, err := SignMessage(wc.WalletStake.SigningKey, wc.WalletStake.VerificationKey, []byte(msg3))
	require.NoError(t, err)

	require.NoError(t, VerifyMessage([]byte(msg1), wc.Wallet.VerificationKey, signature))
	require.NoError(t, VerifyMessage([]byte(msg2), wc.WalletStake.StakeVerificationKey, signature2))
	require.NoError(t, VerifyMessage([]byte(msg3), wc.WalletStake.VerificationKey, signature3))
	require.Error(t, VerifyMessage([]byte(msg3), wc.Wallet.VerificationKey, signature))
	require.Error(t, VerifyMessage([]byte(msg1), wc.WalletStake.StakeVerificationKey, signature2))
	require.Error(t, VerifyMessage([]byte(msg2), wc.WalletStake.VerificationKey, signature3))
}

func TestGetKeyBytes(t *testing.T) {
	t.Parallel()

	key1, err := GetKeyBytes("58201825bce09711e1563fc1702587da6892d1d869894386323bd4378ea5e3d6cba0")

	require.NoError(t, err)
	require.Equal(t, []byte{
		0x18, 0x25, 0xbc, 0xe0, 0x97, 0x11, 0xe1, 0x56, 0x3f, 0xc1, 0x70, 0x25, 0x87, 0xda, 0x68, 0x92, 0xd1, 0xd8, 0x69, 0x89, 0x43, 0x86, 0x32, 0x3b, 0xd4, 0x37, 0x8e, 0xa5, 0xe3, 0xd6, 0xcb, 0xa0,
	}, key1)

	key2, err := GetKeyBytes("581Ebce09711e1563fc1702587da6892d1d869894386323bd4378ea5e3d6cba0")

	require.NoError(t, err)
	require.Equal(t, []byte{
		0x0, 0x0, 0xbc, 0xe0, 0x97, 0x11, 0xe1, 0x56, 0x3f, 0xc1, 0x70, 0x25, 0x87, 0xda, 0x68, 0x92, 0xd1, 0xd8, 0x69, 0x89, 0x43, 0x86, 0x32, 0x3b, 0xd4, 0x37, 0x8e, 0xa5, 0xe3, 0xd6, 0xcb, 0xa0,
	}, key2)

	key3, err := GetKeyBytes("58221825bce09711e1563fc1702587da6892d1d869894386323bd4378ea5e3d6cba0FFFF")

	require.NoError(t, err)
	require.Equal(t, []byte{
		0x18, 0x25, 0xbc, 0xe0, 0x97, 0x11, 0xe1, 0x56, 0x3f, 0xc1, 0x70, 0x25, 0x87, 0xda, 0x68, 0x92, 0xd1, 0xd8, 0x69, 0x89, 0x43, 0x86, 0x32, 0x3b, 0xd4, 0x37, 0x8e, 0xa5, 0xe3, 0xd6, 0xcb, 0xa0,
	}, key3)

	key4, err := GetKeyBytes("58800800c832ac40041bcbd83fc7b6be8f9a93c508d06f767518bad3266d62c3ad497d022a84b1b6663e0c3c62955c43bdfc333b3434ea232ab4e8c41d6b99c7ee12c73cd59dbfba2e07577ad69621e964d404c7bef56f69e1691438abd373561999899ccba5b358e8e3af736263283a472bb941c185ff4b523f532800766f1427c2")

	require.NoError(t, err)
	require.Equal(t, []byte{
		0x8, 0x0, 0xc8, 0x32, 0xac, 0x40, 0x4, 0x1b, 0xcb, 0xd8, 0x3f, 0xc7, 0xb6, 0xbe, 0x8f, 0x9a, 0x93, 0xc5, 0x8, 0xd0, 0x6f, 0x76, 0x75, 0x18, 0xba, 0xd3, 0x26, 0x6d, 0x62, 0xc3, 0xad, 0x49, 0x7d, 0x2, 0x2a, 0x84, 0xb1, 0xb6, 0x66, 0x3e, 0xc, 0x3c, 0x62, 0x95, 0x5c, 0x43, 0xbd, 0xfc, 0x33, 0x3b, 0x34, 0x34, 0xea, 0x23, 0x2a, 0xb4, 0xe8, 0xc4, 0x1d, 0x6b, 0x99, 0xc7, 0xee, 0x12, 0xc7, 0x3c, 0xd5, 0x9d, 0xbf, 0xba, 0x2e, 0x7, 0x57, 0x7a, 0xd6, 0x96, 0x21, 0xe9, 0x64, 0xd4, 0x4, 0xc7, 0xbe, 0xf5, 0x6f, 0x69, 0xe1, 0x69, 0x14, 0x38, 0xab, 0xd3, 0x73, 0x56, 0x19, 0x99, 0x89, 0x9c, 0xcb, 0xa5, 0xb3, 0x58, 0xe8, 0xe3, 0xaf, 0x73, 0x62, 0x63, 0x28, 0x3a, 0x47, 0x2b, 0xb9, 0x41, 0xc1, 0x85, 0xff, 0x4b, 0x52, 0x3f, 0x53, 0x28, 0x0, 0x76, 0x6f, 0x14, 0x27, 0xc2,
	}, key4)

	for _, key := range [][]byte{key1, key2, key3, key4} {
		vkey, err := getBech32Key(key, "addr_vk")
		require.NoError(t, err)

		key2, err := GetKeyBytes(vkey)

		require.NoError(t, err)
		require.Equal(t, key, key2)
	}

	k, err := GetKeyBytes("5803010203")
	require.NoError(t, err)
	require.Len(t, k, KeySize)

	k, err = GetKeyBytes("584601020304040404040404040404040404040404040404040404040404040404040404040102030404040404040404040404040404040404040404040404040404040404040404")
	require.NoError(t, err)
	require.Len(t, k, KeyExtendedSize)
}
