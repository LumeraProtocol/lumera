//go:generate mockgen --copyright_file=../../../testutil/mock_header.txt -destination=../mocks/legroast_mocks.go -package=lumeraidmocks -source=legroast.go

package legroast

/**********************************************************************
 * \file   legroast.go
 * \brief  Post-Quantum signatures based on the Legendre PRF
 * Based on LegRoast implementation https://github.com/WardBeullens/LegRoast
 * by Ward Beullens
 *
 * Copyright (c) 2021-2025 The Lumera developers
 *********************************************************************/

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"golang.org/x/crypto/sha3"
	"lukechampine.com/uint128"
)

// ProverState represents the prover state for the LegRoast algorithm.
type ProverState struct {
	Params    *LegRoastParams
	SeedTrees *seedTree
	Shares    *Uint128Matrix      // [NRounds][Parties][SharesPerParty]
	Sums      [][]uint128.Uint128 // [NRounds][SharesPerParty]
	Indices   []uint128.Uint128   // [RESSYMPerRound]
}

// newProverState initializes a new ProverState
func newProverState(alg LegRoastAlgorithm) *ProverState {
	params := GetLegRoastParams(alg)

	sums := make([][]uint128.Uint128, params.NRounds)
	for i := range sums {
		sums[i] = make([]uint128.Uint128, params.SharesPerParty)
	}

	indices := make([]uint128.Uint128, params.RESSYMPerRound)

	return &ProverState{
		Params:    params,
		SeedTrees: newSeedTree(params),
		Shares:    NewUint128Matrix(params.NRounds, params.Parties, params.SharesPerParty),
		Sums:      sums,
		Indices:   indices,
	}
}

// Clear resets the prover state.
func (ps *ProverState) Clear() {
	ps.SeedTrees.Clear()
	ps.Shares.Clear()
	for i := range ps.Sums {
		for j := range ps.Sums[i] {
			ps.Sums[i][j] = uint128.Uint128{}
		}
	}
	for i := range ps.Indices {
		ps.Indices[i] = uint128.Uint128{}
	}
}

// getBit returns the value of the bit at the specified index.
func (lr *LegRoast) getBit(nBit uint32) byte {
	if lr.isLegendre {
		// Calculate the byte index and bit position within the byte
		byteIndex := nBit / 8
		bitPosition := nBit % 8

		// Check if the bit is set and return 1 or 0
		if lr.pk[byteIndex]&(1<<bitPosition) > 0 {
			return 1
		}
		return 0
	}

	// Return the value of the byte at the specified index
	return lr.pk[nBit]
}

// LegRoastInterface defines the interface for the LegRoast class.
type LegRoastInterface interface {
	Keygen(seed []byte) error
	Sign(message []byte) ([]byte, error)
	Verify(message, signature []byte) error
	SetPublicKey(pk []byte) error
	PublicKey() []byte
	Params() *LegRoastParams
}

// LegRoast represents the LegRoast class with templated algorithm.
type LegRoast struct {
	pk          [PkBytes]byte // public key
	sk          [SkBytes]byte // secret key
	proverState *ProverState  // prover state
	isLegendre  bool          // is Legendre-based algorithm
}

// NewLegRoast initializes a new instance of LegRoast.
var NewLegRoast = func(alg LegRoastAlgorithm) LegRoastInterface {
	return &LegRoast{
		proverState: newProverState(alg),
		isLegendre:  alg < PowerFast,
	}
}

// Params returns the parameters for the given LegRoast algorithm.
func (lr *LegRoast) Params() *LegRoastParams {
	return lr.proverState.Params
}

// SetPublicKey sets the public key for the LegRoast instance.
func (lr *LegRoast) SetPublicKey(pk []byte) error {
	if len(pk) != int(PkBytes) {
		return fmt.Errorf("invalid public key length. Expected %d bytes, got %d bytes", PkBytes, len(pk))
	}
	copy(lr.pk[:], pk)
	return nil
}

// PublicKey returns the public key for the LegRoast instance.
func (lr *LegRoast) PublicKey() []byte {
	return lr.pk[:]
}

