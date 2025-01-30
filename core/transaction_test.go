package core

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	protocolParameters = []byte(`{"costModels":{"PlutusV1":[197209,0,1,1,396231,621,0,1,150000,1000,0,1,150000,32,2477736,29175,4,29773,100,29773,100,29773,100,29773,100,29773,100,29773,100,100,100,29773,100,150000,32,150000,32,150000,32,150000,1000,0,1,150000,32,150000,1000,0,8,148000,425507,118,0,1,1,150000,1000,0,8,150000,112536,247,1,150000,10000,1,136542,1326,1,1000,150000,1000,1,150000,32,150000,32,150000,32,1,1,150000,1,150000,4,103599,248,1,103599,248,1,145276,1366,1,179690,497,1,150000,32,150000,32,150000,32,150000,32,150000,32,150000,32,148000,425507,118,0,1,1,61516,11218,0,1,150000,32,148000,425507,118,0,1,1,148000,425507,118,0,1,1,2477736,29175,4,0,82363,4,150000,5000,0,1,150000,32,197209,0,1,1,150000,32,150000,32,150000,32,150000,32,150000,32,150000,32,150000,32,3345831,1,1],"PlutusV2":[205665,812,1,1,1000,571,0,1,1000,24177,4,1,1000,32,117366,10475,4,23000,100,23000,100,23000,100,23000,100,23000,100,23000,100,100,100,23000,100,19537,32,175354,32,46417,4,221973,511,0,1,89141,32,497525,14068,4,2,196500,453240,220,0,1,1,1000,28662,4,2,245000,216773,62,1,1060367,12586,1,208512,421,1,187000,1000,52998,1,80436,32,43249,32,1000,32,80556,1,57667,4,1000,10,197145,156,1,197145,156,1,204924,473,1,208896,511,1,52467,32,64832,32,65493,32,22558,32,16563,32,76511,32,196500,453240,220,0,1,1,69522,11687,0,1,60091,32,196500,453240,220,0,1,1,196500,453240,220,0,1,1,1159724,392670,0,2,806990,30482,4,1927926,82523,4,265318,0,4,0,85931,32,205665,812,1,1,41182,32,212342,32,31220,32,32696,32,43357,32,32247,32,38314,32,35892428,10,9462713,1021,10,38887044,32947,10]},"protocolVersion":{"major":7,"minor":0},"maxBlockHeaderSize":1100,"maxBlockBodySize":65536,"maxTxSize":16384,"txFeeFixed":155381,"txFeePerByte":44,"stakeAddressDeposit":0,"stakePoolDeposit":0,"minPoolCost":0,"poolRetireMaxEpoch":18,"stakePoolTargetNum":100,"poolPledgeInfluence":0,"monetaryExpansion":0.1,"treasuryCut":0.1,"collateralPercentage":150,"executionUnitPrices":{"priceMemory":0.0577,"priceSteps":0.0000721},"utxoCostPerByte":4310,"maxTxExecutionUnits":{"memory":16000000,"steps":10000000000},"maxBlockExecutionUnits":{"memory":80000000,"steps":40000000000},"maxCollateralInputs":3,"maxValueSize":5000,"extraPraosEntropy":null,"decentralization":null,"minUTxOValue":null}`)
)

