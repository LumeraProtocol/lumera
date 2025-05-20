//go:generate mockgen -copyright_file=../../../testutil/mock_header.txt -destination=../mocks/securekeyx_mocks.go -package=lumeraidmocks -source=securekeyx.go

package securekeyx

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	sdkcodec "github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	proto "github.com/cosmos/gogoproto/proto"

	lumeraidtypes "github.com/LumeraProtocol/lumera/x/lumeraid/types"
	sntypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

type PeerType int

const (
	accountValidationTimeout = 5 * time.Second

	// PeerType represents the type of peer in the network.
	Simplenode PeerType = iota
	Supernode
)

// KeyExchanger defines the interface for secure key exchange between peers using Cosmos accounts.
type KeyExchanger interface {
	// CreateRequest generates handshake info and signs it with the specified Cosmos account.
	CreateRequest(remoteAddress string) ([]byte, []byte, error)
	// ComputeSharedSecret computes the shared secret using the ephemeral private key and the remote public key.
	ComputeSharedSecret(handshakeBytes, signature []byte) ([]byte, error)
	// PeerType returns the type of the local peer
	PeerType() PeerType
	// LocalAddress returns the local Cosmos address
	LocalAddress() string
}

type KeyExchangerValidator interface {
	// AccountInfoByAddress gets the account info by address
	AccountInfoByAddress(ctx context.Context, addr string) (*authtypes.QueryAccountInfoResponse, error)
	// GetSupernodeBySupernodeAddress gets the supernode info by supernode address
	GetSupernodeBySupernodeAddress(ctx context.Context, address string) (*sntypes.SuperNode, error)
}

type SecureKeyExchange struct {
	keyring    keyring.Keyring       // keyring to access Cosmos accounts
	accAddress sdk.AccAddress        // local Cosmos address
	peerType   PeerType              // local peer type (Simplenode or Supernode)
	curve      ecdh.Curve            // curve used for ECDH key exchange
	validator  KeyExchangerValidator // validator to check if the account is a valid

	mutex         sync.Mutex                  // mutex to protect ephemeralKeys
	ephemeralKeys map[string]*ecdh.PrivateKey // map of [remote_address -> ephemeral private keys]
	codec         *sdkcodec.ProtoCodec        // codec for serialization/deserialization
}

/*
Performance and Security Comparison of the curves supported in ECDH Go package

+--------+----------+----------------+-------------+---------------------------------------+
| Curve  | Bit Size | Security Level | Performance | Use Case                              |
+--------+----------+----------------+-------------+---------------------------------------+
| P-256  | 256 bits | 128 bits       | Fast        | General-purpose, TLS, mobile apps.    |
| P-384  | 384 bits | 192 bits       | Moderate    | Higher security, sensitive data.      |
| P-521  | 521 bits | 256 bits       | Slow        | Extreme security, niche applications. |
+--------+----------+----------------+-------------+---------------------------------------+
*/

// Helper to get curve name
func (s *SecureKeyExchange) GetCurveName() string {
	switch s.curve {
	case ecdh.P256():
		return "P256"
	case ecdh.P384():
		return "P384"
	case ecdh.P521():
		return "P521"
	default:
		return "Unknown"
	}
}

// validateSupernode checks if the given account belongs to a valid supernode.
func (s *SecureKeyExchange) validateSupernode(accAddress sdk.AccAddress) error {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), accountValidationTimeout)
	defer cancel()

	supernode, err := s.validator.GetSupernodeBySupernodeAddress(ctx, accAddress.String())
	if err != nil || supernode == nil {
		return fmt.Errorf("supernode peer cannot be verified: %w", err)
	}

	// GetSupernodeAccount returns string
	// Check if the account address matches the expected address
	if !accAddress.Equals(sdk.AccAddress(supernode.GetSupernodeAccount())) {
		return fmt.Errorf("supernode account address mismatch: expected %s, got %s", accAddress.String(),
			supernode.GetSupernodeAccount())
	}

	return nil
}