// Keygen generates a private/public key pair for LegRoast.
func (lr *LegRoast) Keygen(seed []byte) error {
	if seed == nil {
		// Generate random private key
		_, err := rand.Read(lr.sk[:seedBytes])
		if err != nil {
			return fmt.Errorf("failed to generate private key: %w", err)
		}
	} else {
		// Use the provided seed to generate the private key
		// Ensure the seed is the correct length
		if uint32(len(seed)) != seedBytes {
			return fmt.Errorf("invalid seed length. Expected %d bytes, got %d bytes", seedBytes, len(seed))
		}
		copy(lr.sk[:], seed)
	}

	// Generate modular key
	key := sampleModP(lr.sk[:])

	// Initialize public key to zero
	for i := range lr.pk {
		lr.pk[i] = 0
	}

	// Compute public key
	if lr.isLegendre {
		// Compute Legendre-based public key
		for i := uint32(0); i < PkBytes*8; i++ {
			temp := computeIndex(i)
			addModP(&temp, key)
			bit := legendreSymbolCT(&temp) << (i % 8)
			lr.pk[i/8] |= bit
		}
	} else {
		// Compute Power Residue-based public key
		for i := uint32(0); i < PkBytes; i++ {
			temp := computeIndex(i)
			addModP(&temp, key)
			lr.pk[i] = powerResidueSymbol(&temp)
		}
	}

	return nil
}

// Sign signs the message using the selected LegRoast algorithm.
// Returns a signature or an error if any issue occurs.
func (lr *LegRoast) Sign(msg []byte) ([]byte, error) {
	// Ensure the prover state is initialized
	if lr.proverState == nil {
		return nil, fmt.Errorf("failed to sign. LegRoast prover state is not initialized")
	}

	params := lr.proverState.Params

	// Allocate memory for the signature
	signature := make([]byte, params.SigBytes)
	if signature == nil {
		return nil, fmt.Errorf("failed to allocate memory [%d bytes] for the signature", params.SigBytes)
	}

	hashBuffer := make([]byte, 4*hashBytes)
	// Use a temporary buffer to avoid read-write overlap
	tempHashBuf := make([]byte, 2*hashBytes)
	pHashBuf := hashBuffer[:hashBytes]

	// Phase 1: Hash the message (hash #1)
	LRHash(msg, pHashBuf)

	// Phase 2: Commit
	pSig := signature
	lr.proverState.Clear()
	lr.commit(pSig)

	// Phase 3: Generate challenge1
	pHashBuf = hashBuffer[hashBytes : 2*hashBytes] // hash #2
	LRHash(pSig[:params.Message1Bytes], pHashBuf)
	LRHash(hashBuffer[:2*hashBytes], tempHashBuf)
	copy(pHashBuf, tempHashBuf)

	challenge1 := lr.generateChallenge1(pHashBuf)

	// Phase 4: Respond to challenge1 (msg2)
	pSig = pSig[params.Message1Bytes:] // Move pointer to msg2
	lr.respond1(challenge1, pSig)

	// Phase 5: Generate challenge2
	pHashBuf = hashBuffer[2*hashBytes : 3*hashBytes] // hash #3
	LRHash(pSig[:params.Message2Bytes], pHashBuf)
	LRHash(hashBuffer[hashBytes:3*hashBytes], tempHashBuf)
	copy(pHashBuf, tempHashBuf)

	challenge2 := lr.generateChallenge2(pHashBuf)

	// Phase 6: Respond to challenge2 (msg3)
	lr.respond2(challenge1, challenge2, pSig, pSig[params.Message2Bytes:])

	// Phase 7: Generate challenge3
	pSig = pSig[params.Message2Bytes:]  // Move pointer to msg3
	pHashBuf = hashBuffer[3*hashBytes:] // hash #4
	LRHash(pSig[:params.Message3Bytes], pHashBuf)
	LRHash(hashBuffer[2*hashBytes:], tempHashBuf)
	copy(pHashBuf, tempHashBuf)

	challenge3 := lr.generateChallenge3(pHashBuf)

	// Phase 8: Respond to challenge3 (msg4)
	pSig = pSig[params.Message3Bytes:] // Move pointer to msg4
	lr.respond3(challenge3, pSig)

	return signature, nil
}