func Test_TransactionBuilder(t *testing.T) {
	t.Parallel()

	const (
		testNetMagic = 203
		ttl          = uint64(28096)
	)

	walletsKeyHashes := []string{
		"d6b67f93ffa4e2651271cc9bcdbdedb2539911266b534d9c163cba21",
		"cba89c7084bf0ce4bf404346b668a7e83c8c9c250d1cafd8d8996e41",
		"79df3577e4c7d7da04872c2182b8d8829d7b477912dbf35d89287c39",
		"2368e8113bd5f32d713751791d29acee9e1b5a425b0454b963b2558b",
		"06b4c7f5254d6395b527ac3de60c1d77194df7431d85fe55ca8f107d",
	}
	walletsFeeKeyHashes := []string{
		"f0f4837b3a306752a2b3e52394168bc7391de3dce11364b723cc55cf",
		"47344d5bd7b2fea56336ba789579705a944760032585ef64084c92db",
		"f01018c1d8da54c2f557679243b09af1c4dd4d9c671512b01fa5f92b",
		"6837232854849427dae7c45892032d7ded136c5beb13c68fda635d87",
		"d215701e2eb17c741b9d306cba553f9fbaaca1e12a5925a065b90fa8",
	}

	policyScriptMultiSig := NewPolicyScript(walletsKeyHashes, len(walletsKeyHashes)*2/3+1)
	policyScriptFeeMultiSig := NewPolicyScript(walletsFeeKeyHashes, len(walletsFeeKeyHashes)*2/3+1)
	cliUtils := NewCliUtils(ResolveCardanoCliBinary(TestNetNetwork))

	multisigPolicyID, err := cliUtils.GetPolicyID(policyScriptMultiSig)
	require.NoError(t, err)

	feeMultisigPolicyID, err := cliUtils.GetPolicyID(policyScriptFeeMultiSig)
	require.NoError(t, err)

	multiSigAddr, err := NewPolicyScriptAddress(TestNetNetwork, multisigPolicyID)
	require.NoError(t, err)

	multiSigFeeAddr, err := NewPolicyScriptAddress(TestNetNetwork, feeMultisigPolicyID)
	require.NoError(t, err)

	type metaDataKey0 struct {
		Type       string `json:"type" cbor:"type"`
		Signers    int    `json:"signers" cbor:"signers"`
		FeeSigners int    `json:"feeSigners" cbor:"feeSigners"`
	}

	type metaDataKey1 struct {
		Company string `json:"comp" cbor:"comp"`
		City    string `json:"city" cbor:"city"`
	}

	metadata := map[uint64]interface{}{
		0: metaDataKey0{
			Type:       "multi",
			Signers:    len(walletsKeyHashes),
			FeeSigners: len(walletsFeeKeyHashes),
		},
		4: metaDataKey1{
			Company: "Ethernal",
			City:    "Novi Sad",
		},
	}
	outputs := []TxOutput{
		{
			Addr:   "addr_test1vqjysa7p4mhu0l25qknwznvj0kghtr29ud7zp732ezwtzec0w8g3u",
			Amount: uint64(1_000_000),
		},
	}
	outputsSum := GetOutputsSum(outputs)

	builder, err := NewTxBuilder(ResolveCardanoCliBinary(TestNetNetwork))
	require.NoError(t, err)

	defer builder.Dispose()

	metadataBytes, err := json.Marshal(metadata)
	require.NoError(t, err)

	multiSigInputs := TxInputs{
		Inputs: []TxInput{
			{
				Hash:  "e99a5bde15aa05f24fcc04b7eabc1520d3397283b1ee720de9fe2653abbb0c9f",
				Index: 0,
			},
			{
				Hash:  "d1fd0d772be7741d9bfaf0b037d02d2867a987ccba3e6ba2ee9aa2a861b73145",
				Index: 2,
			},
		},
		Sum: map[string]uint64{
			AdaTokenName: uint64(1_000_000)*3 - 10,
		},
	}

	multiSigFeeInputs := TxInputs{
		Inputs: []TxInput{
			{
				Hash:  "098236134e0f2077a6434dd9d7727126fa8b3627bcab3ae030a194d46eded73e",
				Index: 0,
			},
		},
		Sum: map[string]uint64{
			AdaTokenName: uint64(1_000_000) * 2,
		},
	}

	builder.SetTimeToLive(ttl).SetProtocolParameters(protocolParameters)
	builder.SetMetaData(metadataBytes).SetTestNetMagic(testNetMagic)
	builder.AddOutputs(outputs...).AddOutputs(TxOutput{
		Addr: multiSigAddr.String(),
	}).AddOutputs(TxOutput{
		Addr: multiSigFeeAddr.String(),
	})
	builder.AddInputsWithScript(policyScriptMultiSig, multiSigInputs.Inputs...)
	builder.AddInputsWithScript(policyScriptFeeMultiSig, multiSigFeeInputs.Inputs...)

	fee, err := builder.CalculateFee(0)
	require.NoError(t, err)

	builder.SetFee(fee)

	builder.UpdateOutputAmount(-2, multiSigInputs.Sum[AdaTokenName]-outputsSum[AdaTokenName])
	builder.UpdateOutputAmount(-1, multiSigFeeInputs.Sum[AdaTokenName]-fee)

	txRaw, txHash, err := builder.Build()
	require.NoError(t, err)

	assert.Equal(t, "84a50083825820098236134e0f2077a6434dd9d7727126fa8b3627bcab3ae030a194d46eded73e00825820d1fd0d772be7741d9bfaf0b037d02d2867a987ccba3e6ba2ee9aa2a861b7314502825820e99a5bde15aa05f24fcc04b7eabc1520d3397283b1ee720de9fe2653abbb0c9f00018382581d60244877c1aeefc7fd5405a6e14d927d91758d45e37c20fa2ac89cb1671a000f424082581d704aaad0f0626a8ce7b097497e542055b6520842ade881f980e002ae661a001e847682581d703ea4c4aef89a27f111e78464d7d6717b099f85ce27109ee9e5fbddec1a001a79bf021a00040ac103196dc0075820802e4d6f15ce98826886a5451e94855e77aae779cb341d3aab1e3bae4fb2f78da10182830304858200581c47344d5bd7b2fea56336ba789579705a944760032585ef64084c92db8200581c6837232854849427dae7c45892032d7ded136c5beb13c68fda635d878200581cd215701e2eb17c741b9d306cba553f9fbaaca1e12a5925a065b90fa88200581cf01018c1d8da54c2f557679243b09af1c4dd4d9c671512b01fa5f92b8200581cf0f4837b3a306752a2b3e52394168bc7391de3dce11364b723cc55cf830304858200581c06b4c7f5254d6395b527ac3de60c1d77194df7431d85fe55ca8f107d8200581c2368e8113bd5f32d713751791d29acee9e1b5a425b0454b963b2558b8200581c79df3577e4c7d7da04872c2182b8d8829d7b477912dbf35d89287c398200581ccba89c7084bf0ce4bf404346b668a7e83c8c9c250d1cafd8d8996e418200581cd6b67f93ffa4e2651271cc9bcdbdedb2539911266b534d9c163cba21f5d90103a100a200a36a6665655369676e65727305677369676e657273056474797065656d756c746904a26463697479684e6f76692053616464636f6d706845746865726e616c", hex.EncodeToString(txRaw))

	txHashUtil, err := cliUtils.GetTxHash(txRaw)
	require.NoError(t, err)

	require.Equal(t, "1b9298c51f4dc05c04cae37104124cfb76e9f98f04a7f6b8179cfe02913152ec", txHashUtil)
	require.Equal(t, txHash, txHashUtil)
}

