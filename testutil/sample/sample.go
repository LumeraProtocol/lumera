package sample

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"io"
)

// AccAddress returns a sample account address
func AccAddress() string {
	return AccAddressAcc().String()
}

func AccAddressAcc() sdk.AccAddress {
	pk := ed25519.GenPrivKey().PubKey()
	addr := pk.Address()
	return sdk.AccAddress(addr)
}

func ValAddress() string {
	return ValAddressVal().String()
}

func ValAddressVal() sdk.ValAddress {
	pk := ed25519.GenPrivKey().PubKey()
	addr := pk.Address()
	return sdk.ValAddress(addr)
}

func SupernodeAddresses() (ed25519.PrivKey, sdk.AccAddress, sdk.ValAddress) {
	key := ed25519.GenPrivKey()
	addr := key.PubKey().Address()
	return *key, sdk.AccAddress(addr), sdk.ValAddress(addr)
}

func KeyAndAddress() (ed25519.PrivKey, sdk.AccAddress) {
	key := ed25519.GenPrivKey()
	addr := key.PubKey().Address()
	return *key, sdk.AccAddress(addr)
}

func SignString(privKey ed25519.PrivKey, data string) (string, error) {
	signatureBytes, err := privKey.Sign([]byte(data))
	if err != nil {
		return "", fmt.Errorf("failed to sign data: %w", err)
	}

	signatureB64 := base64.StdEncoding.EncodeToString(signatureBytes)
	return signatureB64, nil
}

func CreateSignatureString(privKeys []ed25519.PrivKey, dataLen int) (string, error) {
	// 1. Generate arbitrary data
	data := make([]byte, dataLen)
	if _, err := io.ReadFull(rand.Reader, data); err != nil {
		return "", fmt.Errorf("failed to generate random data: %w", err)
	}

	// 2. Base64-encode the data
	dataB64 := base64.StdEncoding.EncodeToString(data)
	result := dataB64

	for _, privKey := range privKeys {
		// 3. Sign the original data
		signatureBytes, err := privKey.Sign([]byte(dataB64))
		if err != nil {
			return "", fmt.Errorf("failed to sign data: %w", err)
		}

		// 4. verify the signature
		pubKey := privKey.PubKey()
		if !pubKey.VerifySignature([]byte(dataB64), signatureBytes) {
			return "", fmt.Errorf("signature verification failed")
		}

		// 5. Concatenate: "Base64(data).<signature>..."
		signatureB64 := base64.StdEncoding.EncodeToString(signatureBytes)
		result = fmt.Sprintf("%s.%s", result, signatureB64)
	}

	return result, nil
}