// Verify verifies the signature for the given message.
// Before calling this function, the public key must be set.
// Returns an error if the signature is invalid or if any other issue occurs.
func (lr *LegRoast) Verify(msg, signature []byte) error {
	// Ensure the prover state is initialized
	if lr.proverState == nil {
		return fmt.Errorf("failed to verify signature. LegRoast prover state is not initialized")
	}

	params := lr.proverState.Params

	if msg == nil {
		return fmt.Errorf("message is not defined")
	}
	if len(msg) == 0 {
		return fmt.Errorf("message is empty")
	}
	if signature == nil {
		return fmt.Errorf("signature is not defined")
	}
	// Ensure the signature length is correct
	if len(signature) != int(params.SigBytes) {
		return fmt.Errorf("invalid signature length. Expected %d bytes, got %d bytes", params.SigBytes, len(signature))
	}
	// Ensure the public key is set
	pk := [PkBytes]byte{}
	if bytes.Equal(lr.pk[:], pk[:]) {
		return fmt.Errorf("public key is not set")
	}

	hashBuffer := make([]byte, 4*hashBytes)
	// Use a temporary buffer to avoid read-write overlap
	tempHashBuf := make([]byte, 2*hashBytes)
	pHashBuf := hashBuffer[:hashBytes]

	// Phase 1: Hash the message (hash #1)
	LRHash(msg, pHashBuf)

	// Phase 2: Reconstruct challenge1
	pHashBuf = hashBuffer[hashBytes : 2*hashBytes] // hash #2
	sigMsg1 := signature[:params.Message1Bytes]
	LRHash(sigMsg1, pHashBuf)
	LRHash(hashBuffer[:2*hashBytes], tempHashBuf)
	copy(pHashBuf, tempHashBuf)

	challenge1 := lr.generateChallenge1(pHashBuf)

	// Phase 3: Reconstruct challenge2
	pHashBuf = hashBuffer[2*hashBytes : 3*hashBytes] // hash #3
	sigMsg2 := signature[params.Message1Bytes : params.Message1Bytes+params.Message2Bytes]
	LRHash(sigMsg2, pHashBuf)
	LRHash(hashBuffer[hashBytes:3*hashBytes], tempHashBuf)
	copy(pHashBuf, tempHashBuf)

	challenge2 := lr.generateChallenge2(pHashBuf)

	// Phase 4: Reconstruct challenge3
	pHashBuf = hashBuffer[3*hashBytes:] // hash #4
	sigMsg3 := signature[params.Message1Bytes+params.Message2Bytes : params.Message1Bytes+params.Message2Bytes+params.Message3Bytes]
	LRHash(sigMsg3, pHashBuf)
	LRHash(hashBuffer[2*hashBytes:], tempHashBuf)
	copy(pHashBuf, tempHashBuf)

	challenge3 := lr.generateChallenge3(pHashBuf)

	// Phase 5: Check signature validity
	if err := lr.check(
		sigMsg1, challenge1,
		sigMsg2, challenge2,
		sigMsg3, challenge3,
		signature[params.Message1Bytes+params.Message2Bytes+params.Message3Bytes:],
	); err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	return nil
}

// LRExpand performs SHAKE-128 hash with variable output length and returns the result.
func LRExpand(data []byte, output []byte) {
	hasher := sha3.NewShake128()
	_, err := hasher.Write(data)
	if err != nil {
		panic(fmt.Errorf("failed to write data to hasher: %w", err))
	}
	_, err = hasher.Read(output)
	if err != nil {
		panic(fmt.Errorf("failed to read data from hasher: %w", err))
	}
}

// LRHash performs SHAKE-128 hash with a fixed length of hashBytes and returns the result.
func LRHash(data []byte, output []byte) {
	LRExpand(data, output[:hashBytes])
}

// sampleModP samples a value modulo m127 based on the provided seed.
func sampleModP(seed []byte) uint128.Uint128 {
	out := make([]byte, 16)
	LRExpand(seed, out)
	result := uint128.FromBytes(out)
	reduceModP(&result)
	return result
}

func uint32ToBytesLE(a uint32) []byte {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, a)
	return bytes
}

// computeIndex computes a uint128 index from the given uint32 value.
func computeIndex(a uint32) uint128.Uint128 {
	out := make([]byte, 16)
	LRExpand(uint32ToBytesLE(a), out)
	return uint128.FromBytes(out)
}

