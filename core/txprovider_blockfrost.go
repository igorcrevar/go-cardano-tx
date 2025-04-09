package core

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

const (
	blockfrostAuthHeaderKey = "project_id"
)

type blockFrostQueryUtxoResponse struct {
	Address     string `json:"address"`
	Hash        string `json:"tx_hash"`
	Index       uint32 `json:"tx_index"`
	OutputIndex uint32 `json:"output_index"`
	Amount      []struct {
		Unit     string `json:"unit"`
		Quantity string `json:"quantity"`
	} `json:"amount"`
}

type TxProviderBlockFrost struct {
	url           string
	projectID     string
	authHeaderKey string
}

var _ ITxProvider = (*TxProviderBlockFrost)(nil)

func NewTxProviderBlockFrost(url string, projectID string) *TxProviderBlockFrost {
	return &TxProviderBlockFrost{
		projectID:     projectID,
		url:           url,
		authHeaderKey: blockfrostAuthHeaderKey,
	}
}

func (b *TxProviderBlockFrost) Dispose() {
}

func (b *TxProviderBlockFrost) GetProtocolParameters(ctx context.Context) ([]byte, error) {
	// Create a request with the JSON payload
	req, err := http.NewRequestWithContext(ctx, "GET", b.url+"/epochs/latest/parameters", nil)
	if err != nil {
		return nil, err
	}

	// Set the Content-Type header to application/json
	req.Header.Set("Content-Type", "application/cbor")
	req.Header.Set(b.authHeaderKey, b.projectID)

	// Make the HTTP request
	resp, err := new(http.Client).Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// Check the HTTP status code
	if resp.StatusCode != http.StatusOK {
		return nil, getErrorFromResponse(resp)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return convertProtocolParameters(bytes)
}

func (b *TxProviderBlockFrost) GetUtxos(ctx context.Context, addr string) ([]Utxo, error) {
	// Create a request with the JSON payload
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/addresses/%s/utxos", b.url, addr), nil)
	if err != nil {
		return nil, err
	}

	// Set the Content-Type header to application/json
	req.Header.Set("Content-Type", "application/cbor")
	req.Header.Set(b.authHeaderKey, b.projectID)

	// Make the HTTP request
	resp, err := new(http.Client).Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// Check the HTTP status code
	if resp.StatusCode == http.StatusNotFound {
		return []Utxo{}, nil // this address does not have any UTxOs
	} else if resp.StatusCode != http.StatusOK {
		return nil, getErrorFromResponse(resp)
	}

	var bfResponse []blockFrostQueryUtxoResponse
	if err = json.NewDecoder(resp.Body).Decode(&bfResponse); err != nil {
		return nil, err
	}

	response := make([]Utxo, len(bfResponse))

	for i, bfUtxo := range bfResponse {
		var (
			tokens []TokenAmount
			amount uint64
		)

		for _, x := range bfUtxo.Amount {
			tmpAmount, err := strconv.ParseUint(x.Quantity, 0, 64)
			if err != nil {
				return nil, err
			}

			if x.Unit == AdaTokenName {
				amount = tmpAmount
			} else {
				policyID, name := x.Unit[0:KeyHashSize*2], x.Unit[KeyHashSize*2:]

				realName, err := hex.DecodeString(name)
				if err == nil {
					name = string(realName)
				}

				tokens = append(tokens, TokenAmount{
					Token:  NewToken(policyID, name),
					Amount: tmpAmount,
				})
			}
		}

		response[i] = Utxo{
			Hash:   bfUtxo.Hash,
			Index:  bfUtxo.Index,
			Amount: amount,
			Tokens: tokens,
		}
	}

	return response, nil
}

func (b *TxProviderBlockFrost) GetTip(ctx context.Context) (QueryTipData, error) {
	// Create a request with the JSON payload
	req, err := http.NewRequestWithContext(ctx, "GET", b.url+"/blocks/latest", nil)
	if err != nil {
		return QueryTipData{}, err
	}

	// Set the Content-Type header to application/json
	req.Header.Set("Content-Type", "application/cbor")
	req.Header.Set(b.authHeaderKey, b.projectID)

	// Make the HTTP request
	resp, err := new(http.Client).Do(req)
	if err != nil {
		return QueryTipData{}, err
	}

	defer resp.Body.Close()

	// Check the HTTP status code
	if resp.StatusCode != http.StatusOK {
		return QueryTipData{}, getErrorFromResponse(resp)
	}

	var bfResponse map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&bfResponse); err != nil {
		return QueryTipData{}, err
	}

	//nolint:forcetypeassert
	return QueryTipData{
		Slot:        uint64(bfResponse["slot"].(float64)),
		Block:       uint64(bfResponse["height"].(float64)),
		Epoch:       uint64(bfResponse["epoch"].(float64)),
		SlotInEpoch: uint64(bfResponse["epoch_slot"].(float64)),
		Hash:        bfResponse["hash"].(string),
	}, nil
}

func (b *TxProviderBlockFrost) SubmitTx(ctx context.Context, txSigned []byte) error {
	return blockfrostSubmitTx(ctx, b.url+"/tx/submit", b.authHeaderKey, b.projectID, txSigned)
}

func blockfrostSubmitTx(ctx context.Context, endpointURL, authHeader, authKey string, txSigned []byte) error {
	// Create a request with the JSON payload
	req, err := http.NewRequestWithContext(ctx, "POST", endpointURL, bytes.NewBuffer(txSigned))
	if err != nil {
		return err
	}

	// Set the Content-Type header to application/json
	req.Header.Set("Content-Type", "application/cbor")
	req.Header.Set(authHeader, authKey)

	// Make the HTTP request
	resp, err := new(http.Client).Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Check the HTTP status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return getErrorFromResponse(resp)
	}

	return nil
}