// checkAcountExistsGRPC checks if the given account exists in the local chain's auth keeper using gRPC.
func (s *SecureKeyExchange) checkAccountExists(accAddress sdk.AccAddress) error {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), accountValidationTimeout)
	defer cancel()

	resp, err := s.validator.AccountInfoByAddress(ctx, accAddress.String())
	if err != nil {
		return fmt.Errorf("account cannot be verified: %w", err)
	}
	if resp == nil || resp.Info == nil || resp.Info.GetAddress() == nil {
		return fmt.Errorf("account info is nil")
	}

	// Check if the account address matches the expected address
	if !accAddress.Equals(resp.Info.GetAddress()) {
		return fmt.Errorf("account address mismatch: expected %s, got %s", accAddress.String(),
			resp.Info.GetAddress().String())
	}
	return nil
}

// NewSecureKeyExchange creates a new instance of SecureCommManager.
//
// Parameters:
//   - kr: keyring to access Cosmos accounts
//   - localAddress: the Cosmos address of the local peer
//   - localPeerType: the type of the local peer (Simplenode or Supernode)
//   - curve: the curve to be used for ECDH key exchange (default is P256)
//
// Returns:
//   - SecureKeyExchange: the instance of SecureKeyExchange
//   - error: if any error occurs
func NewSecureKeyExchange(
	kr keyring.Keyring,
	localAddress string,
	localPeerType PeerType,
	curve ecdh.Curve,
	validator KeyExchangerValidator,
) (*SecureKeyExchange, error) {
	accAddress, err := sdk.AccAddressFromBech32(localAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}
	if _, err := kr.KeyByAddress(accAddress); err != nil {
		return nil, fmt.Errorf("address not found in keyring: %w", err)
	}
	if curve == nil {
		curve = ecdh.P256()
	}
	if validator == nil {
		return nil, fmt.Errorf("KeyExchanger validator is required")
	}

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	protoCodec := sdkcodec.NewProtoCodec(interfaceRegistry)

	ske := &SecureKeyExchange{
		keyring:       kr,
		accAddress:    accAddress,
		peerType:      localPeerType,
		curve:         curve,
		ephemeralKeys: make(map[string]*ecdh.PrivateKey),
		codec:         protoCodec,
		validator:     validator,
	}

	// validate local peer
	if err := ske.checkAccount(accAddress, localPeerType); err != nil {
		return nil, fmt.Errorf("invalid local peer: %w", err)
	}

	return ske, nil
}

// Helper to sign data with keyring.
func (s *SecureKeyExchange) signWithKeyring(data []byte) ([]byte, error) {
	signature, _, err := s.keyring.SignByAddress(s.accAddress, data, signingtypes.SignMode_SIGN_MODE_DIRECT)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	return signature, nil
}

func (s *SecureKeyExchange) checkAccount(accAddress sdk.AccAddress, peerType PeerType) error {
	if peerType == Supernode {
		return s.validateSupernode(accAddress)
	} else if peerType == Simplenode {
		return s.checkAccountExists(accAddress)
	}
	return fmt.Errorf("invalid peer type: %d", peerType)
}

// Helper to validate signature received from remote peer.
//
// Parameters:
//   - pubKey: public key of the remote peer's Cosmos account
//   - data: the data to be verified
//   - signature: signature
//
// Returns:
//   - true if the signature is valid
//   - error if any
func (s *SecureKeyExchange) validateSignature(pubKey cryptotypes.PubKey, data, signature []byte) (bool, error) {
	if pubKey == nil {
		return false, fmt.Errorf("public key is nil")
	}

	if !pubKey.VerifySignature(data, signature) {
		return false, fmt.Errorf("invalid signature")
	}
	return true, nil
}

// PeerType returns the type of the peer
func (s *SecureKeyExchange) PeerType() PeerType {
	return s.peerType
}

// LocalAddress returns the local address
func (s *SecureKeyExchange) LocalAddress() string {
	return s.accAddress.String()
}

func (s *SecureKeyExchange) getLocalPubKey() (cryptotypes.PubKey, error) {
	cryptoLocalAddr := sdk.AccAddress(s.accAddress)
	// Get public key for the local Cosmos account
	keyInfo, err := s.keyring.KeyByAddress(cryptoLocalAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get key info for local address: %w", err)
	}

	pubKey, err := keyInfo.GetPubKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get public key for local address: %w", err)
	}

	return pubKey, nil
}