// generateChallenge1 generates and returns the challenge1 array based on the provided hash.
func (lr *LegRoast) generateChallenge1(hash []byte) []byte {
	params := lr.proverState.Params

	challenge1 := make([]byte, params.Challenge1Bytes)
	LRExpand(hash, challenge1)

	indices := make([]uint32, params.RESSYMPerRound)
	for i := range indices {
		indices[i] = binary.LittleEndian.Uint32(challenge1[i*4 : (i+1)*4])
		if lr.isLegendre {
			indices[i] &= ((uint32(1) << pkDepth) - 1)
		} else {
			indices[i] &= ((uint32(1) << (pkDepth - 3)) - 1)
		}
		binary.LittleEndian.PutUint32(challenge1[i*4:(i+1)*4], indices[i])
	}

	return challenge1
}

// computeIndices computes the indices based on the challenge and stores them in the provided indices slice.
func (lr *LegRoast) computeIndices(challenge1 []byte) {
	params := lr.proverState.Params
	indices := lr.proverState.Indices
	index := uint32(0)
	for i := uint32(0); i < params.RESSYMPerRound; i++ {
		indices[i] = computeIndex(binary.LittleEndian.Uint32(challenge1[index : index+4]))
		index += 4
	}
}

// respond1 computes the response message based on challenge1 and writes it to the preallocated part of the signature buffer pointed by message2.
func (lr *LegRoast) respond1(challenge1 []byte, message2 []byte) {
	params := lr.proverState.Params
	lr.computeIndices(challenge1)

	// generate key
	key := sampleModP(lr.sk[:])
	output := make([]uint128.Uint128, params.Message2Bytes/Uint128Size)

	// Compute output
	for nRound := uint32(0); nRound < params.NRounds; nRound++ {
		pSums := lr.proverState.Sums[nRound]
		for i := uint32(0); i < params.NResiduositySymbolsPerRound; i++ {
			index := nRound*params.NResiduositySymbolsPerRound + i
			keyPlusI := lr.proverState.Indices[index]
			addModP(&keyPlusI, key)
			mulAddModP(&output[index], &keyPlusI, &pSums[sharesR+i])
			reduceModP(&output[index])
		}
	}

	// Copy output to message2
	index := uint32(0)
	for i := range output {
		output[i].PutBytes(message2[index : index+Uint128Size])
		index += Uint128Size
	}
}

func (lr *LegRoast) generateChallenge2(hash []byte) []byte {
	params := lr.proverState.Params

	challenge2 := make([]byte, params.Challenge2Bytes)
	LRExpand(hash, challenge2)
	return challenge2
}