func Test_TxBuilder_UpdateOutputAmountAndRemoveOutput(t *testing.T) {
	t.Parallel()

	builder, err := NewTxBuilder(ResolveCardanoCliBinary(TestNetNetwork))
	require.NoError(t, err)

	defer builder.Dispose()

	builder.AddOutputs(
		TxOutput{Addr: "0x1"},
		TxOutput{Addr: "0x2"},
		TxOutput{Addr: "0x3"},
		TxOutput{Addr: "0x4"},
	)

	require.Len(t, builder.outputs, 4)
	assert.Equal(t, uint64(0), builder.outputs[2].Amount)
	assert.Equal(t, uint64(0), builder.outputs[3].Amount)

	builder.UpdateOutputAmount(2, 200)
	builder.UpdateOutputAmount(-1, 500)

	assert.Equal(t, uint64(200), builder.outputs[2].Amount)
	assert.Equal(t, "0x3", builder.outputs[2].Addr)
	assert.Equal(t, uint64(500), builder.outputs[3].Amount)
	assert.Equal(t, "0x4", builder.outputs[3].Addr)

	builder.RemoveOutput(1)

	require.Len(t, builder.outputs, 3)
	assert.Equal(t, "0x1", builder.outputs[0].Addr)
	assert.Equal(t, uint64(0), builder.outputs[0].Amount)
	assert.Equal(t, "0x3", builder.outputs[1].Addr)
	assert.Equal(t, uint64(200), builder.outputs[1].Amount)
	assert.Equal(t, "0x4", builder.outputs[2].Addr)
	assert.Equal(t, uint64(500), builder.outputs[2].Amount)

	builder.RemoveOutput(0)

	require.Len(t, builder.outputs, 2)
	assert.Equal(t, "0x3", builder.outputs[0].Addr)
	assert.Equal(t, uint64(200), builder.outputs[0].Amount)
	assert.Equal(t, "0x4", builder.outputs[1].Addr)
	assert.Equal(t, uint64(500), builder.outputs[1].Amount)

	builder.RemoveOutput(1)

	require.Len(t, builder.outputs, 1)
	assert.Equal(t, "0x3", builder.outputs[0].Addr)
	assert.Equal(t, uint64(200), builder.outputs[0].Amount)

	builder.RemoveOutput(0)

	require.Len(t, builder.outputs, 0)
}

func Test_TxBuilder_CheckOutputs(t *testing.T) {
	t.Parallel()

	b, err := NewTxBuilder(ResolveCardanoCliBinary(TestNetNetwork))
	require.NoError(t, err)

	defer b.Dispose()

	b.AddOutputs(TxOutput{
		Addr:   "x1",
		Amount: 2,
	}, TxOutput{
		Addr:   "x2",
		Amount: 1,
	})

	require.NoError(t, b.CheckOutputs())

	b.AddOutputs(TxOutput{
		Addr:   "x3",
		Amount: 2,
	}, TxOutput{
		Addr:   "x4",
		Amount: 0,
	})

	require.Error(t, b.CheckOutputs(), errors.New("output (x4, 3) amount not specified"))
}

