package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"cosmossdk.io/math"
	lumeracrypto "github.com/LumeraProtocol/lumera/x/claim/keeper/crypto"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type StringInt64 int64

// UnmarshalJSON - Custom JSON unmarshaler for int64
func (si *StringInt64) UnmarshalJSON(data []byte) error {
	var strVal string
	if err := json.Unmarshal(data, &strVal); err != nil {
		return err
	}
	// Handle empty string
	if strVal == "" {
		*si = 0
		return nil
	}
	// Convert string to int64
	val, err := strconv.ParseInt(strVal, 10, 64)
	if err != nil {
		return err
	}
	*si = StringInt64(val)
	return nil
}

// BroadcastTxResponse - Custom response type that matches the JSON structure
type BroadcastTxResponse struct {
	TxResponse struct {
		Height    StringInt64          `json:"height"`
		TxHash    string               `json:"txhash"`
		Codespace string               `json:"codespace"`
		Code      uint32               `json:"code"`
		Data      string               `json:"data"`
		RawLog    string               `json:"raw_log"`
		Logs      []sdk.ABCIMessageLog `json:"logs"`
		Info      string               `json:"info"`
		GasWanted StringInt64          `json:"gas_wanted"`
		GasUsed   StringInt64          `json:"gas_used"`
		Tx        any                  `json:"tx"`
		Timestamp string               `json:"timestamp"`
		Events    []sdk.StringEvent    `json:"events"`
	} `json:"tx_response"`
}

// EncodingConfig specifies the concrete encoding types to use for a given app.
// This is provided for compatibility between protobuf and amino implementations.
type EncodingConfig struct {
	InterfaceRegistry types.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

// Config - Configuration struct
type Config struct {
	NodeURL        string       `json:"node_url"`        // URL for local lumerad node
	FaucetKeyName  string       `json:"faucet_key_name"` // Name of faucet account
	FaucetAddress  string       `json:"faucet_address"`  // Address to send from
	FaucetMnemonic string       `json:"faucet_mnemonic"` // Mnemonic for faucet account
	FeeAmount      sdk.Coins    `json:"fee_amount"`      // Amount to send for fees
	ChainID        string       `json:"chain_id"`        // Chain ID
	Gas            uint64       `json:"gas"`             // Gas limit
	GasAdjustment  float64      `json:"gas_adjustment"`  // Gas adjustment
	GasPrice       sdk.DecCoins `json:"gas_price"`       // Gas price
}

// FaucetRequest - Request struct for the faucet endpoint
type FaucetRequest struct {
	OldAddress string `json:"old_address"`
	OldPubKey  string `json:"old_pub_key"`
	NewAddress string `json:"new_address"`
	Signature  string `json:"signature"`
}

// FaucetResponse - Response struct
type FaucetResponse struct {
	TxHash string    `json:"tx_hash"`
	Amount sdk.Coins `json:"amount"`
}

type Server struct {
	config         Config
	client         client.Context
	logger         *log.Logger
	encodingConfig EncodingConfig
}

func makeEncodingConfig() EncodingConfig {
	amino := codec.NewLegacyAmino()
	interfaceRegistry := types.NewInterfaceRegistry()

	// Register crypto interfaces
	cryptocodec.RegisterInterfaces(interfaceRegistry)

	// Register auth interfaces
	authtypes.RegisterInterfaces(interfaceRegistry)

	// Register bank interfaces
	banktypes.RegisterInterfaces(interfaceRegistry)

	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txConfig := authtx.NewTxConfig(marshaler, authtx.DefaultSignModes)

	return EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             marshaler,
		TxConfig:          txConfig,
		Amino:             amino,
	}
}

func createClientContext(config Config, encodingConfig EncodingConfig) (client.Context, error) {
	// Initialize SDK configuration
	sdkConfig := sdk.GetConfig()
	sdkConfig.SetBech32PrefixForAccount("lumera", "lumerapub")
	sdkConfig.SetBech32PrefixForValidator("lumeravaloper", "lumeravaloperpub")
	sdkConfig.SetBech32PrefixForConsensusNode("lumeravalcons", "lumeravalconspub")
	sdkConfig.Seal()

	// Create keyring
	kb, err := keyring.New(
		"lumera",
		keyring.BackendMemory,
		"",
		nil,
		encodingConfig.Codec,
	)
	if err != nil {
		return client.Context{}, fmt.Errorf("failed to create keyring: %w", err)
	}

	// Import faucet account
	_, err = kb.NewAccount(
		config.FaucetKeyName,
		config.FaucetMnemonic,
		keyring.DefaultBIP39Passphrase,
		sdk.FullFundraiserPath,
		hd.Secp256k1,
	)
	if err != nil {
		return client.Context{}, fmt.Errorf("failed to import faucet account: %w", err)
	}

	// Create client context
	clientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithOutput(os.Stdout).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithBroadcastMode("block").
		WithHomeDir(os.ExpandEnv("$HOME/.lumera")).
		WithKeyring(kb).
		WithChainID(config.ChainID).
		WithNodeURI(config.NodeURL)

	return clientCtx, nil
}