// respond2 computes the response message based on challenge1 and challenge2, writing it to message3.
func (lr *LegRoast) respond2(challenge1 []byte, challenge2 []byte, message2 []byte, message3 []byte) {
	params := lr.proverState.Params

	// generate key
	key := sampleModP(lr.sk[:])

	openings := NewUint128Matrix(params.NRounds, params.Parties, 3)
	indices := make([]uint32, len(challenge1)/4)
	for i := range indices {
		indices[i] = binary.LittleEndian.Uint32(challenge1[i*4 : (i+1)*4])
	}

	for nRound := uint32(0); nRound < params.NRounds; nRound++ {
		pSums := lr.proverState.Sums[nRound]
		shR := lr.proverState.Shares.GetPlainSlice(nRound)
		openingsR := openings.GetPlainSlice(nRound)
		rIndex := nRound * Uint128Size
		epsilon := uint128.FromBytes(challenge2[rIndex : rIndex+Uint128Size])

		// compute alpha in the clear
		alpha := uint128.Zero
		mulAddModP(&alpha, &epsilon, &key)
		addModP(&alpha, pSums[sharesTriple])
		reduceModP(&alpha)
		alpha.PutBytes(message3[message3Alpha+rIndex : message3Alpha+rIndex+Uint128Size])

		// import lambda and compute beta in the clear
		lambdaIndex := params.Challenge2Lambda + nRound*params.NResiduositySymbolsPerRound*Uint128Size
		lambda := challenge2[lambdaIndex:]

		beta := pSums[sharesTriple+1]
		index := uint32(0)
		for i := uint32(0); i < params.NResiduositySymbolsPerRound; i++ {
			lambdaValue := uint128.FromBytes(lambda[index : index+Uint128Size])
			mulAddModP(&beta, &lambdaValue, &pSums[sharesR+i])
			index += Uint128Size
		}
		reduceModP(&beta)
		beta.PutBytes(message3[params.Message3Beta+rIndex : params.Message3Beta+rIndex+Uint128Size])

		// computes shares of alpha, beta and v
		for nParty := uint32(0); nParty < params.Parties; nParty++ {
			sharesIndex := nParty * params.SharesPerParty
			openingsIndex := nParty * 3
			// compute share of alpha
			p0 := &openingsR[openingsIndex]
			mulAddModP(p0, &epsilon, &shR[sharesIndex+shareK])
			addModP(p0, shR[sharesIndex+sharesTriple])
			reduceModP(p0)

			// compute share of beta and z
			zShare := uint128.Zero
			p1 := &openingsR[openingsIndex+1]
			*p1 = shR[sharesIndex+sharesTriple+1]
			for j := uint32(0); j < params.NResiduositySymbolsPerRound; j++ {
				lambdaValue := uint128.FromBytes(lambda[j*16 : (j+1)*16])

				// share of beta
				mulAddModP(p1, &shR[sharesIndex+sharesR+j], &lambdaValue)
				reduceModP(p1)

				// share of z
				temp2 := uint128.Zero
				mulAddModP(&temp2, &shR[sharesIndex+sharesR+j], &lambdaValue)
				reduceModP(&temp2)
				temp2 = m127.Sub(temp2)
				index := lr.proverState.Indices[nRound*params.NResiduositySymbolsPerRound+j]
				mulAddModP(&zShare, &temp2, &index)

				if nParty == 0 {
					message2Index := uint128.FromBytes(message2[nRound*params.NResiduositySymbolsPerRound*16+j*16 : nRound*params.NResiduositySymbolsPerRound*16+(j+1)*16])
					mulAddModP(&zShare, &lambdaValue, &message2Index)
				}
			}

			// compute sharing of v
			p2 := &openingsR[openingsIndex+2]
			*p2 = shR[sharesIndex+sharesTriple+2]
			if nParty == 0 {
				mulAddModP(p2, &alpha, &beta)
			}
			reduceModP(p2)
			*p2 = m127.Sub(openingsR[openingsIndex+2])
			mulAddModP(p2, &alpha, &shR[sharesIndex+sharesTriple+1])
			mulAddModP(p2, &beta, &shR[sharesIndex+sharesTriple])
			mulAddModP(p2, &epsilon, &zShare)
			reduceModP(p2)
		}
	}

	LRHash(openings.GetAsBytes(), message3)
}

// generateChallenge3 generates and returns the challenge3 array based on the provided hash.
func (lr *LegRoast) generateChallenge3(hash []byte) []byte {
	params := lr.proverState.Params

	challenge3 := make([]byte, params.Challenge3Bytes)
	LRExpand(hash, challenge3)

	index := uint32(0)
	for i := uint32(0); i < params.NRounds; i++ {
		// Read and mask the value directly
		unopenedParty := binary.LittleEndian.Uint32(challenge3[index : index+4])
		unopenedParty &= params.Parties - 1
		binary.LittleEndian.PutUint32(challenge3[index:index+4], unopenedParty)

		index += 4
	}

	return challenge3
}

// respond3 computes the response message based on challenge3, writing it to message4.
func (lr *LegRoast) respond3(challenge3 []byte, message4 []byte) {
	params := lr.proverState.Params

	unopenedParty := make([]uint32, len(challenge3)/4)
	index := uint32(0)
	for i := range unopenedParty {
		unopenedParty[i] = binary.LittleEndian.Uint32(challenge3[index : index+4])
		index += 4
	}

	st := lr.proverState.SeedTrees
	for nRound := uint32(0); nRound < params.NRounds; nRound++ {
		unopenedIndex := unopenedParty[nRound]
		st.ReleaseSeeds(nRound, unopenedIndex, message4[nRound*params.NPartyDepth*seedBytes:])
		stR := st.GetRoundSlice(nRound)
		index := (params.Parties - 1 + unopenedIndex) * seedBytes
		LRHash(stR[index:index+seedBytes], message4[params.Message4Commitment+nRound*hashBytes:])
	}
}

