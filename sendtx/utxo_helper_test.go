package sendtx

import (
	"testing"

	cardanowallet "github.com/igorcrevar/go-cardano-tx/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUTXOsForAmounts(t *testing.T) {
	utxos := []cardanowallet.Utxo{
		{
			Hash:   "1",
			Amount: 100,
		},
		{
			Hash:   "2",
			Amount: 50,
			Tokens: []cardanowallet.TokenAmount{
				{
					PolicyID: "1",
					Name:     "1",
					Amount:   100,
				},
			},
		},
		{
			Hash:   "3",
			Amount: 150,
		},
		{
			Hash:   "4",
			Amount: 200,
		},
		{
			Hash:   "5",
			Amount: 160,
			Tokens: []cardanowallet.TokenAmount{
				{
					PolicyID: "1",
					Name:     "1",
					Amount:   50,
				},
			},
		},
		{
			Hash:   "6",
			Amount: 400,
		},
		{
			Hash:   "7",
			Amount: 200,
			Tokens: []cardanowallet.TokenAmount{
				{
					PolicyID: "1",
					Name:     "1",
					Amount:   400,
				},
			},
		},
		{
			Hash:   "8",
			Amount: 50,
			Tokens: []cardanowallet.TokenAmount{
				{
					PolicyID: "1",
					Name:     "1",
					Amount:   200,
				},
			},
		},
	}

	t.Run("exact amount", func(t *testing.T) {
		txInputs, err := GetUTXOsForAmounts(append([]cardanowallet.Utxo{}, utxos...), map[string]uint64{
			cardanowallet.AdaTokenName: 610,
		}, 4, 1)

		require.NoError(t, err)
		assert.Equal(t, map[string]uint64{
			cardanowallet.AdaTokenName: 610,
			"1.31":                     50,
		}, txInputs.Sum)
		assert.Equal(t, []cardanowallet.TxInput{
			{
				Hash: "1",
			},
			{
				Hash: "4",
			},
			{
				Hash: "3",
			},
			{
				Hash: "5",
			},
		}, txInputs.Inputs)
	})

	t.Run("greater amount", func(t *testing.T) {
		txInputs, err := GetUTXOsForAmounts(append([]cardanowallet.Utxo{}, utxos...), map[string]uint64{
			cardanowallet.AdaTokenName: 710,
		}, 3, 1)

		require.NoError(t, err)
		assert.Equal(t, map[string]uint64{
			cardanowallet.AdaTokenName: 760,
			"1.31":                     50,
		}, txInputs.Sum)
		assert.Equal(t, []cardanowallet.TxInput{
			{
				Hash: "4",
			},
			{
				Hash: "5",
			},
			{
				Hash: "6",
			},
		}, txInputs.Inputs)
	})

	t.Run("greater tokens", func(t *testing.T) {
		txInputs, err := GetUTXOsForAmounts(append([]cardanowallet.Utxo{}, utxos...), map[string]uint64{
			cardanowallet.AdaTokenName: 200,
			"1.31":                     410,
		}, 2, 1)

		require.NoError(t, err)
		assert.Equal(t, map[string]uint64{
			cardanowallet.AdaTokenName: 250,
			"1.31":                     500,
		}, txInputs.Sum)
		assert.Equal(t, []cardanowallet.TxInput{
			{
				Hash: "2",
			},
			{
				Hash: "7",
			},
		}, txInputs.Inputs)
	})

	t.Run("exact tokens", func(t *testing.T) {
		txInputs, err := GetUTXOsForAmounts(append([]cardanowallet.Utxo{}, utxos...), map[string]uint64{
			cardanowallet.AdaTokenName: 200,
			"1.31":                     700,
		}, 3, 1)

		require.NoError(t, err)
		assert.Equal(t, map[string]uint64{
			cardanowallet.AdaTokenName: 300,
			"1.31":                     700,
		}, txInputs.Sum)
		assert.Equal(t, []cardanowallet.TxInput{
			{
				Hash: "7",
			},
			{
				Hash: "2",
			},
			{
				Hash: "8",
			},
		}, txInputs.Inputs)
	})

	t.Run("not enough tokens", func(t *testing.T) {
		_, err := GetUTXOsForAmounts(append([]cardanowallet.Utxo{}, utxos...), map[string]uint64{
			cardanowallet.AdaTokenName: 300,
			"1.31":                     1000,
		}, 3, 1)

		require.ErrorContains(t, err, "not enough funds")
	})

	t.Run("with tryAtLeastInputs", func(t *testing.T) {
		utxos := []cardanowallet.Utxo{
			{
				Hash:   "1",
				Amount: 50,
			},
			{
				Hash:   "2",
				Amount: 1000,
			},
			{
				Hash:   "3",
				Amount: 150,
				Tokens: []cardanowallet.TokenAmount{
					{
						PolicyID: "1",
						Name:     "1",
						Amount:   100,
					},
				},
			},
			{
				Hash:   "4",
				Amount: 200,
			},
		}

		txInputs, err := GetUTXOsForAmounts(append([]cardanowallet.Utxo{}, utxos...), map[string]uint64{
			cardanowallet.AdaTokenName: 1000,
		}, 5, 4)

		require.NoError(t, err)
		assert.Equal(t, map[string]uint64{
			cardanowallet.AdaTokenName: 1400,
			"1.31":                     100,
		}, txInputs.Sum)
		assert.Equal(t, []cardanowallet.TxInput{
			{
				Hash: "1",
			},
			{
				Hash: "2",
			},
			{
				Hash: "3",
			},
			{
				Hash: "4",
			},
		}, txInputs.Inputs)
	})
}
