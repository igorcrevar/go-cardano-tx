package core

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	ogmiosJSONRPCVersion = "2.0"
	adaTokenPolicyID     = "ada"
)

type TxProviderOgmios struct {
	url string
}

var _ ITxProvider = (*TxProviderOgmios)(nil)

func NewTxProviderOgmios(url string) *TxProviderOgmios {
	return &TxProviderOgmios{
		url: url,
	}
}

// Dispose implements ITxProvider.
func (o *TxProviderOgmios) Dispose() {}

// GetProtocolParameters implements ITxProvider.
func (o *TxProviderOgmios) GetProtocolParameters(ctx context.Context) ([]byte, error) {
	params, err := executeHTTPOgmios[ogmiosQueryProtocolParamsResponse](
		ctx, o.url, ogmiosQueryStateRequest{
			Jsonrpc: ogmiosJSONRPCVersion,
			Method:  "queryLedgerState/protocolParameters",
		}, false,
	)
	if err != nil {
		return nil, err
	}

	asFloat := func(s string) float64 {
		parts := strings.Split(s, "/")
		if len(parts) == 2 {
			v1, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			v2, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)

			return v1 / v2
		}

		v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)

		return v
	}

	pp := ProtocolParameters{
		ProtocolVersion:      NewProtocolParametersVersion(params.Result.Version.Major, params.Result.Version.Minor),
		MaxBlockHeaderSize:   params.Result.MaxBlockHeaderSize.Bytes,
		MaxBlockBodySize:     params.Result.MaxBlockBodySize.Bytes,
		MaxTxSize:            params.Result.MaxTransactionSize.Bytes,
		TxFeeFixed:           params.Result.MinFeeConstant.Ada.Lovelace,
		TxFeePerByte:         params.Result.MinFeeCoefficient,
		StakeAddressDeposit:  params.Result.StakeCredentialDeposit.Ada.Lovelace,
		StakePoolDeposit:     params.Result.StakePoolDeposit.Ada.Lovelace,
		MinPoolCost:          params.Result.MinStakePoolCost.Ada.Lovelace,
		PoolRetireMaxEpoch:   params.Result.StakePoolRetirementEpochBound,
		StakePoolTargetNum:   params.Result.DesiredNumberOfStakePools,
		PoolPledgeInfluence:  asFloat(params.Result.StakePoolPledgeInfluence),
		MonetaryExpansion:    asFloat(params.Result.MonetaryExpansion),
		TreasuryCut:          asFloat(params.Result.TreasuryExpansion),
		CollateralPercentage: params.Result.CollateralPercentage,
		ExecutionUnitPrices: NewProtocolParametersPriceMemorySteps(
			asFloat(params.Result.ScriptExecutionPrices.Memory), asFloat(params.Result.ScriptExecutionPrices.CPU)),
		UtxoCostPerByte: params.Result.MinUtxoDepositCoefficient, // coins_per_utxo_size ?
		MaxTxExecutionUnits: NewProtocolParametersMemorySteps(
			params.Result.MaxExecutionUnitsPerTransaction.Memory,
			params.Result.MaxExecutionUnitsPerTransaction.CPU),
		MaxBlockExecutionUnits: NewProtocolParametersMemorySteps(
			params.Result.MaxExecutionUnitsPerBlock.Memory,
			params.Result.MaxExecutionUnitsPerBlock.CPU),
		MaxCollateralInputs: params.Result.MaxCollateralInputs,
		MaxValueSize:        params.Result.MaxValueSize.Bytes,
		CostModels:          map[string][]int64{},
	}

	for scriptName, values := range params.Result.PlutusCostModels {
		if parts := strings.Split(scriptName, ":"); len(parts) == 2 && len(parts[1]) > 0 {
			pp.CostModels["PlutusV"+parts[1][1:]] = values
		}
	}

	return json.Marshal(pp)
}

