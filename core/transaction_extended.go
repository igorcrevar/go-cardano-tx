package core

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrUTXOsLimitReached   = errors.New("utxos limit reached, consolidation is required")
	ErrUTXOsCouldNotSelect = errors.New("couldn't select UTXOs")
)

const defaultTimeToLiveInc = 200

func (b *TxBuilder) SetProtocolParametersAndTTL(
	ctx context.Context, retriever ITxDataRetriever, timeToLiveInc uint64,
) error {
	if timeToLiveInc == 0 {
		timeToLiveInc = defaultTimeToLiveInc
	}

	protocolParams, err := retriever.GetProtocolParameters(ctx)
	if err != nil {
		return err
	}

	tip, err := retriever.GetTip(ctx)
	if err != nil {
		return err
	}

	b.SetProtocolParameters(protocolParams).SetTimeToLive(tip.Slot + timeToLiveInc)

	return nil
}

type TxInputs struct {
	Inputs []TxInput
	Sum    map[string]uint64
}

func GetUTXOsForAmount(
	utxos []Utxo,
	tokenName string,
	desiredSum uint64,
	maxInputs int,
) (TxInputs, error) {
	findMinUtxo := func(utxos []Utxo) (Utxo, int) {
		minUtxo := utxos[0]
		minAmount := GetTokenAmountFromUtxo(minUtxo, tokenName)
		idx := 0

		for i, utxo := range utxos[1:] {
			if newAmount := GetTokenAmountFromUtxo(utxo, tokenName); newAmount < minAmount {
				minUtxo = utxo
				minAmount = newAmount
				idx = i + 1
			}
		}

		return minUtxo, idx
	}

	utxos2TxInputs := func(utxos []Utxo) []TxInput {
		inputs := make([]TxInput, len(utxos))
		for i, x := range utxos {
			inputs[i] = TxInput{
				Hash:  x.Hash,
				Index: x.Index,
			}
		}

		return inputs
	}

	// Loop through utxos to find first input with enough tokens
	// If we don't have this UTXO we need to use more of them
	//nolint:prealloc
	var (
		currentSum  = map[string]uint64{}
		chosenUTXOs []Utxo
	)

	for _, utxo := range utxos {
		currentSum[AdaTokenName] += utxo.Amount

		for _, token := range utxo.Tokens {
			currentSum[token.TokenName()] += token.Amount
		}

		chosenUTXOs = append(chosenUTXOs, utxo)

		if len(chosenUTXOs) > maxInputs {
			lastIdx := len(chosenUTXOs) - 1
			minChosenUTXO, minChosenUTXOIdx := findMinUtxo(chosenUTXOs)

			chosenUTXOs[minChosenUTXOIdx] = chosenUTXOs[lastIdx]
			chosenUTXOs = chosenUTXOs[:lastIdx]
			currentSum[AdaTokenName] -= minChosenUTXO.Amount

			for _, token := range minChosenUTXO.Tokens {
				currentSum[token.TokenName()] -= token.Amount
			}
		}

		if currentSum[tokenName] >= desiredSum {
			return TxInputs{
				Inputs: utxos2TxInputs(chosenUTXOs),
				Sum:    currentSum,
			}, nil
		}
	}

	utxosSum := GetUtxosSum(utxos)

	if utxosSum[tokenName] >= desiredSum {
		return TxInputs{}, fmt.Errorf(
			"%w: %d vs %d", ErrUTXOsLimitReached, utxosSum[tokenName], desiredSum)
	}

	return TxInputs{}, fmt.Errorf(
		"%w: %d vs %d", ErrUTXOsCouldNotSelect, currentSum[tokenName], desiredSum)
}

func GetTokenCostSum(txBuilder *TxBuilder, userAddress string, utxos []Utxo) (uint64, error) {
	userTokenSum := GetUtxosSum(utxos)

	txOutput := TxOutput{
		Addr:   userAddress,
		Amount: userTokenSum[AdaTokenName],
	}

	for tokenName, amount := range userTokenSum {
		if tokenName != AdaTokenName {
			token, err := NewTokenWithFullName(tokenName, true)
			if err != nil {
				return 0, err
			}

			txOutput.Tokens = append(txOutput.Tokens, NewTokenAmount(token, amount))
		}
	}

	retSum, err := txBuilder.CalculateMinUtxo(txOutput)
	if err != nil {
		return 0, err
	}

	return retSum, nil
}

// CreateTxOutputChange generates a TxOutput representing the change
// by subtracting the total sum of outputs from the total available amount of each currency/token.
func CreateTxOutputChange(
	baseTxOutput TxOutput, totalSum map[string]uint64, outputsSum map[string]uint64,
) (TxOutput, error) {
	totalAmount := baseTxOutput.Amount + totalSum[AdaTokenName]
	outputAmount := outputsSum[AdaTokenName]

	if totalAmount < outputAmount {
		return TxOutput{}, fmt.Errorf("invalid amount: has = %d, required = %d", totalAmount, outputAmount)
	}

	changeAmount := totalAmount - outputAmount
	changeTokens := []TokenAmount(nil)

	for tokenName, tokenAmount := range totalSum {
		if tokenName == AdaTokenName {
			continue
		}

		outputTokenAmount := outputsSum[tokenName]
		totalTokenAmount := tokenAmount

		for _, token := range baseTxOutput.Tokens {
			if token.TokenName() == tokenName {
				totalTokenAmount += token.Amount

				break
			}
		}

		if totalTokenAmount < outputTokenAmount {
			return TxOutput{}, fmt.Errorf("invalid token amount: has = %d, required = %d",
				totalTokenAmount, outputTokenAmount)
		}

		changeTokenAmount := totalTokenAmount - outputTokenAmount
		if changeTokenAmount > 0 {
			newToken, err := NewTokenWithFullName(tokenName, true)
			if err != nil {
				return TxOutput{}, err
			}

			changeTokens = append(changeTokens, NewTokenAmount(newToken, changeTokenAmount))
		}
	}

	return TxOutput{
		Addr:   baseTxOutput.Addr,
		Amount: changeAmount,
		Tokens: changeTokens,
	}, nil
}

func GetTokenAmountFromUtxo(utxo Utxo, tokenName string) uint64 {
	if tokenName == AdaTokenName {
		return utxo.Amount
	}

	for _, tok := range utxo.Tokens {
		if tok.TokenName() == tokenName {
			return tok.Amount
		}
	}

	return 0
}
