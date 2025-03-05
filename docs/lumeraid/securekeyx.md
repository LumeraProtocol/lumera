# Lumeraid Module: SecureKeyX API Documentation

This document describes the SecureKeyX API in the `lumeraid` module for enabling secure key exchange and shared secret computation between peers using Cosmos accounts.

---

## **API Overview**

The SecureKeyX implementation provides secure key exchange between different types of peers (Simplenodes and Supernodes) using Cosmos accounts for authentication. The implementation leverages ECDH (Elliptic Curve Diffie-Hellman) for generating shared secrets with forward secrecy.

## **Peer Types**

The system supports two types of peers:

- **Simplenode**: Regular node in the network
- **Supernode**: Validator node with additional privileges (verified against chain state)

---

## **Using the SecureKeyX API**

The `SecureKeyExchange` type implements the `KeyExchanger` interface, which provides methods for secure key exchange. Here's how to use it:

### **1. Creating a SecureKeyExchange Instance**

```go
// Initialize a SecureKeyExchange instance
keyExchanger, err := NewSecureKeyExchange(
    kr,                // cosmos-sdk keyring.Keyring
    localAddress,      // string - local Cosmos address (bech32)
    localPeerType,     // PeerType - Simplenode or Supernode
    curve,             // ecdh.Curve - optional, defaults to P256
)
```

### **2. Initiating Key Exchange**

To initiate a key exchange, the local peer creates a request containing its ephemeral public key:

```go
// Generate handshake info and signature
handshakeBytes, signature, err := keyExchanger.CreateRequest(remoteAddress)
if err != nil {
    // Handle error
}

// Send handshakeBytes and signature to the remote peer
// through your communication channel
```

The handshake info contains:

- Local address
- Local Peer type
- Ephemeral public key
- Curve information (as a string: "P256", "P384", or "P521")

### **3. Processing a Key Exchange Request**

When receiving a key exchange request, the remote peer computes the shared secret:

```go
// Compute shared secret using received handshake info and signature
sharedSecret, err := keyExchanger.ComputeSharedSecret(handshakeBytes, signature)
if err != nil {
    // Handle error - could be invalid signature, unauthorized peer, etc.
}

// Use sharedSecret for secure communication
```

This method:

1. Deserializes the handshake info
2. Retrieves the ephemeral private key for the corresponding remote address
3. Validates the signature using the remote peer's Cosmos account
4. Verifies supernode status if applicable
5. Computes and returns the shared secret

### **4. Getting Local Information**

You can retrieve information about the local peer:

```go
// Get local peer type
peerType := keyExchanger.PeerType()

// Get local address
address := keyExchanger.LocalAddress()
```

---

## **Complete Exchange Flow**

A complete key exchange between two peers involves the following steps:

### **Peer A (Initiator):**

```go
// 1. Create key exchanger for Peer A
keyExchangerA, err := NewSecureKeyExchange(keyringA, addressA, peerTypeA, nil)
if err != nil {
    // Handle error
}

// 2. Create request to send to Peer B
handshakeBytesA, signatureA, err := keyExchangerA.CreateRequest(addressB)
if err != nil {
    // Handle error
}

// 3. Send handshakeBytesA and signatureA to Peer B
// ...communication code...

// 6. Receive handshakeBytesB and signatureB from Peer B
// ...communication code...

// 7. Compute shared secret
sharedSecret, err := keyExchangerA.ComputeSharedSecret(handshakeBytesB, signatureB)
if err != nil {
    // Handle error
}

// 8. Use shared secret for secure communication
```

### **Peer B (Responder):**

```go
// 4. Create key exchanger for Peer B
keyExchangerB, err := NewSecureKeyExchange(keyringB, addressB, peerTypeB, nil)
if err != nil {
    // Handle error
}

// 5. Create request to send to Peer A
handshakeBytesB, signatureB, err := keyExchangerB.CreateRequest(addressA)
if err != nil {
    // Handle error
}

// Send handshakeBytesB and signatureB to Peer A
// ...communication code...

// Receive handshake data from Peer A
// ...communication code...

// Compute shared secret from Peer A's data
sharedSecret, err := keyExchangerB.ComputeSharedSecret(handshakeBytesA, signatureA)
if err != nil {
    // Handle error
}
```

**Important Note**: Both peers must call `CreateRequest()` with the other peer's address and exchange the resulting handshake bytes and signatures. Each peer then calls `ComputeSharedSecret()` with the received handshake bytes and signature. The same shared secret will be computed on both sides.

---

## **Security Features**

1. **Forward Secrecy**:
   - Ephemeral keys are generated for each session
   - Keys are deleted after shared secret computation

2. **Authentication**:
   - All handshake information is signed using Cosmos accounts
   - Signatures are verified against on-chain accounts

3. **Supernode Validation**:
   - Peers claiming to be supernodes are validated against chain state

4. **Curve Options**:
   - Supports multiple ECDH curves with different security levels:
     - P256: 128-bit security (fast, suitable for most applications)
     - P384: 192-bit security (moderate performance)
     - P521: 256-bit security (highest security, slower performance)

---

## **Implementation Notes**

1. **Multiple Remote Peers**:
   - A single `SecureKeyExchange` instance can handle secure handshakes with multiple remote peers
   - The implementation maintains separate ephemeral keys for each remote peer address
   - This allows a node to establish secure channels with many peers simultaneously using the same local identity

2. **Ephemeral Key Management**:
   - Ephemeral private keys are stored in a protected map keyed by remote address
   - Keys are automatically deleted after computing the shared secret
   - A mutex protects concurrent access to the key map

3. **Error Handling**:
   - The API provides detailed error messages to help diagnose issues
   - Common errors include invalid addresses, signature verification failures, missing ephemeral keys, and unauthorized supernode claims

4. **Performance Considerations**:
   - P256 is the default curve and provides good performance for most use cases
   - For higher security requirements, P384 or P521 can be specified but will be slower

5. **Best Practices**:
   - The shared secret should be used with appropriate key derivation functions before using it for encryption
   - For long-term sessions, periodic re-keying is recommended
   - Always validate both ends of the communication
   - The ephemeral keys are stored in memory until `ComputeSharedSecret` is called, so it's important to complete the exchange process

6. **Error Scenarios**:
   - If `CreateRequest` is called multiple times for the same remote address before completing the exchange, the previous ephemeral key will be overwritten
   - If the ephemeral key for a remote address is not found when calling `ComputeSharedSecret`, an error will be returned

---

For further assistance or feedback, please contact the LumeraNetwork team.