func NewServer(configPath string) (*Server, error) {
	// Read and parse config
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Create encoding config
	encodingConfig := makeEncodingConfig()

	// Initialize client context
	clientCtx, err := createClientContext(config, encodingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create client context: %w", err)
	}

	return &Server{
		config:         config,
		client:         clientCtx,
		logger:         log.New(os.Stdout, "[FAUCET] ", log.LstdFlags|log.Lshortfile),
		encodingConfig: encodingConfig,
	}, nil
}

func (s *Server) handleGetFeeForClaiming(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		return // Middleware already handled it
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req FaucetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 1. Query if account exists
	accountURL := fmt.Sprintf("%s/cosmos/auth/v1beta1/accounts/%s", s.config.NodeURL, req.NewAddress)

	resp, err := http.Get(accountURL)
	if err != nil {
		http.Error(w, "Failed to query account", http.StatusInternalServerError)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			http.Error(w, "Failed to close response body", http.StatusInternalServerError)
		}
	}(resp.Body)

	// If account exists and response is 200, return error
	//if resp.StatusCode == http.StatusOK {
	//	http.Error(w, "Account already exists", http.StatusBadRequest)
	//	return
	//}

	// 2. Query claim record
	claimRecordURL := fmt.Sprintf("%s/LumeraProtocol/lumera/claim/claim_record/%s", s.config.NodeURL, req.OldAddress)

	resp, err = http.Get(claimRecordURL)
	if err != nil {
		http.Error(w, "Failed to query claim record", http.StatusInternalServerError)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			http.Error(w, "Failed to close response body", http.StatusInternalServerError)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Claim record not found", http.StatusBadRequest)
		return
	}

	var claimResp struct {
		Record struct {
			Balance []struct {
				Amount string `json:"amount"`
				Denom  string `json:"denom"`
			} `json:"balance"`
			Claimed    bool   `json:"claimed"`
			OldAddress string `json:"oldAddress"`
		} `json:"record"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&claimResp); err != nil {
		http.Error(w, "Failed to decode claim record", http.StatusInternalServerError)
		return
	}

	// Check if claim exists and is not claimed
	if claimResp.Record.Claimed {
		http.Error(w, "Claim already processed", http.StatusBadRequest)
		return
	}
	if claimResp.Record.OldAddress != req.OldAddress {
		http.Error(w, "Mismatch in claim address", http.StatusBadRequest)
		return
	}
	if len(claimResp.Record.Balance) == 0 {
		http.Error(w, "Claim record has no balance", http.StatusBadRequest)
		return
	}

	// 3. Verify signature
	// Verify address reconstruction and signature
	reconstructedAddress, err := lumeracrypto.GetAddressFromPubKey(req.OldPubKey)
	if err != nil {
		http.Error(w, "Invalid public key", http.StatusBadRequest)
		return
	}

	if reconstructedAddress != req.OldAddress {
		http.Error(w, "Mismatch in reconstructed address", http.StatusBadRequest)
		return
	}

	verificationMessage := req.OldAddress + "." + req.OldPubKey + "." + req.NewAddress
	valid, err := lumeracrypto.VerifySignature(req.OldPubKey, verificationMessage, req.Signature)
	if err != nil || !valid {
		http.Error(w, "Invalid signature", http.StatusBadRequest)
		return
	}

	// 4. Send fee amount
	msg := &banktypes.MsgSend{
		FromAddress: s.config.FaucetAddress,
		ToAddress:   req.NewAddress,
		Amount:      s.config.FeeAmount,
	}

	// Create and sign transaction
	txHash, err := s.broadcastTx(msg)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send transaction: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare and send response
	response := FaucetResponse{
		TxHash: txHash,
		Amount: s.config.FeeAmount,
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (s *Server) broadcastTx(msg sdk.Msg) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Get account number and sequence
	accountURL := fmt.Sprintf("%s/cosmos/auth/v1beta1/accounts/%s",
		s.config.NodeURL, s.config.FaucetAddress)

	resp, err := http.Get(accountURL)
	if err != nil {
		return "", fmt.Errorf("failed to get account info: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			s.logger.Printf("Failed to close response body: %v", err)
		}
	}(resp.Body)

	var accResp struct {
		Account struct {
			AccountNumber string `json:"account_number"`
			Sequence      string `json:"sequence"`
		} `json:"account"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&accResp); err != nil {
		return "", fmt.Errorf("failed to decode account response: %w", err)
	}

	accNum, err := math.ParseUint(accResp.Account.AccountNumber)
	if err != nil {
		return "", fmt.Errorf("failed to parse account number: %w", err)
	}

	seq, err := math.ParseUint(accResp.Account.Sequence)
	if err != nil {
		return "", fmt.Errorf("failed to parse sequence: %w", err)
	}

	// Create transaction factory
	factory := tx.Factory{}.
		WithTxConfig(s.client.TxConfig).
		WithKeybase(s.client.Keyring).
		WithAccountNumber(accNum.Uint64()).
		WithSequence(seq.Uint64()).
		WithGas(s.config.Gas).
		WithGasAdjustment(s.config.GasAdjustment).
		WithChainID(s.config.ChainID).
		WithGasPrices(s.config.GasPrice.String()).
		WithSignMode(signingtypes.SignMode_SIGN_MODE_DIRECT)

	// Build unsigned transaction
	txBuilder, err := factory.BuildUnsignedTx(msg)
	if err != nil {
		return "", fmt.Errorf("failed to build unsigned tx: %w", err)
	}

	// Sign transaction
	err = tx.Sign(ctx, factory, s.config.FaucetKeyName, txBuilder, true)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Get the encoded transaction bytes
	txBytes, err := s.client.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return "", fmt.Errorf("failed to encode transaction: %w", err)
	}

	// Convert to base64 instead of hex
	encodedTx := base64.StdEncoding.EncodeToString(txBytes)

	// Broadcast transaction
	txURL := fmt.Sprintf("%s/cosmos/tx/v1beta1/txs", s.config.NodeURL)

	broadcastReq := struct {
		TxBytes string `json:"tx_bytes"`
		Mode    string `json:"mode"`
	}{
		TxBytes: encodedTx, // Use base64 encoded transaction
		Mode:    "BROADCAST_MODE_SYNC",
	}

	txJSON, err := json.Marshal(broadcastReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal broadcast request: %w", err)
	}

	s.logger.Printf("Broadcasting transaction to %s", txURL)
	s.logger.Printf("Request body: %s", string(txJSON))

	resp, err = http.Post(txURL, "application/json", bytes.NewReader(txJSON))
	if err != nil {
		return "", fmt.Errorf("failed to broadcast transaction: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			s.logger.Printf("Failed to close response body: %v", err)
		}
	}(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	s.logger.Printf("Response status: %d", resp.StatusCode)
	s.logger.Printf("Response body: %s", string(bodyBytes))

	var broadcastResp BroadcastTxResponse
	if err := json.Unmarshal(bodyBytes, &broadcastResp); err != nil {
		return "", fmt.Errorf("failed to decode broadcast response: %w, body: %s", err, string(bodyBytes))
	}

	s.logger.Printf("Transaction response: Code=%d, TxHash=%s, RawLog=%s",
		broadcastResp.TxResponse.Code, broadcastResp.TxResponse.TxHash, broadcastResp.TxResponse.RawLog)

	if broadcastResp.TxResponse.Code != 0 {
		return "", fmt.Errorf("transaction failed: %s", broadcastResp.TxResponse.RawLog)
	}

	return broadcastResp.TxResponse.TxHash, nil
}

func loggingMiddleware(logger *log.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Printf("Request: %s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}

func recoveryMiddleware(logger *log.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Printf("Panic recovered: %v", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Create server
	server, err := NewServer("config.json")
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Create router
	r := mux.NewRouter()
	r.HandleFunc("/api/getfeeforclaiming", server.handleGetFeeForClaiming).Methods("POST", "OPTIONS")

	// Add middleware
	r.Use(loggingMiddleware(server.logger))
	r.Use(recoveryMiddleware(server.logger))
	r.Use(corsMiddleware)

	// Start server
	addr := ":8080"
	server.logger.Printf("Starting faucet server on %s", addr)
	server.logger.Printf("ChainID: %s", server.config.ChainID)
	server.logger.Printf("Gas limit: %d", server.config.Gas)
	server.logger.Printf("Gas Price: %s", server.config.GasPrice)
	server.logger.Printf("Gas Adjustment: %f", server.config.GasAdjustment)
	server.logger.Printf("Faucet FeeAmount: %s", server.config.FeeAmount)

	if err := http.ListenAndServe(addr, r); err != nil {
		server.logger.Fatalf("Failed to start server: %v", err)
	}
}