// GetSlot implements ITxProvider.
func (o *TxProviderOgmios) GetTip(ctx context.Context) (QueryTipData, error) {
	heightResponse, err := executeHTTPOgmios[ogmiosQueryNetworkBlockHeightResponse](
		ctx, o.url, ogmiosQueryStateRequest{
			Jsonrpc: ogmiosJSONRPCVersion,
			Method:  "queryNetwork/blockHeight",
		}, false,
	)
	if err != nil {
		return QueryTipData{}, err
	}

	tipResponse, err := executeHTTPOgmios[ogmiosQueryTipResponse](
		ctx, o.url, ogmiosQueryStateRequest{
			Jsonrpc: ogmiosJSONRPCVersion,
			Method:  "queryLedgerState/tip",
		}, false,
	)
	if err != nil {
		return QueryTipData{}, err
	}

	return QueryTipData{
		Block: heightResponse.Result,
		Hash:  tipResponse.Result.ID,
		Slot:  tipResponse.Result.Slot,
	}, nil
}

// GetUtxos implements ITxProvider.
func (o *TxProviderOgmios) GetUtxos(ctx context.Context, addr string) ([]Utxo, error) {
	responseData, err := executeHTTPOgmios[ogmiosQueryUtxoResponse](
		ctx, o.url, ogmiosQueryUtxoRequest{
			Jsonrpc: ogmiosJSONRPCVersion,
			Method:  "queryLedgerState/utxo",
			Params: ogmiosQueryUtxoRequestParams{
				Addresses: []string{addr},
			},
		}, true,
	)
	if err != nil {
		return nil, err
	}

	var retVal = make([]Utxo, len(responseData.Result))
	for i, utxo := range responseData.Result {
		var (
			adaValue uint64
			tokens   []TokenAmount
		)

		if len(utxo.Value) > 1 {
			tokens = make([]TokenAmount, 0, len(utxo.Value)-1)
		}

		for policyID, nameValueMap := range utxo.Value {
			if policyID == adaTokenPolicyID {
				adaValue = nameValueMap[AdaTokenName]
			} else {
				for name, value := range nameValueMap {
					realName, err := hex.DecodeString(name)
					if err == nil {
						name = string(realName)
					}

					tokens = append(tokens, TokenAmount{
						Token:  NewToken(policyID, name),
						Amount: value,
					})
				}
			}
		}

		retVal[i] = Utxo{
			Hash:   utxo.Transaction.ID,
			Index:  utxo.Index,
			Amount: adaValue,
			Tokens: tokens,
		}
	}

	return retVal, nil
}

// Expects TxCborString
func (o *TxProviderOgmios) SubmitTx(ctx context.Context, txSigned []byte) error {
	response, err := executeHTTPOgmios[ogmiosSubmitTransactionResponse](
		ctx, o.url, ogmiosSubmitTransaction{
			Jsonrpc: ogmiosJSONRPCVersion,
			Method:  "submitTransaction",
			Params: ogmiosSubmitTransactionParams{
				Transaction: ogmiosSubmitTransactionParamsTransaction{
					CBOR: hex.EncodeToString(txSigned),
				},
			},
			ID: nil,
		}, false,
	)
	if err != nil {
		return err
	}

	if response.Error.Message != "" {
		return fmt.Errorf("ogmios submit tx error: %s", response.Error.Message)
	}

	return nil
}

func (o *TxProviderOgmios) GetTxByHash(ctx context.Context, hash string) (map[string]interface{}, error) {
	panic("not implemented") //nolint:gocritic
}

func executeHTTPOgmios[T any](
	ctx context.Context, url string, request any, notFoundIsNotError bool,
) (T, error) {
	var result T // Zero value for type T

	queryBytes, err := json.Marshal(request)
	if err != nil {
		return result, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(queryBytes))
	if err != nil {
		return result, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request
	resp, err := new(http.Client).Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if notFoundIsNotError && resp.StatusCode == http.StatusNotFound {
		return result, nil // tx not included in block (yet)
	}

	if resp.StatusCode != http.StatusOK {
		return result, getErrorFromResponseOgmios(resp)
	}

	err = json.NewDecoder(resp.Body).Decode(&result)

	return result, err
}

func getErrorFromResponseOgmios(resp *http.Response) error {
	var responseData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		return fmt.Errorf("status code %d", resp.StatusCode)
	}

	msg := responseData["error"].(map[string]interface{})["message"].(string) //nolint:forcetypeassert

	return fmt.Errorf("status code %d: %s", resp.StatusCode, msg)
}
