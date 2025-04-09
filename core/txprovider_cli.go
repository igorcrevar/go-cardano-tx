package core

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type TxProviderCli struct {
	baseDirectory    string
	testNetMagic     uint
	socketPath       string
	cardanoCliBinary string
}

var _ ITxProvider = (*TxProviderCli)(nil)

func NewTxProviderCli(testNetMagic uint, socketPath string, cardanoCliBinary string) (*TxProviderCli, error) {
	baseDirectory, err := os.MkdirTemp("", "cardano-txs")
	if err != nil {
		return nil, err
	}

	return &TxProviderCli{
		baseDirectory:    baseDirectory,
		testNetMagic:     testNetMagic,
		socketPath:       socketPath,
		cardanoCliBinary: cardanoCliBinary,
	}, nil
}

func (b *TxProviderCli) Dispose() {
	os.RemoveAll(b.baseDirectory)
}

func (b *TxProviderCli) GetProtocolParameters(_ context.Context) ([]byte, error) {
	args := append([]string{
		"query", "protocol-parameters",
		"--socket-path", b.socketPath,
	}, getTestNetMagicArgs(b.testNetMagic)...)

	response, err := runCommand(b.cardanoCliBinary, args)
	if err != nil {
		return nil, err
	}

	return []byte(response), nil
}

func (b *TxProviderCli) GetUtxos(_ context.Context, addr string) ([]Utxo, error) {
	args := append([]string{
		"query", "utxo",
		"--socket-path", b.socketPath,
		"--address", addr,
	}, getTestNetMagicArgs(b.testNetMagic)...)

	output, err := runCommand(b.cardanoCliBinary, args)
	if err != nil {
		return nil, err
	}

	rows := strings.Split(strings.Trim(output, "\n"), "\n")[2:]
	inputs := make([]Utxo, len(rows))

	for i, x := range rows {
		j, cnt, parts := 0, 0, strings.Split(x, " ")

		for j < len(parts) {
			partStr := parts[j]
			j++

			if partStr == "" {
				continue
			}

			cnt++
			switch cnt {
			case 1:
				inputs[i].Hash = partStr
			case 2:
				outputIndex, err := strconv.ParseUint(partStr, 0, 64)
				if err != nil {
					return nil, err
				}

				inputs[i].Index = uint32(outputIndex) //nolint:gosec
			default:
				if partStr == "" || partStr == "+" || strings.Contains(partStr, "Datum") {
					continue
				}

				amount, err := strconv.ParseUint(partStr, 10, 64)
				if err != nil {
					continue
				}

				if j < len(parts) {
					if parts[j] == AdaTokenName {
						inputs[i].Amount = amount

						j++
					} else if tokenData := strings.Split(parts[j], "."); len(tokenData) == 2 {
						realName, err := hex.DecodeString(tokenData[1])
						if err == nil {
							tokenData[1] = string(realName)
						}

						inputs[i].Tokens = append(inputs[i].Tokens, TokenAmount{
							Token:  NewToken(tokenData[0], tokenData[1]),
							Amount: amount,
						})

						j++
					}
				}
			}
		}
	}

	return inputs, nil
}

func (b *TxProviderCli) GetTip(_ context.Context) (QueryTipData, error) {
	args := append([]string{
		"query", "tip",
		"--socket-path", b.socketPath,
	}, getTestNetMagicArgs(b.testNetMagic)...)

	res, err := runCommand(b.cardanoCliBinary, args)
	if err != nil {
		return QueryTipData{}, err
	}

	var result QueryTipData

	if err := json.Unmarshal([]byte(res), &result); err != nil {
		return result, err
	}

	return result, nil
}

func (b *TxProviderCli) SubmitTx(_ context.Context, txSigned []byte) error {
	txFilePath := filepath.Join(b.baseDirectory, "tx.send")

	txBytes, err := transactionWitnessedRaw(txSigned).ToJSON()
	if err != nil {
		return err
	}

	if err := os.WriteFile(txFilePath, txBytes, FilePermission); err != nil {
		return err
	}

	args := append([]string{
		"transaction", "submit",
		"--socket-path", b.socketPath,
		"--tx-file", txFilePath,
	}, getTestNetMagicArgs(b.testNetMagic)...)

	res, err := runCommand(b.cardanoCliBinary, args)
	if err != nil {
		return err
	}

	if strings.Contains(res, "Transaction successfully submitted.") {
		return nil
	}

	return fmt.Errorf("unknown error submiting tx: %s", res)
}

func (b *TxProviderCli) GetTxByHash(ctx context.Context, hash string) (map[string]interface{}, error) {
	panic("not implemented") //nolint:gocritic
}
