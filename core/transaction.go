package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	draftTxFile = "tx.draft"
)

type TxInput struct {
	Hash  string `json:"hsh"`
	Index uint32 `json:"ind"`
}

func NewTxInput(hash string, index uint32) TxInput {
	return TxInput{
		Hash:  hash,
		Index: index,
	}
}

func (i TxInput) String() string {
	return fmt.Sprintf("%s#%d", i.Hash, i.Index)
}

type TxOutput struct {
	Addr   string        `json:"addr"`
	Amount uint64        `json:"amount"`
	Tokens []TokenAmount `json:"token,omitempty"`
}

func NewTxOutput(addr string, amount uint64, tokens ...TokenAmount) TxOutput {
	return TxOutput{
		Addr:   addr,
		Amount: amount,
		Tokens: tokens,
	}
}

func (o TxOutput) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s+%d", o.Addr, o.Amount))

	for _, token := range o.Tokens {
		sb.WriteRune('+')
		sb.WriteString(token.String())
	}

	return sb.String()
}

type TxBuilder struct {
	baseDirectory      string
	inputs             []txInputWithPolicyScript
	outputs            []TxOutput
	mints              txTokenMintInputs
	metadata           []byte
	protocolParameters []byte
	timeToLive         uint64
	testNetMagic       uint
	fee                uint64
	cardanoCliBinary   string
}

func NewTxBuilder(cardanoCliBinary string) (*TxBuilder, error) {
	baseDirectory, err := os.MkdirTemp("", "cardano-txs")
	if err != nil {
		return nil, err
	}

	return &TxBuilder{
		baseDirectory:    baseDirectory,
		cardanoCliBinary: cardanoCliBinary,
	}, nil
}

func (b *TxBuilder) Dispose() {
	os.RemoveAll(b.baseDirectory)
}

func (b *TxBuilder) SetTestNetMagic(testNetMagic uint) *TxBuilder {
	b.testNetMagic = testNetMagic

	return b
}

func (b *TxBuilder) SetFee(fee uint64) *TxBuilder {
	b.fee = fee

	return b
}

func (b *TxBuilder) AddInputsWithScript(script IPolicyScript, inputs ...TxInput) *TxBuilder {
	for _, inp := range inputs {
		b.inputs = append(b.inputs, txInputWithPolicyScript{
			txInput:      inp,
			policyScript: script,
		})
	}

	return b
}

func (b *TxBuilder) AddInputsWithScripts(inputs []TxInput, scripts []IPolicyScript) *TxBuilder {
	cnt := len(inputs)
	if l := len(scripts); cnt > l {
		cnt = l
	}

	for i, inp := range inputs[:cnt] {
		b.inputs = append(b.inputs, txInputWithPolicyScript{
			txInput:      inp,
			policyScript: scripts[i],
		})
	}

	return b
}

func (b *TxBuilder) AddInputs(inputs ...TxInput) *TxBuilder {
	for _, inp := range inputs {
		b.inputs = append(b.inputs, txInputWithPolicyScript{
			txInput: inp,
		})
	}

	return b
}

func (b *TxBuilder) AddOutputs(outputs ...TxOutput) *TxBuilder {
	b.outputs = append(b.outputs, outputs...)

	return b
}

func (b *TxBuilder) UpdateOutputAmount(index int, amount uint64, tokenAmounts ...uint64) *TxBuilder {
	if index < 0 {
		index = len(b.outputs) + index
	}

	b.outputs[index].Amount = amount

	for i, amount := range tokenAmounts {
		if len(b.outputs[index].Tokens) > i {
			b.outputs[index].Tokens[i].Amount = amount
		}
	}

	return b
}

func (b *TxBuilder) RemoveOutput(index int) *TxBuilder {
	if index < 0 {
		index = len(b.outputs) + index
	}

	copy(b.outputs[index:], b.outputs[index+1:])
	b.outputs = b.outputs[:len(b.outputs)-1]

	return b
}

func (b *TxBuilder) AddTokenMints(
	policyScripts []IPolicyScript, tokens []TokenAmount,
) *TxBuilder {
	b.mints.tokens = append(b.mints.tokens, tokens...)
	b.mints.policyScripts = append(b.mints.policyScripts, policyScripts...)

	return b
}

func (b *TxBuilder) SetMetaData(metadata []byte) *TxBuilder {
	b.metadata = metadata

	return b
}

func (b *TxBuilder) SetProtocolParameters(protocolParameters []byte) *TxBuilder {
	b.protocolParameters = protocolParameters

	return b
}

func (b *TxBuilder) SetTimeToLive(timeToLive uint64) *TxBuilder {
	b.timeToLive = timeToLive

	return b
}

