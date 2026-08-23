# LegRoast APIs: Signing and Verification

## Overview of LegRoast
LegRoast is a post-quantum cryptographic signature scheme designed for efficiency and security. It leverages computational hardness assumptions, ensuring robustness against both classical and quantum attacks. 

LegRoast offers six algorithmic variants, each tailored to balance performance, signature size, and computational requirements:

### Supported Algorithms
1. **LegendreFast**: Optimized for fast computation but has larger signature sizes.
2. **LegendreMiddle**: Offers a balance between speed and signature size.
3. **LegendreCompact**: Focused on minimizing signature size at the cost of computational speed.
4. **PowerFast**: Fast computation, leveraging power-based optimizations.
5. **PowerMiddle**: Provides a middle ground between speed and signature size for power-based operations.
6. **PowerCompact**: Minimizes signature size with a power-based approach, requiring more computational resources.

Each variant is suited for different use cases depending on the constraints of your environment (e.g., network bandwidth, computational power).

### Algorithm Comparison Table
| **Algorithm**      | **Symbol Type** | **Focus**                  | **Memory Usage** | **Signature Size (bytes)** |
|---------------------|-----------------|----------------------------|------------------|----------------------------|
| **LegendreFast**    | Legendre        | Optimized for speed        | High             | 16,480                     |
| **LegendreMiddle**  | Legendre        | Balanced speed and size    | Medium           | 14,272                     |
| **LegendreCompact** | Legendre        | Minimized signature size   | Low              | 12,544                     |
| **PowerFast**       | Power           | Optimized for speed        | High             | 8,800                      |
| **PowerMiddle**     | Power           | Balanced speed and size    | Medium           | 7,408                      |
| **PowerCompact**    | Power           | Minimized signature size   | Low              | 6,448                      |

---

## LegRoast APIs
The LegRoast APIs provide functionality for signing and verifying messages in a Cosmos SDK-based blockchain application. These APIs integrate LegRoast into the blockchain�s cryptographic framework.

### 1. **LegRoast-Sign API**
#### Command: `legroast-sign`

This command signs a message using a private key derived deterministically from a Cosmos address. The signature process uses one of the supported LegRoast algorithms.

#### Key Features:
- Derives a cryptographic seed from the Cosmos address and a predefined passphrase.
- Initializes the LegRoast instance using the derived seed and the specified algorithm.
- Signs the input message and returns the resulting signature.

#### Usage:
```bash
lumerad query lumeraid legroast-sign [address] [text] --algo [algorithm] --output [json|text]
```

#### Parameters:
- `address`: The Cosmos address used to derive the private key.
- `text`: The message to be signed. The text can be raw or Base64-encoded.
- `--algo` (optional): The LegRoast algorithm to use. Defaults to `LegendreMiddle` if not specified.
- `--output` (optional): The output format. Supports `json` (default) or `text`.

#### Output:
- **JSON format (default)**:
```json
{
  "address": "lumera1zvnc27832srgxa207y5hu2agy83wazfzurufyp"
  "algorithm": "LegendreFast",
  "public_key": "Base64EncodedPublicKey"
  "signature": "Base64EncodedSignature",
}
```

#### Example:
```bash
lumerad query lumeraid legroast-sign lumera1zvnc27832srgxa207y5hu2agy83wazfzurufyp "Hello, LegRoast!" --algo LegendreFast --output json
```

#### **Text Output Example**:
```bash
Base64EncodedSignature
```

---

### 2. **LegRoast-Verify API**
#### Command: `legroast-verify`

This command verifies a message against a given signature and public key using the LegRoast algorithm.

#### Key Features:
- Verifies the authenticity of the signature for a given message and public key.
- Ensures compatibility with the LegRoast algorithm used during signing.
- Legroast algorithm used for signing is automatically detected by signature size.

#### Usage:
```bash
lumerad query lumeraid legroast-verify [text] [pubkey] [signature] --output [json|text]
```

#### Parameters:
- `text`: The original message. Can be raw or Base64-encoded.
- `pubkey`: The public key used for verification (Base64-encoded).
- `signature`: The signature to verify (Base64-encoded).
- `--output` (optional): The output format. Supports `json` (default) or `text`.

#### Output:
- **JSON format (default)**:
```json
{
  "verified": true,
}
```

#### Example:
```bash
lumerad query lumeraid legroast-verify "Hello, LegRoast!" "Base64EncodedPubKey" "Base64EncodedSignature" --output text
```

#### **Text Output Example**:
```bash
Verification successful
```

---

## Key Design Principles
1. **Deterministic Key Derivation**: The `legroast-sign` API derives the private key from the Cosmos address and a passphrase, ensuring consistency across signings for the same address.
2. **Flexibility**: Supports multiple LegRoast algorithms to suit various application requirements.
3. **Post-Quantum Security**: Ensures cryptographic security in a future-proof manner against quantum attacks.

---


By integrating LegRoast with Cosmos SDK, these APIs provide a robust, quantum-resistant solution for signing and verifying messages in a blockchain environment.

