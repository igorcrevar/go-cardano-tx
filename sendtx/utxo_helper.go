package sendtx

import (
	"fmt"
	"strconv"
	"strings"

	cardanowallet "github.com/igorcrevar/go-cardano-tx/core"
)

// GetUTXOsForAmounts selects UTXOs that fulfill specified token amount conditions while adhering
// to a maximum input limit per transaction.
//
// Parameters:
// - utxos: A list of available UTXOs for selection.
// - conditions: A map defining required token conditions (e.g., exact or minimum amounts).
// - maxInputsPerTx: The maximum number of UTXOs that should be returned.
// - tryAtLeastInputsPerTx: If possible it should be returned at least this number of UTXOs
//
// Returns:
// - cardanowallet.TxInputs: Selected UTXOs and their total sum if conditions are met.
// - error: An error if no valid selection can satisfy the conditions.
//
// The function iteratively selects UTXOs, replacing the smallest ones when the limit is reached,
// until the specified conditions are satisfied or no valid selection is possible.
func GetUTXOsForAmounts(
	utxos []cardanowallet.Utxo,
	conditions map[string]uint64,
	maxInputs int,
	tryAtLeastInputs int,
) (cardanowallet.TxInputs, error) {
	currentSum := map[string]uint64{}
	currentSumTotal := map[string]uint64{}
	choosenCount := 0

	for i, utxo := range utxos {
		// move the current UTXO to the end of the selected UTXOs list
		// while ensuring that all existing UTXOs remain in the list
		utxos[choosenCount], utxos[i] = utxo, utxos[choosenCount]
		choosenCount++
		currentSum[cardanowallet.AdaTokenName] += utxo.Amount
		currentSumTotal[cardanowallet.AdaTokenName] += utxo.Amount

		for _, token := range utxo.Tokens {
			currentSum[token.TokenName()] += token.Amount
			currentSumTotal[token.TokenName()] += token.Amount
		}

		if isSumSatisfiesCondition(currentSum, conditions) {
			return prepareTxInputs(utxos, currentSum, maxInputs, tryAtLeastInputs, choosenCount), nil
		}

		// replace the smallest utxo with the current one
		if choosenCount == maxInputs {
			minChosenUTXO, minChosenUTXOIdx := findMinUtxo(utxos[:choosenCount], currentSum, conditions)

			choosenCount--
			utxos[minChosenUTXOIdx], utxos[choosenCount] = utxos[choosenCount], utxos[minChosenUTXOIdx]

			currentSum[cardanowallet.AdaTokenName] -= minChosenUTXO.Amount

			for _, token := range minChosenUTXO.Tokens {
				currentSum[token.TokenName()] -= token.Amount
			}
		}
	}

	if isSumSatisfiesCondition(currentSumTotal, conditions) {
		return cardanowallet.TxInputs{}, fmt.Errorf(
			"%w: %s vs %s", cardanowallet.ErrUTXOsLimitReached,
			mapStrUInt64ToStr(currentSumTotal), mapStrUInt64ToStr(conditions))
	}

	return cardanowallet.TxInputs{}, fmt.Errorf(
		"%w: %s vs %s", cardanowallet.ErrUTXOsCouldNotSelect, mapStrUInt64ToStr(currentSum), mapStrUInt64ToStr(conditions))
}

func utxos2TxInputs(utxos []cardanowallet.Utxo) []cardanowallet.TxInput {
	txInputs := make([]cardanowallet.TxInput, len(utxos))
	for i, utxo := range utxos {
		txInputs[i] = cardanowallet.TxInput{
			Hash:  utxo.Hash,
			Index: utxo.Index,
		}
	}

	return txInputs
}

func prepareTxInputs(
	utxos []cardanowallet.Utxo, currentSum map[string]uint64, maxInputsPerTx, tryAtLeastInputsPerTx, choosenCount int,
) cardanowallet.TxInputs {
	// try to add utxos until we reach tryAtLeastUtxoCount
	cnt := max(min(
		len(utxos)-choosenCount,            // still available in inputUTXOs
		tryAtLeastInputsPerTx-choosenCount, // needed to fill tryAtLeastUtxoCount
		maxInputsPerTx-choosenCount,        // maxUtxoCount limit must be preserved
	), 0)

	for i := choosenCount; i < choosenCount+cnt; i++ {
		currentSum[cardanowallet.AdaTokenName] += utxos[i].Amount

		for _, token := range utxos[i].Tokens {
			currentSum[token.TokenName()] += token.Amount
		}
	}

	return cardanowallet.TxInputs{
		Inputs: utxos2TxInputs(utxos[:choosenCount+cnt]),
		Sum:    currentSum,
	}
}

func findMinUtxo(
	utxos []cardanowallet.Utxo, currentSum map[string]uint64, conditions map[string]uint64,
) (cardanowallet.Utxo, int) {
	replaceTokenName := cardanowallet.AdaTokenName
	biggestDiff := uint64(0)
	// take the token with the biggest difference as the one to replace
	for tokenName, desiredAmount := range conditions {
		sum := currentSum[tokenName]
		if desiredAmount > sum && desiredAmount-sum > biggestDiff {
			biggestDiff = desiredAmount - sum
			replaceTokenName = tokenName
		}
	}

	idx := 0
	minUtxo := utxos[0]
	minCmpAmount := minUtxo.GetTokenAmount(replaceTokenName)

	for i, utxo := range utxos[1:] {
		if amount := utxo.GetTokenAmount(replaceTokenName); amount < minCmpAmount {
			minUtxo = utxo
			minCmpAmount = amount
			idx = i + 1
		}
	}

	return minUtxo, idx
}

func isSumSatisfiesCondition(
	currentSum map[string]uint64, conditions map[string]uint64,
) bool {
	for tokenName, desiredAmount := range conditions {
		if currentSum[tokenName] < desiredAmount {
			return false
		}
	}

	return true
}

func mapStrUInt64ToStr(m map[string]uint64) string {
	var sb strings.Builder
	for k, v := range m {
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}

		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(strconv.FormatUint(v, 10))
	}

	return sb.String()
}