func (b *TxProviderBlockFrost) GetTxByHash(ctx context.Context, hash string) (map[string]interface{}, error) {
	// Create a request with the JSON payload
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/txs/%s", b.url, hash), nil)
	if err != nil {
		return nil, err
	}

	// Set the Content-Type header to application/json
	req.Header.Set(b.authHeaderKey, b.projectID)

	// Make the HTTP request
	resp, err := new(http.Client).Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // tx not included in block (yet)
	} else if resp.StatusCode != http.StatusOK {
		return nil, getErrorFromResponse(resp)
	}

	var bfResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&bfResponse); err != nil {
		return nil, err
	}

	return bfResponse, nil
}

func convertProtocolParameters(bytes []byte) ([]byte, error) {
	var bfpp struct {
		ProtocolMajorVer    uint64                      `json:"protocol_major_ver"`
		ProtocolMinorVer    uint64                      `json:"protocol_minor_ver"`
		MaxBlockHeaderSize  uint64                      `json:"max_block_header_size"`
		MaxBlockSize        uint64                      `json:"max_block_size"`
		MaxTxSize           uint64                      `json:"max_tx_size"`
		MinFeeB             uint64                      `json:"min_fee_b"`
		MinFeeA             uint64                      `json:"min_fee_a"`
		KeyDeposit          string                      `json:"key_deposit"`
		PoolDeposit         string                      `json:"pool_deposit"`
		MinPoolCost         string                      `json:"min_pool_cost"`
		EMax                uint64                      `json:"e_max"`
		NOpt                uint64                      `json:"n_opt"`
		A0                  float64                     `json:"a0"`
		Rho                 float64                     `json:"rho"`
		Tau                 float64                     `json:"tau"`
		CollateralPercent   uint64                      `json:"collateral_percent"`
		PriceMem            float64                     `json:"price_mem"`
		PriceStep           float64                     `json:"price_step"`
		CoinsPerUtxoWord    string                      `json:"coins_per_utxo_word"`
		MaxTxExMem          string                      `json:"max_tx_ex_mem"`
		MaxTxExSteps        string                      `json:"max_tx_ex_steps"`
		MaxBlockExMem       string                      `json:"max_block_ex_mem"`
		MaxBlockExSteps     string                      `json:"max_block_ex_steps"`
		MaxCollateralInputs uint64                      `json:"max_collateral_inputs"`
		MaxValSize          string                      `json:"max_val_size"`
		CostModels          map[string]map[string]int64 `json:"cost_models"`
	}

	if err := json.Unmarshal(bytes, &bfpp); err != nil {
		return nil, err
	}

	strToUInt64 := func(s string) uint64 {
		v, _ := strconv.ParseUint(s, 0, 64)

		return v
	}

	pp := ProtocolParameters{
		ProtocolVersion: NewProtocolParametersVersion(
			bfpp.ProtocolMajorVer, bfpp.ProtocolMinorVer),
		MaxBlockHeaderSize:   bfpp.MaxBlockHeaderSize,
		MaxBlockBodySize:     bfpp.MaxBlockSize,
		MaxTxSize:            bfpp.MaxTxSize,
		TxFeeFixed:           bfpp.MinFeeB,
		TxFeePerByte:         bfpp.MinFeeA,
		StakeAddressDeposit:  strToUInt64(bfpp.KeyDeposit),
		StakePoolDeposit:     strToUInt64(bfpp.PoolDeposit),
		MinPoolCost:          strToUInt64(bfpp.MinPoolCost),
		PoolRetireMaxEpoch:   bfpp.EMax,
		StakePoolTargetNum:   bfpp.NOpt,
		PoolPledgeInfluence:  bfpp.A0,
		MonetaryExpansion:    bfpp.Rho,
		TreasuryCut:          bfpp.Tau,
		CollateralPercentage: bfpp.CollateralPercent,
		ExecutionUnitPrices: NewProtocolParametersPriceMemorySteps(
			bfpp.PriceMem, bfpp.PriceStep),
		UtxoCostPerByte: strToUInt64(bfpp.CoinsPerUtxoWord),
		MaxTxExecutionUnits: NewProtocolParametersMemorySteps(
			strToUInt64(bfpp.MaxTxExMem), strToUInt64(bfpp.MaxTxExSteps)),
		MaxBlockExecutionUnits: NewProtocolParametersMemorySteps(
			strToUInt64(bfpp.MaxBlockExMem), strToUInt64(bfpp.MaxBlockExSteps)),
		MaxCollateralInputs: bfpp.MaxCollateralInputs,
		MaxValueSize:        strToUInt64(bfpp.MaxValSize),
		CostModels:          map[string][]int64{},
	}

	for scriptName, mapValue := range bfpp.CostModels {
		ints := make([]int64, len(mapValue))

		for k, v := range mapValue {
			if val, err := strconv.Atoi(k); err == nil {
				ints[val] = v
			}
		}

		pp.CostModels[scriptName] = ints
	}

	return json.Marshal(pp)
}

func getErrorFromResponse(resp *http.Response) error {
	var bfResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&bfResponse); err != nil {
		return fmt.Errorf("status code %d", resp.StatusCode)
	}

	return fmt.Errorf("status code %d: %s", resp.StatusCode, bfResponse["message"])
}