func TestCreateTxWitnessAndAssembleTxWitnesses(t *testing.T) {
	t.Parallel()

	const (
		skey        = "58800800c832ac40041bcbd83fc7b6be8f9a93c508d06f767518bad3266d62c3ad497d022a84b1b6663e0c3c62955c43bdfc333b3434ea232ab4e8c41d6b99c7ee12c73cd59dbfba2e07577ad69621e964d404c7bef56f69e1691438abd373561999899ccba5b358e8e3af736263283a472bb941c185ff4b523f532800766f1427c2"
		witnessData = "825820c73cd59dbfba2e07577ad69621e964d404c7bef56f69e1691438abd37356199958408233a747b14fc78ba32fbe8501b842d3290c591a565f589dbeec1c1e8b3dfe27de19002784c6c7020871fd07a5dd70e1003b6d1449255985c823464123085a00"
		txRaw       = "84a500818258201f55818892cc447cbf9fc27e04899ea98795538889555d3846a8071f4fdb75eb01018282581d70c4aab1955b120811d634e3a1b282ea090537d9e753842e8f46c280041a00200b2082583900712c77c7e146b95a569f2f7edf1dd81df2545edecb132701f17f84d4694c18049dcafc175d262c06eac9f52b86f205e38e8bfca6e6a545611a055e8308021a0002e908031a0152a319075820cb1b53bb62ee65e8ae893d04331dcc70d745298a32fcedf5ff9cc7a12d8471e3a0f5d90103a100a101a5616466766563746f726266611a0010c8e06173837828616464725f74657374317170636a63613738753972746a6b6a6b6e756868616863616d71776c793478287a376d6d39337866637037396c636634726666737671663877326c73743436663376716d34766e61781c6674736d6571746375773330373264653439673473737a333437377a61746662726964676562747881a26161827828766563746f725f7465737431766772677868347333356135706476306463347a6771333363726e33781934656d6e6b326537766e656e73663474657a7133746b6d396d616d1a000f4240"
		txWitness   = "84a500818258201f55818892cc447cbf9fc27e04899ea98795538889555d3846a8071f4fdb75eb01018282581d70c4aab1955b120811d634e3a1b282ea090537d9e753842e8f46c280041a00200b2082583900712c77c7e146b95a569f2f7edf1dd81df2545edecb132701f17f84d4694c18049dcafc175d262c06eac9f52b86f205e38e8bfca6e6a545611a055e8308021a0002e908031a0152a319075820cb1b53bb62ee65e8ae893d04331dcc70d745298a32fcedf5ff9cc7a12d8471e3a10081825820c73cd59dbfba2e07577ad69621e964d404c7bef56f69e1691438abd37356199958408233a747b14fc78ba32fbe8501b842d3290c591a565f589dbeec1c1e8b3dfe27de19002784c6c7020871fd07a5dd70e1003b6d1449255985c823464123085a00f5d90103a100a101a5616466766563746f726266611a0010c8e06173837828616464725f74657374317170636a63613738753972746a6b6a6b6e756868616863616d71776c793478287a376d6d39337866637037396c636634726666737671663877326c73743436663376716d34766e61781c6674736d6571746375773330373264653439673473737a333437377a61746662726964676562747881a26161827828766563746f725f7465737431766772677868347333356135706476306463347a6771333363726e33781934656d6e6b326537766e656e73663474657a7133746b6d396d616d1a000f4240"
	)

	skeyBytes, err := GetKeyBytes(skey)
	require.NoError(t, err)

	txRawBytes, err := hex.DecodeString(txRaw)
	require.NoError(t, err)

	wallet := NewWallet(skeyBytes, nil)

	txBuilder, err := NewTxBuilder(ResolveCardanoCliBinary(TestNetNetwork))
	require.NoError(t, err)

	defer txBuilder.Dispose()

	txWitnessBytes, err := txBuilder.CreateTxWitness(txRawBytes, wallet)
	require.NoError(t, err)

	require.Equal(t, witnessData, hex.EncodeToString(txWitnessBytes))

	cliUtils := NewCliUtils(ResolveCardanoCliBinary(TestNetNetwork))

	txHash, err := cliUtils.GetTxHash(txRawBytes)
	require.NoError(t, err)

	witness := TxWitnessRaw(txWitnessBytes)

	signature, vkey, err := witness.GetSignatureAndVKey()
	require.NoError(t, err)

	require.Equal(t, vkey, wallet.VerificationKey)

	txHashBytes, err := hex.DecodeString(txHash)
	require.NoError(t, err)

	require.NoError(t, VerifyMessage(txHashBytes, wallet.VerificationKey, signature))

	txFinal, err := txBuilder.AssembleTxWitnesses(txRawBytes, [][]byte{txWitnessBytes})
	require.NoError(t, err)

	require.Equal(t, txWitness, hex.EncodeToString(txFinal))
}
