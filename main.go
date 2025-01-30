package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/igorcrevar/go-cardano-tx/common"
	cardano "github.com/igorcrevar/go-cardano-tx/core"
	"github.com/igorcrevar/go-cardano-tx/sendtx"
)

const (
	socketPath              = "/home/bbs/Apps/card/node.socket"
	testNetMagic            = uint(2)
	ogmiosUrl               = "http://localhost:1337"
	blockfrostUrl           = "https://cardano-preview.blockfrost.io/api/v0"
	blockfrostProjectApiKey = "preview7mGSjpyEKb24OxQ4cCxomxZ5axMs5PvE"
	potentialFee            = uint64(300_000)
	providerName            = "blockfrost"
	receiverAddr            = "addr_test1wz4k6frsfd9q98rya6zjxtpcmzn83pwc8uyl9yqw25p8qqcx3e0c0"
	receiverMultisigAddr    = "addr_test1wqh4yha0ndhwykrh9cuhr47nh2y97zvkls74h4jq6uhlpacujv3z3"
	minUtxoValue            = uint64(1_000_000)
)

func getSplitedStr(s string, mxlen int) (res []string) {
	for i := 0; i < len(s); i += mxlen {
		end := i + mxlen
		if end > len(s) {
			end = len(s)
		}

		res = append(res, s[i:end])
	}

	return res
}

func getKeyHashes(wallets []*cardano.Wallet) []string {
	keyHashes := make([]string, len(wallets))
	for i, w := range wallets {
		keyHashes[i], _ = cardano.GetKeyHash(w.VerificationKey)
	}

	return keyHashes
}

func createTx(
	cardanoCliBinary string,
	txProvider cardano.ITxProvider,
	wallet *cardano.Wallet,
	testNetMagic uint,
	receiverAddr string,
	lovelaceSendAmount uint64,
	potentialFee uint64,
) ([]byte, string, error) {
	enterptiseAddress, err := cardano.NewEnterpriseAddress(
		cardano.TestNetNetwork, wallet.VerificationKey)
	if err != nil {
		return nil, "", err
	}

	senderAddress := enterptiseAddress.String()
	metadata := map[string]interface{}{
		"0": map[string]interface{}{
			"type": "single",
		},
	}

	builder, err := cardano.NewTxBuilder(cardanoCliBinary)
	if err != nil {
		return nil, "", err
	}

	defer builder.Dispose()

	if err := builder.SetProtocolParametersAndTTL(context.Background(), txProvider, 0); err != nil {
		return nil, "", err
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, "", err
	}

	utxos, err := txProvider.GetUtxos(context.Background(), senderAddress)
	if err != nil {
		return nil, "", err
	}

	inputs, err := sendtx.GetUTXOsForAmounts(
		utxos, map[string]uint64{
			cardano.AdaTokenName: lovelaceSendAmount + potentialFee + minUtxoValue,
		}, 20, 1)
	if err != nil {
		return nil, "", err
	}

	tokens, err := cardano.GetTokensFromSumMap(inputs.Sum)
	if err != nil {
		return nil, "", err
	}

	lovelaceInputsSum := inputs.Sum[cardano.AdaTokenName]
	outputs := []cardano.TxOutput{
		{
			Addr:   receiverAddr,
			Amount: lovelaceSendAmount,
		},
		{
			Addr:   senderAddress,
			Tokens: tokens,
		},
	}

	builder.SetMetaData(metadataBytes).SetTestNetMagic(testNetMagic)
	builder.AddInputs(inputs.Inputs...).AddOutputs(outputs...)

	fee, err := builder.CalculateFee(1)
	if err != nil {
		return nil, "", err
	}

	builder.SetFee(fee)

	builder.UpdateOutputAmount(-1, lovelaceInputsSum-lovelaceSendAmount-fee)

	txRaw, txHash, err := builder.Build()
	if err != nil {
		return nil, "", err
	}

	txSignedRaw, err := builder.SignTx(txRaw, []cardano.ITxSigner{wallet})
	if err != nil {
		return nil, "", err
	}

	return txSignedRaw, txHash, nil
}

