package core

import (
	"context"
	"slices"
	"sort"
)

// GetUtxosSum returns sum for tokens in utxos (including lovelace)
func GetUtxosSum(utxos []Utxo) map[string]uint64 {
	result := map[string]uint64{}

	for _, utxo := range utxos {
		result[AdaTokenName] += utxo.Amount

		for _, token := range utxo.Tokens {
			result[token.TokenName()] += token.Amount
		}
	}

	return result
}

// GetOutputsSum returns sum or tokens in outputs (including lovelace)
func GetOutputsSum(outputs []TxOutput) map[string]uint64 {
	result := map[string]uint64{}

	for _, output := range outputs {
		result[AdaTokenName] += output.Amount

		for _, token := range output.Tokens {
			result[token.TokenName()] += token.Amount
		}
	}

	return result
}

// IsTxInUtxos checks whether a specified transaction hash (txHash)
// exists within the UTXOs associated with the given address (addr).
func IsTxInUtxos(ctx context.Context, utxoRetriever IUTxORetriever, addr string, txHash string) (bool, error) {
	utxos, err := utxoRetriever.GetUtxos(ctx, addr)
	if err != nil {
		return false, err
	}

	for _, x := range utxos {
		if x.Hash == txHash {
			return true, nil
		}
	}

	return false, nil
}

// GetTokensFromSumMap processes a map of token names to their quantities and returns a slice of TokenAmount objects
func GetTokensFromSumMap(sum map[string]uint64, skipTokenNames ...string) ([]TokenAmount, error) {
	tokens := make([]TokenAmount, 0, len(sum)-1)

	for tokenName, amount := range sum {
		// lovelace should be skipped always
		if tokenName == AdaTokenName || slices.Contains(skipTokenNames, tokenName) {
			continue
		}

		token, err := NewTokenWithFullName(tokenName, true)
		if err != nil {
			token, err = NewTokenWithFullName(tokenName, false)
			if err != nil {
				return nil, err
			}
		}

		tokens = append(tokens, NewTokenAmount(token, amount))
	}

	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].TokenName() < tokens[j].TokenName()
	})

	return tokens, nil
}