func (b *TxBuilder) CalculateFee(witnessCount int) (uint64, error) {
	if b.protocolParameters == nil {
		return 0, errors.New("protocol parameters not set")
	}

	protocolParamsFilePath := filepath.Join(b.baseDirectory, "protocol-parameters.json")
	if err := os.WriteFile(protocolParamsFilePath, b.protocolParameters, FilePermission); err != nil {
		return 0, err
	}

	if err := b.buildRawTx(protocolParamsFilePath, 0); err != nil {
		return 0, err
	}

	if witnessCount == 0 {
		for _, inp := range b.inputs {
			witnessCount += inp.GetWitnessCount()
		}

		witnessCount = max(witnessCount, 1)
	}

	feeOutput, err := runCommand(b.cardanoCliBinary, append([]string{
		"transaction", "calculate-min-fee",
		"--tx-body-file", filepath.Join(b.baseDirectory, draftTxFile),
		"--tx-in-count", strconv.Itoa(len(b.inputs)),
		"--tx-out-count", strconv.Itoa(len(b.outputs)),
		"--witness-count", strconv.FormatUint(uint64(witnessCount), 10),
		"--protocol-params-file", protocolParamsFilePath,
	}, getTestNetMagicArgs(b.testNetMagic)...))
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(strings.Split(feeOutput, " ")[0], 10, 64)
}

func (b *TxBuilder) Build() ([]byte, string, error) {
	if b.protocolParameters == nil {
		return nil, "", errors.New("protocol parameters not set")
	}

	if err := b.CheckOutputs(); err != nil {
		return nil, "", err
	}

	protocolParamsFilePath := filepath.Join(b.baseDirectory, "protocol-parameters.json")
	if err := os.WriteFile(protocolParamsFilePath, b.protocolParameters, FilePermission); err != nil {
		return nil, "", err
	}

	if err := b.buildRawTx(protocolParamsFilePath, b.fee); err != nil {
		return nil, "", err
	}

	bytes, err := os.ReadFile(filepath.Join(b.baseDirectory, draftTxFile))
	if err != nil {
		return nil, "", err
	}

	txRaw, err := newTransactionUnwitnessedRawFromJSON(bytes)
	if err != nil {
		return nil, "", err
	}

	txHash, err := NewCliUtils(b.cardanoCliBinary).getTxHash(txRaw, b.baseDirectory)
	if err != nil {
		return nil, "", err
	}

	return txRaw, txHash, nil
}

func (b *TxBuilder) CheckOutputs() error {
	var errs []error

	for i, x := range b.outputs {
		if x.Amount == 0 {
			errs = append(errs, fmt.Errorf("output (%s, %d) amount not specified", x.Addr, i))
		}
	}

	return errors.Join(errs...)
}

func (b *TxBuilder) buildRawTx(protocolParamsFilePath string, fee uint64) error {
	args := []string{
		"transaction", "build-raw",
		"--protocol-params-file", protocolParamsFilePath,
		"--fee", strconv.FormatUint(fee, 10),
		"--invalid-hereafter", strconv.FormatUint(b.timeToLive, 10),
		"--out-file", filepath.Join(b.baseDirectory, draftTxFile),
	}

	if b.metadata != nil {
		metaDataFilePath := filepath.Join(b.baseDirectory, "metadata.json")
		if err := os.WriteFile(metaDataFilePath, b.metadata, FilePermission); err != nil {
			return err
		}

		args = append(args, "--metadata-json-file", metaDataFilePath)
	}

	if err := b.mints.Apply(&args, b.baseDirectory); err != nil {
		return err
	}

	for i, inp := range b.inputs {
		if err := inp.Apply(&args, b.baseDirectory, i); err != nil {
			return err
		}
	}

	for _, out := range b.outputs {
		args = append(args, "--tx-out", out.String())
	}

	_, err := runCommand(b.cardanoCliBinary, args)

	return err
}

// SignTx signs tx and assembles all signatures in final tx
func (b *TxBuilder) SignTx(txRaw []byte, signers []ITxSigner) (res []byte, err error) {
	witnesses := make([][]byte, len(signers))

	for i, signer := range signers {
		witnesses[i], err = b.CreateTxWitness(txRaw, signer)
		if err != nil {
			return nil, err
		}
	}

	return b.AssembleTxWitnesses(txRaw, witnesses)
}