func createMultiSigTx(
	cardanoCliBinary string,
	txProvider cardano.ITxProvider,
	signers []*cardano.Wallet,
	feeSigners []*cardano.Wallet,
	testNetMagic uint,
	receiverAddr string,
	lovelaceSendAmount uint64,
	potentialFee uint64,
) ([]byte, string, error) {
	policyScriptMultiSig := cardano.NewPolicyScript(getKeyHashes(signers), len(signers)*2/3+1)
	policyScriptFeeMultiSig := cardano.NewPolicyScript(getKeyHashes(feeSigners), len(signers)*2/3+1)
	cliUtils := cardano.NewCliUtils(cardanoCliBinary)

	multisigPolicyID, err := cliUtils.GetPolicyID(policyScriptMultiSig)
	if err != nil {
		return nil, "", err
	}

	feeMultisigPolicyID, err := cliUtils.GetPolicyID(policyScriptFeeMultiSig)
	if err != nil {
		return nil, "", err
	}

	multiSigAddr, err := cardano.NewPolicyScriptAddress(cardano.TestNetNetwork, multisigPolicyID)
	if err != nil {
		return nil, "", err
	}

	multiSigFeeAddr, err := cardano.NewPolicyScriptAddress(cardano.TestNetNetwork, feeMultisigPolicyID)
	if err != nil {
		return nil, "", err
	}

	metadata := map[string]interface{}{
		"0": map[string]interface{}{
			"type": "multi",
		},
		"1": map[string]interface{}{
			"destinationChainId": "vector",
			"senderAddr": getSplitedStr(
				"addr_test1qzf762fxqdyc79d3zzjplc57z6dpnrkygq5960tjguh683n3evd0dmxh9k7yzdxvqv9279nmkkwhx4m5wkj006a44nyscj7w9r",
				40,
			),
			"transactions": []map[string]interface{}{
				{
					"address": getSplitedStr(
						"addr_test1wp9g0wy5f58ruvt3d8cf2v3hylna934p99y0pwv8a4pm2wcx9he4s",
						40,
					),
					"amount": 1100000,
				},
				{
					"address": getSplitedStr(
						"addr_test1qqpszngm7jx9seaw9pr6pql7hey62an4k8lk6uncmagfd6wtn8ktl44rmpwahjg9w349v2tcf9zvujxd442qr3j24fms3fr687",
						40,
					),
					"amount": 1000000,
				},
			},
			"type": "bridgingRequest",
		},
	}

	builder, err := cardano.NewTxBuilder(cardanoCliBinary)
	if err != nil {
		return nil, "", err
	}

	defer builder.Dispose()

	if err := builder.SetProtocolParametersAndTTL(context.Background(), txProvider, 0); err != nil {
		return nil, "", err
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, "", err
	}

	utxosMultisig, err := txProvider.GetUtxos(context.Background(), multiSigAddr.String())
	if err != nil {
		return nil, "", err
	}

	utxosFee, err := txProvider.GetUtxos(context.Background(), multiSigFeeAddr.String())
	if err != nil {
		return nil, "", err
	}

	multiSigInputs, err := sendtx.GetUTXOsForAmounts(
		utxosMultisig, map[string]uint64{
			cardano.AdaTokenName: lovelaceSendAmount + minUtxoValue,
		}, 20, 1)
	if err != nil {
		return nil, "", err
	}

	multiSigFeeInputs, err := sendtx.GetUTXOsForAmounts(
		utxosFee, map[string]uint64{
			cardano.AdaTokenName: potentialFee + minUtxoValue,
		}, 20, 1)
	if err != nil {
		return nil, "", err
	}

	tokens, err := cardano.GetTokensFromSumMap(multiSigInputs.Sum)
	if err != nil {
		return nil, "", err
	}

	tokensFee, err := cardano.GetTokensFromSumMap(multiSigFeeInputs.Sum)
	if err != nil {
		return nil, "", err
	}

	lovelaceInputsSum := multiSigInputs.Sum[cardano.AdaTokenName]
	lovelaceInputsFeeSum := multiSigFeeInputs.Sum[cardano.AdaTokenName]
	outputs := []cardano.TxOutput{
		{
			Addr:   receiverAddr,
			Amount: lovelaceSendAmount,
		},
		{
			Addr:   multiSigAddr.String(),
			Amount: lovelaceInputsSum - lovelaceSendAmount,
			Tokens: tokens,
		},
		{
			Addr:   multiSigFeeAddr.String(),
			Tokens: tokensFee,
		},
	}

	builder.SetMetaData(metadataBytes).SetTestNetMagic(testNetMagic)
	builder.AddOutputs(outputs...)
	builder.AddInputsWithScript(policyScriptMultiSig, multiSigInputs.Inputs...)
	builder.AddInputsWithScript(policyScriptFeeMultiSig, multiSigFeeInputs.Inputs...)

	fee, err := builder.CalculateFee(0)
	if err != nil {
		return nil, "", err
	}

	builder.SetFee(fee)

	if change := lovelaceInputsFeeSum - fee; change > 0 {
		builder.UpdateOutputAmount(-1, change)
	} else {
		if cnt := len(tokens); cnt > 0 {
			return nil, "", fmt.Errorf("fee address change is zero but there is %d tokens", cnt)
		}

		builder.RemoveOutput(-1)
	}

	txRaw, txHash, err := builder.Build()
	if err != nil {
		return nil, "", err
	}

	allTxSigners := make([]cardano.ITxSigner, len(feeSigners)+len(signers))
	for i, w := range signers {
		allTxSigners[i] = w
	}

	for i, w := range feeSigners {
		allTxSigners[i+len(signers)] = w
	}

	txSignedRaw, err := builder.SignTx(txRaw, allTxSigners)
	if err != nil {
		return nil, "", err
	}

	return txSignedRaw, txHash, nil
}