// CreateRequest generates handshake info and signs it with the local address.
//
// Parameters:
//   - remoteAddress: the address of the remote peer
//
// Returns:
//   - handshakeBytes: the serialized handshake info to be sent to the remote peer
//   - signature: signature of the handshake info (signed with the s.accAddress)
//   - error: if any error occurs
func (s *SecureKeyExchange) CreateRequest(remoteAddress string) ([]byte, []byte, error) {
	if s.curve == nil {
		return nil, nil, fmt.Errorf("curve not set")
	}

	// Generate ephemeral key pair
	privKey, err := s.curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate ephemeral key for %s: %w", remoteAddress, err)
	}

	// store ephemeral private key temporarily until shared secret is computed
	s.mutex.Lock()
	s.ephemeralKeys[remoteAddress] = privKey
	s.mutex.Unlock()

	// Get public key for the local Cosmos account
	accountPubKey, err := s.getLocalPubKey()
	if err != nil {
		return nil, nil, err
	}

	accountPubKeyBytes, err := s.codec.MarshalInterface(accountPubKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal local account public key: %w", err)
	}

	// Create handshake info
	handshakeInfo := &lumeraidtypes.HandshakeInfo{
		Address:          s.LocalAddress(),
		PeerType:         int32(s.peerType),
		PublicKey:        privKey.PublicKey().Bytes(),
		AccountPublicKey: accountPubKeyBytes,
		Curve:            s.GetCurveName(),
	}

	// Serialize HandshakeInfo
	handshakeBytes, err := proto.Marshal(handshakeInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to serialize handshake info: %w", err)
	}

	// Sign handshake info with the private key from keyring
	signature, err := s.signWithKeyring(handshakeBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign handshake info: %w", err)
	}

	return handshakeBytes, signature, nil
}

// ComputeSharedSecret computes the shared secret using the ephemeral private key and the remote public key.
// It also validates the signature of the handshake info.
//
// Parameters:
//   - handshakeBytes: the serialized handshake info received from the remote peer
//   - signature: signature for the handshakeBytes (signed with the remote peer's Cosmos account)
//
// Returns:
//   - sharedSecret: the computed shared secret
//   - error: if any error occurs
func (s *SecureKeyExchange) ComputeSharedSecret(handshakeBytes, signature []byte) ([]byte, error) {
	if s.curve == nil {
		return nil, fmt.Errorf("curve not set")
	}

	var handshake lumeraidtypes.HandshakeInfo
	if err := proto.Unmarshal(handshakeBytes, &handshake); err != nil {
		return nil, fmt.Errorf("failed to deserialize handshake info: %w", err)
	}

	// Retrieve ephemeral private key
	s.mutex.Lock()
	// Check if ephemeral private key exists
	privKey, exists := s.ephemeralKeys[handshake.Address]
	if exists {
		// Remove ephemeral private key from the map after retrieving it
		delete(s.ephemeralKeys, handshake.Address)
	}
	s.mutex.Unlock()
	if !exists {
		return nil, fmt.Errorf("ephemeral private key not found for address: %s", handshake.Address)
	}

	if handshake.AccountPublicKey == nil {
		return nil, fmt.Errorf("account public key is nil")
	}

	var accountPubKey cryptotypes.PubKey
	if err := s.codec.UnmarshalInterface(handshake.AccountPublicKey, &accountPubKey); err != nil {
		return nil, fmt.Errorf("failed to unmarshal remote account's public key: %w", err)
	}

	derivedAddr := sdk.AccAddress(accountPubKey.Address()).String()
	if derivedAddr != handshake.Address {
		return nil, fmt.Errorf("address mismatch: expected %s, got %s", derivedAddr, handshake.Address)
	}

	// Validate signature for the handshake info from the remote peer
	isValid, err := s.validateSignature(accountPubKey, handshakeBytes, signature)
	if err != nil || !isValid {
		return nil, fmt.Errorf("signature validation failed: %w", err)
	}

	// Validate remote peer
	switch remotePeerType := PeerType(handshake.PeerType); remotePeerType {
	case Simplenode, Supernode:
		remoteAccAddress, err := sdk.AccAddressFromBech32(handshake.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid remote address: %w", err)
		}
		if err := s.checkAccount(remoteAccAddress, remotePeerType); err != nil {
			return nil, fmt.Errorf("invalid remote peer: %w", err)
		}
	default:
		return nil, fmt.Errorf("invalid remote peer type: %d", handshake.PeerType)
	}

	// Compute shared secret
	remotePubKey, err := s.curve.NewPublicKey(handshake.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote public key: %w", err)
	}

	sharedSecret, err := privKey.ECDH(remotePubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	return sharedSecret, nil
}