func (lr *LegRoast) check(
	message1, challenge1, message2, challenge2, message3, challenge3, message4 []byte,
) error {
	params := lr.proverState.Params

	lr.proverState.Clear()
	lr.computeIndices(challenge1)

	unopenedParty := make([]uint32, len(challenge3)/4)
	index := uint32(0)
	for i := range unopenedParty {
		unopenedParty[i] = binary.LittleEndian.Uint32(challenge3[index : index+4])
		index += 4
	}

	// check first commitment: seeds + jacobi symbols
	bufSize := params.NRounds*params.Parties*hashBytes + params.RESSYMPerRound
	pBuf := make([]byte, bufSize)
	expandedShares := make([]byte, params.SharesPerParty*Uint128Size)

	st := lr.proverState.SeedTrees
	shares := lr.proverState.Shares
	for nRound := uint32(0); nRound < params.NRounds; nRound++ {
		stR := st.GetRoundSlice(nRound)
		shR := shares.GetPlainSlice(nRound)

		unopenedIndex := unopenedParty[nRound]
		// Fill the seed tree and verify commitments
		st.FillDown(nRound, unopenedIndex, message4[nRound*params.NPartyDepth*seedBytes:])

		msgIndex := params.Message4Commitment + nRound*hashBytes
		// Copy the commitment of the unopened value
		bufBaseIndex := nRound * params.Parties * hashBytes
		copy(pBuf[bufBaseIndex+unopenedIndex*hashBytes:], message4[msgIndex:msgIndex+hashBytes])

		// Generate and verify the remaining commitments
		for nParty := uint32(0); nParty < params.Parties; nParty++ {
			if nParty == unopenedIndex {
				continue
			}

			// Commit to seed
			seedIndex := (params.Parties - 1 + nParty) * seedBytes
			LRHash(stR[seedIndex:seedIndex+seedBytes], pBuf[bufBaseIndex+nParty*hashBytes:])

			// Generate shares from seed
			LRExpand(stR[seedIndex:seedIndex+seedBytes], expandedShares)

			sharesPartyIndex := nParty * params.SharesPerParty
			shareIndex := sharesPartyIndex
			for nShare := uint32(0); nShare < params.SharesPerParty; nShare++ {
				shR[shareIndex] = uint128.FromBytes(expandedShares[nShare*Uint128Size : (nShare+1)*Uint128Size])
				shareIndex++
			}

			// Modify deltas for the first share if needed
			if nParty == 0 {
				// Delta K modification
				msgIndex := message1DeltaK + nRound*Uint128Size
				deltaK := uint128.FromBytes(message1[msgIndex : msgIndex+Uint128Size])
				addModP(&shR[sharesPartyIndex+shareK], deltaK)

				// Delta Triple modification
				msgIndex = params.Message1DeltaTriple + nRound*Uint128Size
				deltaTriple := uint128.FromBytes(message1[msgIndex : msgIndex+Uint128Size])
				addModP(&shR[sharesPartyIndex+sharesTriple+2], deltaTriple)
			}
		}

		// Compute Legendre or Power Residue Symbols
		bufBaseIndex = params.NRounds*params.Parties*hashBytes + nRound*params.NResiduositySymbolsPerRound
		msgBaseIndex := nRound * params.NResiduositySymbolsPerRound
		for i := uint32(0); i < params.NResiduositySymbolsPerRound; i++ {
			index := msgBaseIndex + i
			uint128Index := index * Uint128Size
			uint32Index := index * 4

			// Extract the uint128 value from message2
			ui := uint128.FromBytes(message2[uint128Index : uint128Index+Uint128Size])
			// Extract the challenge index from challenge1
			challengeIndex := binary.LittleEndian.Uint32(challenge1[uint32Index : uint32Index+4])

			bufOffset := bufBaseIndex + i

			if lr.isLegendre {
				// Compute Legendre symbol and XOR with the challenge bit
				pBuf[bufOffset] = legendreSymbolCT(&ui) ^ lr.getBit(challengeIndex)
			} else {
				// Compute Power Residue Symbol (PRS)
				prs := uint16(powerResidueSymbol(&ui))
				prs += uint16(254) - uint16(lr.pk[challengeIndex])

				// Store PRS in the buffer
				pBuf[bufOffset] = byte(prs % 254)
			}
		}
	}

	// Verify first commitment
	hash1 := make([]byte, hashBytes)
	LRHash(pBuf, hash1)
	if !bytes.Equal(hash1, message1[:hashBytes]) {
		return fmt.Errorf("first commitment verification failed")
	}

	// Check the second commitment: alpha, beta, and v
	openings := NewUint128Matrix(params.NRounds, params.Parties, 3)
	for nRound := uint32(0); nRound < params.NRounds; nRound++ {
		shR := shares.GetPlainSlice(nRound)
		openingsR := openings.GetPlainSlice(nRound)
		unopenedPartyIndex := unopenedParty[nRound]

		// Import epsilon, alpha, beta, and lambda
		rIndex := nRound * Uint128Size
		epsilon := uint128.FromBytes(challenge2[rIndex : rIndex+Uint128Size])
		alpha := uint128.FromBytes(message3[message3Alpha+rIndex : message3Alpha+rIndex+Uint128Size])
		beta := uint128.FromBytes(message3[params.Message3Beta+rIndex : params.Message3Beta+rIndex+Uint128Size])

		var sumVShares, sumAlphaShares, sumBetaShares uint128.Uint128
		nSymPerRound := nRound * params.NResiduositySymbolsPerRound
		lambdaStart := params.Challenge2Lambda + nSymPerRound*Uint128Size

		// Compute shares of alpha, beta, and v
		for nParty := uint32(0); nParty < params.Parties; nParty++ {
			if nParty == unopenedPartyIndex {
				continue
			}

			sharesIndex := nParty * params.SharesPerParty
			openingsIndex := nParty * 3

			// Compute share of alpha
			pAlpha := &openingsR[openingsIndex]
			mulAddModP(pAlpha, &epsilon, &shR[sharesIndex+shareK])
			addModP(pAlpha, shR[sharesIndex+sharesTriple])
			reduceModP(pAlpha)
			addModP(&sumAlphaShares, *pAlpha)

			// Compute share of beta and z
			var zShare uint128.Uint128
			pBeta := &openingsR[openingsIndex+1]
			*pBeta = shR[sharesIndex+sharesTriple+1]
			for j := uint32(0); j < params.NResiduositySymbolsPerRound; j++ {
				lambdaIndex := j * Uint128Size
				lambdaValue := uint128.FromBytes(challenge2[lambdaStart+lambdaIndex : lambdaStart+lambdaIndex+Uint128Size])

				// Share of beta
				mulAddModP(pBeta, &shR[sharesIndex+sharesR+j], &lambdaValue)

				// Share of z
				var temp2 uint128.Uint128
				mulAddModP(&temp2, &shR[sharesIndex+sharesR+j], &lambdaValue)
				reduceModP(&temp2)
				temp2 = m127.Sub(temp2)
				index := lr.proverState.Indices[nSymPerRound+j]
				mulAddModP(&zShare, &temp2, &index)

				if nParty == 0 {
					msg2Index := nSymPerRound*Uint128Size + j*Uint128Size
					msg2Value := uint128.FromBytes(message2[msg2Index : msg2Index+Uint128Size])
					mulAddModP(&zShare, &lambdaValue, &msg2Value)
				}
			}

			reduceModP(pBeta)
			addModP(&sumBetaShares, *pBeta)

			// Compute sharing of v
			pV := &openingsR[openingsIndex+2]
			*pV = shR[sharesIndex+sharesTriple+2]
			if nParty == 0 {
				mulAddModP(pV, &alpha, &beta)
			}
			reduceModP(pV)
			*pV = m127.Sub(*pV)
			mulAddModP(pV, &alpha, &shR[sharesIndex+sharesTriple+1])
			mulAddModP(pV, &beta, &shR[sharesIndex+sharesTriple])
			mulAddModP(pV, &epsilon, &zShare)
			reduceModP(pV)
			addModP(&sumVShares, *pV)
		}

		// Fill in unopened shares
		reduceModP(&sumAlphaShares)
		reduceModP(&sumBetaShares)
		reduceModP(&sumVShares)
		pAlphaUnopened := &openingsR[unopenedPartyIndex*3]
		pBetaUnopened := &openingsR[unopenedPartyIndex*3+1]
		pVUnopened := &openingsR[unopenedPartyIndex*3+2]

		*pAlphaUnopened = m127.Sub(sumAlphaShares)
		*pBetaUnopened = m127.Sub(sumBetaShares)
		*pVUnopened = m127.Sub(sumVShares)

		addModP(pAlphaUnopened, alpha)
		addModP(pBetaUnopened, beta)
		reduceModP(pAlphaUnopened)
		reduceModP(pBetaUnopened)
		reduceModP(pVUnopened)
	}

	// Compute hash2 and compare with message3
	hash2 := make([]byte, hashBytes)
	LRHash(openings.GetAsBytes(), hash2)
	if !bytes.Equal(hash2, message3[:hashBytes]) {
		return fmt.Errorf("second hash failed")
	}

	return nil
}