func createProvider(name string, cardanoCliBinary string) (cardano.ITxProvider, error) {
	switch name {
	case "blockfrost":
		return cardano.NewTxProviderBlockFrost(blockfrostUrl, blockfrostProjectApiKey), nil
	case "ogmios":
		return cardano.NewTxProviderOgmios(ogmiosUrl), nil
	default:
		return cardano.NewTxProviderCli(testNetMagic, socketPath, cardanoCliBinary)
	}
}

func loadWallets() ([]*cardano.Wallet, error) {
	signingKeys := []string{
		"58201825bce09711e1563fc1702587da6892d1d869894386323bd4378ea5e3d6cba0",
		"5820ccdae0d1cd3fa9be16a497941acff33b9aa20bdbf2f9aa5715942d152988e083",
		"582094bfc7d65a5d936e7b527c93ea6bf75de51029290b1ef8c8877bffe070398b40",
		"58204cd84bf321e70ab223fbdbfe5eba249a5249bd9becbeb82109d45e56c9c610a9",
		"58208fcc8cac6b7fedf4c30aed170633df487642cb22f7e8615684e2b98e367fcaa3",
		"582058fb35da120c65855ad691dadf5681a2e4fc62e9dcda0d0774ff6fdc463a679a",
	}

	wallets := make([]*cardano.Wallet, len(signingKeys))
	for i, sk := range signingKeys {
		signingKey, err := cardano.GetKeyBytes(sk)
		if err != nil {
			return nil, err
		}

		wallets[i] = cardano.NewWallet(signingKey, nil)
	}

	return wallets, nil
}

func submitTx(
	ctx context.Context,
	txProvider cardano.ITxProvider,
	txRaw []byte,
	txHash string,
	addr string,
	tokenName string,
	amountIncrement uint64,
) error {
	utxos, err := txProvider.GetUtxos(ctx, addr)
	if err != nil {
		return err
	}

	if err := txProvider.SubmitTx(context.Background(), txRaw); err != nil {
		return err
	}

	expectedAtLeast := cardano.GetUtxosSum(utxos)[tokenName] + amountIncrement

	fmt.Println("transaction has been submitted. hash =", txHash)

	newBalance, err := common.ExecuteWithRetry(ctx, func(ctx context.Context) (uint64, error) {
		utxos, err := txProvider.GetUtxos(ctx, addr)
		if err != nil {
			return 0, err
		}

		sum := cardano.GetUtxosSum(utxos)

		if sum[tokenName] < expectedAtLeast {
			return 0, common.ErrRetryTryAgain
		}

		return sum[tokenName], nil
	}, common.WithRetryCount(60))
	if err != nil {
		return err
	}

	fmt.Printf("transaction has been included in block. hash = %s, balance = %d\n", txHash, newBalance)

	return nil
}

func main() {
	cardanoCliBinary := cardano.ResolveCardanoCliBinary(cardano.TestNetNetwork)

	wallets, err := loadWallets()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	txProvider, err := createProvider(providerName, cardanoCliBinary)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	defer txProvider.Dispose()

	_, _ = txProvider.GetTip(context.Background())

	txRaw, txHash, err := createTx(
		cardanoCliBinary,
		txProvider,
		wallets[0],
		testNetMagic,
		receiverAddr,
		minUtxoValue,
		potentialFee)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	err = submitTx(
		context.Background(),
		txProvider,
		txRaw,
		txHash,
		receiverAddr,
		cardano.AdaTokenName,
		minUtxoValue)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	txRawMultisig, txHashMultisig, err := createMultiSigTx(
		cardanoCliBinary,
		txProvider,
		wallets[:3],
		wallets[3:],
		testNetMagic,
		receiverMultisigAddr,
		minUtxoValue,
		potentialFee)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	err = submitTx(
		context.Background(),
		txProvider,
		txRawMultisig,
		txHashMultisig,
		receiverMultisigAddr,
		cardano.AdaTokenName,
		minUtxoValue)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