// CreateTxWitness signs transaction hash and creates witness cbor
func (b *TxBuilder) CreateTxWitness(txRaw []byte, wallet ITxSigner) ([]byte, error) {
	outFilePath := filepath.Join(b.baseDirectory, "tx.wit")
	txFilePath := filepath.Join(b.baseDirectory, "tx.raw")
	signingKeyPath := filepath.Join(b.baseDirectory, "tx.skey")
	signingKey, _ := wallet.GetPaymentKeys()

	txBytes, err := transactionUnwitnessedRaw(txRaw).ToJSON()
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(txFilePath, txBytes, FilePermission); err != nil {
		return nil, err
	}

	var title string
	if len(signingKey) > KeySize {
		title = "PaymentExtendedSigningKeyShelley_ed25519_bip32"
	} else {
		title = "PaymentSigningKeyShelley_ed25519"
	}

	key, err := NewKeyFromBytes(title, "", signingKey)
	if err != nil {
		return nil, err
	}

	if err := key.WriteToFile(signingKeyPath); err != nil {
		return nil, err
	}

	args := append([]string{
		"transaction", "witness",
		"--signing-key-file", signingKeyPath,
		"--tx-body-file", txFilePath,
		"--out-file", outFilePath},
		getTestNetMagicArgs(b.testNetMagic)...)

	if _, err = runCommand(b.cardanoCliBinary, args); err != nil {
		return nil, err
	}

	bytes, err := os.ReadFile(outFilePath)
	if err != nil {
		return nil, err
	}

	return newTransactionWitnessedRawFromJSON(bytes)
}

// AssembleTxWitnesses assembles final signed transaction
func (b *TxBuilder) AssembleTxWitnesses(txRaw []byte, witnesses [][]byte) ([]byte, error) {
	outFilePath := filepath.Join(b.baseDirectory, "tx.sig")
	txFilePath := filepath.Join(b.baseDirectory, "tx.raw")
	witnessesFilePaths := make([]string, len(witnesses))

	for i, witness := range witnesses {
		witnessesFilePaths[i] = filepath.Join(b.baseDirectory, fmt.Sprintf("witness-%d", i+1))

		content, err := TxWitnessRaw(witness).ToJSON()
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(witnessesFilePaths[i], content, FilePermission); err != nil {
			return nil, err
		}
	}

	txBytes, err := transactionUnwitnessedRaw(txRaw).ToJSON()
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(txFilePath, txBytes, FilePermission); err != nil {
		return nil, err
	}

	args := []string{
		"transaction", "assemble",
		"--tx-body-file", txFilePath,
		"--out-file", outFilePath}

	for _, fp := range witnessesFilePaths {
		args = append(args, "--witness-file", fp)
	}

	if _, err = runCommand(b.cardanoCliBinary, args); err != nil {
		return nil, err
	}

	bytes, err := os.ReadFile(outFilePath)
	if err != nil {
		return nil, err
	}

	return newTransactionWitnessedRawFromJSON(bytes)
}

type txInputWithPolicyScript struct {
	txInput      TxInput
	policyScript IPolicyScript
}

func (txInputPS txInputWithPolicyScript) Apply(
	args *[]string, basePath string, indx int,
) error {
	*args = append(*args, "--tx-in", txInputPS.txInput.String())

	if txInputPS.policyScript == nil {
		return nil
	}

	policyScriptJSON, err := txInputPS.policyScript.GetPolicyScriptJSON()
	if err != nil {
		return err
	}

	policyFilePath := filepath.Join(basePath, fmt.Sprintf("policy_%d.json", indx))
	if err := os.WriteFile(policyFilePath, policyScriptJSON, FilePermission); err != nil {
		return err
	}

	*args = append(*args, "--tx-in-script-file", policyFilePath)

	return nil
}

func (txInputPS txInputWithPolicyScript) GetWitnessCount() int {
	if txInputPS.policyScript != nil {
		return txInputPS.policyScript.GetCount()
	}

	return 0
}

type txTokenMintInputs struct {
	tokens        []TokenAmount
	policyScripts []IPolicyScript
}

func (txMint txTokenMintInputs) Apply(
	args *[]string, basePath string,
) error {
	if len(txMint.tokens) == 0 {
		return nil
	}

	var sb strings.Builder

	for _, token := range txMint.tokens {
		if sb.Len() > 0 {
			sb.WriteRune('+')
		}

		sb.WriteString(token.String())
	}

	*args = append(*args, "--mint", sb.String())

	for indx, policyScript := range txMint.policyScripts {
		policyScriptJSON, err := policyScript.GetPolicyScriptJSON()
		if err != nil {
			return err
		}

		policyFilePath := filepath.Join(basePath, fmt.Sprintf("policy_mint_%d.json", indx))
		if err := os.WriteFile(policyFilePath, policyScriptJSON, FilePermission); err != nil {
			return err
		}

		*args = append(*args, "--minting-script-file", policyFilePath)
	}

	return nil
}
