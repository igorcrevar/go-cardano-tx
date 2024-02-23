package helper

import "github.com/igorcrevar/cardano-wallet-tx/core"

func PrepareSignedTx(
	txDataRetriever core.ITxDataRetriever,
	wallet *core.Wallet,
	testNetMagic uint,
	outputs []core.TxOutput,
	metadata []byte) ([]byte, string, error) {
	builder, err := core.NewTxBuilder()
	if err != nil {
		return nil, "", err
	}

	defer builder.Dispose()

	dto, err := core.NewTransactionDTO(txDataRetriever, wallet.GetAddress())
	if err != nil {
		return nil, "", err
	}

	dto.TestNetMagic = testNetMagic
	dto.Outputs = outputs
	dto.MetaData = metadata
	dto.PotentialFee = 200_000

	txRaw, hash, err := builder.BuildWithDTO(dto)
	if err != nil {
		return nil, "", err
	}

	txSigned, err := builder.Sign(txRaw, wallet.GetSigningKeyPath())
	if err != nil {
		return nil, "", err
	}

	return txSigned, hash, nil
}

func PrepareMultiSigTx(txDataRetriever core.ITxDataRetriever,
	multisigAddr *core.MultisigAddress,
	testNetMagic uint,
	outputs []core.TxOutput,
	metadata []byte) ([]byte, string, error) {
	builder, err := core.NewTxBuilder()
	if err != nil {
		return nil, "", err
	}

	defer builder.Dispose()

	policy, err := multisigAddr.GetPolicyScript()
	if err != nil {
		return nil, "", err
	}

	dto, err := core.NewTransactionDTO(txDataRetriever, multisigAddr.GetAddress())
	if err != nil {
		return nil, "", err
	}

	dto.TestNetMagic = testNetMagic
	dto.Outputs = outputs
	dto.MetaData = metadata
	dto.Policy = policy
	dto.WitnessCount = multisigAddr.GetCount()
	dto.PotentialFee = 200_000

	return builder.BuildWithDTO(dto)
}

func AssemblyAllWitnesses(txRaw []byte, wallets []*core.Wallet) ([]byte, error) {
	builder, err := core.NewTxBuilder()
	if err != nil {
		return nil, err
	}

	defer builder.Dispose()

	witnesses := make([][]byte, len(wallets))

	for i, x := range wallets {
		witness, err := builder.AddWitness(txRaw, x.GetSigningKeyPath())
		if err != nil {
			return nil, err
		}

		witnesses[i] = witness
	}

	return builder.AssembleWitnesses(txRaw, witnesses)
}
