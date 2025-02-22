package legroast

import "fmt"

// LegRoastAlgorithm defines the supported LegRoast algorithms.
type LegRoastAlgorithm uint32

const (
	LegendreFast LegRoastAlgorithm = iota
	LegendreMiddle
	LegendreCompact
	PowerFast
	PowerMiddle
	PowerCompact

	AlgorithmCount
)

const (
	primeBytes uint32 = 16
	seedBytes  uint32 = 16
	hashBytes  uint32 = 32
	pkDepth    uint32 = 15

	PkBytes = 1 << (pkDepth - 3)
	SkBytes = seedBytes

	//the order of the shares in memory
	shareK       = 0
	sharesTriple = shareK + 1
	sharesR      = sharesTriple + 3

	message1DeltaK = hashBytes
	message3Alpha  = hashBytes

	DefaultAlgorithm = LegendreMiddle
)

// String returns the string representation of the LegRoastAlgorithm.
func (a LegRoastAlgorithm) String() string {
	switch a {
	case LegendreFast:
		return "LegendreFast"
	case LegendreMiddle:
		return "LegendreMiddle"
	case LegendreCompact:
		return "LegendreCompact"
	case PowerFast:
		return "PowerFast"
	case PowerMiddle:
		return "PowerMiddle"
	case PowerCompact:
		return "PowerCompact"
	default:
		return "Unknown"
	}
}

// LegRoastParams defines the parameters for the LegRoast algorithm.
type LegRoastParams struct {
	Alg                         LegRoastAlgorithm
	NRounds                     uint32
	NResiduositySymbolsPerRound uint32
	NPartyDepth                 uint32

	RESSYMPerRound      uint32
	Parties             uint32
	SharesPerParty      uint32
	Message1DeltaTriple uint32
	Message1Bytes       uint32
	Challenge1Bytes     uint32
	Message2Bytes       uint32
	Challenge2Lambda    uint32
	Challenge2Bytes     uint32
	Message3Beta        uint32
	Message3Bytes       uint32
	Challenge3Bytes     uint32
	Message4Commitment  uint32
	Message4Bytes       uint32
	SigBytes            uint32
}

// newLegRoastParams initializes LegRoastParams with derived values.
func newLegRoastParams(alg LegRoastAlgorithm, nRounds, nResiduositySymbolsPerRound, nPartyDepth uint32) LegRoastParams {
	ressymPerRound := nRounds * nResiduositySymbolsPerRound
	parties := uint32(1) << nPartyDepth
	sharesPerParty := sharesR + nResiduositySymbolsPerRound

	message1DeltaTriple := message1DeltaK + nRounds*uint32(16) // assuming uint128_t is 16 bytes
	message1Bytes := message1DeltaTriple + nRounds*primeBytes
	challenge1Bytes := ressymPerRound * uint32(4) // assuming sizeof(uint32_t) is 4 bytes

	message2Bytes := ressymPerRound * primeBytes
	challenge2Lambda := nRounds * primeBytes
	challenge2Bytes := challenge2Lambda + ressymPerRound*primeBytes

	message3Beta := message3Alpha + nRounds*primeBytes
	message3Bytes := message3Beta + nRounds*primeBytes
	challenge3Bytes := nRounds * uint32(4) // assuming sizeof(uint32_t) is 4 bytes

	message4Commitment := nRounds * nPartyDepth * seedBytes
	message4Bytes := message4Commitment + nRounds*hashBytes
	sigBytes := message1Bytes + message2Bytes + message3Bytes + message4Bytes

	return LegRoastParams{
		Alg:                         alg,
		NRounds:                     nRounds,
		NResiduositySymbolsPerRound: nResiduositySymbolsPerRound,
		NPartyDepth:                 nPartyDepth,
		RESSYMPerRound:              ressymPerRound,
		Parties:                     parties,
		SharesPerParty:              sharesPerParty,
		Message1DeltaTriple:         message1DeltaTriple,
		Message1Bytes:               message1Bytes,
		Challenge1Bytes:             challenge1Bytes,
		Message2Bytes:               message2Bytes,
		Challenge2Lambda:            challenge2Lambda,
		Challenge2Bytes:             challenge2Bytes,
		Message3Beta:                message3Beta,
		Message3Bytes:               message3Bytes,
		Challenge3Bytes:             challenge3Bytes,
		Message4Commitment:          message4Commitment,
		Message4Bytes:               message4Bytes,
		SigBytes:                    sigBytes,
	}
}

// Predefined LegRoast parameters for each algorithm.
var legRoastParamsList = []LegRoastParams{
	newLegRoastParams(LegendreFast, 54, 9, 4),
	newLegRoastParams(LegendreMiddle, 37, 12, 6),
	newLegRoastParams(LegendreCompact, 26, 16, 8),
	newLegRoastParams(PowerFast, 39, 4, 4),
	newLegRoastParams(PowerMiddle, 27, 5, 6),
	newLegRoastParams(PowerCompact, 21, 5, 8),
}

// GetLegRoastParams returns the parameters for the specified algorithm.
func GetLegRoastParams(alg LegRoastAlgorithm) *LegRoastParams {
	if int(alg) < len(legRoastParamsList) {
		return &legRoastParamsList[alg]
	}
	return nil
}

// GetAlgorithmBySigSize returns the algorithm based on the signature size.
var GetAlgorithmBySigSize = func(sigSize int) (LegRoastAlgorithm, error) {
	for i, params := range legRoastParamsList {
		if params.SigBytes == uint32(sigSize) {
			return LegRoastAlgorithm(i), nil
		}
	}
	return AlgorithmCount, fmt.Errorf("incorrect signature size %d for LegRoast algorithm", sigSize)
}

// GetLegRoastAlgorithm returns the algorithm based on the string representation.
func GetLegRoastAlgorithm(alg string) (LegRoastAlgorithm, error) {
	if alg == "" {
		return DefaultAlgorithm, nil
	}
	for i, params := range legRoastParamsList {
		if params.Alg.String() == alg {
			return LegRoastAlgorithm(i), nil
		}
	}
	return AlgorithmCount, fmt.Errorf("unknown LegRoast algorithm %s", alg)
}