// commit computes the commitments and writes them to message1.
func (lr *LegRoast) commit(message1 []byte) {
	params := lr.proverState.Params

	copy(message1, make([]byte, params.Message1Bytes)) // Zero-initialize message1

	// generate key
	key := sampleModP(lr.sk[:])
	commitments := make([]byte, params.NRounds*params.Parties*hashBytes+params.RESSYMPerRound)

	st := lr.proverState.SeedTrees
	shares := lr.proverState.Shares
	expanded := make([]byte, params.SharesPerParty*Uint128Size)
	for nRound := uint32(0); nRound < params.NRounds; nRound++ {
		pSums := lr.proverState.Sums[nRound]

		st.Generate(nRound)
		stR := st.GetRoundSlice(nRound)
		shR := shares.GetPlainSlice(nRound)

		index := (params.Parties - 1) * seedBytes
		cmtIndex := nRound * params.Parties * hashBytes
		shIndex := uint32(0)
		// generate the commitments and the shares
		for nParty := uint32(0); nParty < params.Parties; nParty++ {
			// commit to seed
			src := stR[index : index+seedBytes]
			LRHash(src, commitments[cmtIndex:cmtIndex+hashBytes])

			// generate shares from seed
			LRExpand(src, expanded)
			for j := uint32(0); j < params.SharesPerParty; j++ {
				shR[shIndex+j] = uint128.FromBytes(expanded[j*Uint128Size : (j+1)*Uint128Size])
				// add the shares to the sums
				addModP(&pSums[j], shR[shIndex+j])
			}

			index += seedBytes
			cmtIndex += hashBytes
			shIndex += params.SharesPerParty
		}

		// reduce sums mod p
		for i := uint32(0); i < params.SharesPerParty; i++ {
			reduceModP(&pSums[i])
		}

		// compute legendre symbols of R_i
		nIndexBase := params.NRounds*params.Parties*hashBytes + nRound*params.NResiduositySymbolsPerRound
		for i := uint32(0); i < params.NResiduositySymbolsPerRound; i++ {
			if lr.isLegendre {
				commitments[nIndexBase+i] = legendreSymbolCT(&pSums[sharesR+i])
			} else {
				commitments[nIndexBase+i] = powerResidueSymbol(&pSums[sharesR+i])
			}
		}

		// compute Delta K and add to share 0
		deltaK := m127.Sub(pSums[shareK])
		addModP(&deltaK, key)
		reduceModP(&deltaK)
		addModP(&shR[shareK], deltaK)
		deltaKIndex := message1DeltaK + nRound*Uint128Size
		deltaK.PutBytes(message1[deltaKIndex : deltaKIndex+Uint128Size])

		// compute Delta Triple and add to share 0
		deltaTriple := m127.Sub(pSums[sharesTriple+2])
		mulAddModP(&deltaTriple, &pSums[sharesTriple], &pSums[sharesTriple+1])
		reduceModP(&deltaTriple)
		addModP(&shR[sharesTriple+2], deltaTriple)
		deltaTripleIndex := params.Message1DeltaTriple + nRound*Uint128Size
		deltaTriple.PutBytes(message1[deltaTripleIndex : deltaTripleIndex+Uint128Size])
	}

	LRHash(commitments, message1)
}
