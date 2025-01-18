package legroast

import (
	"testing"

	"github.com/stretchr/testify/suite"
)


type LegRoastTestSuite struct {
	suite.Suite
	tests []struct {
		name      string
		algorithm LegRoastAlgorithm
		message   string
	}
}

func (suite *LegRoastTestSuite) SetupSuite() {
	suite.tests = []struct {
		name      string
		algorithm LegRoastAlgorithm
		message   string
	}{
		{name: "Sign and Verify - Legendre Fast", algorithm: LegendreFast, message: "test message for Legendre Fast"},
		{name: "Sign and Verify - Legendre Middle", algorithm: LegendreMiddle, message: "test message for Legendre Middle"},
		{name: "Sign and Verify - Legendre Compact", algorithm: LegendreCompact, message: "test message for Legendre Compact"},
		{name: "Sign and Verify - Power Fast", algorithm: PowerFast, message: "test message for Power Fast"},
		{name: "Sign and Verify - Power Middle", algorithm: PowerMiddle, message: "test message for Power Middle"},
		{name: "Sign and Verify - Power Compact", algorithm: PowerCompact, message: "test message for Power Compact"},
	}
}

func (suite *LegRoastTestSuite) runSignAndVerify(tt struct {
	name      string
	algorithm LegRoastAlgorithm
	message   string
}, seed []byte) {
	// Initialize LegRoast instance based on algorithm
	lr := NewLegRoast(tt.algorithm)

	// Key generation
	suite.NoError(lr.Keygen(seed), "Keygen should succeed")

	// Sign the message
	signature, err := lr.Sign([]byte(tt.message))
	suite.NoError(err, "Signing should succeed")
	suite.NotNil(signature, "Signature should not be nil")
	// check signature length
	suite.Equal(lr.Params().SigBytes, uint32(len(signature)), "Signature length should match")

	// Verify the signature
	err = lr.Verify([]byte(tt.message), signature)
	suite.NoError(err, "Verification should not return an error")

	// Verify the signature size
	alg, err := GetAlgorithmBySigSize(len(signature))
	suite.NoError(err, "GetAlgorithmBySigSize should not return an error")
	suite.Equal(tt.algorithm, alg, "Algorithm should match")
}

func (suite *LegRoastTestSuite) TestFixedSeed() {
	arr := [16]byte{42, 27, 251, 236, 198, 244, 224, 12, 145, 63, 239, 83, 159, 251, 242, 158}
	seed := arr[:]
	for _, tt := range suite.tests {
		suite.Run(tt.name + " - Fixed Seed", func() {
			suite.T().Parallel()
			suite.runSignAndVerify(tt, seed)
		})
	}
}

func (suite *LegRoastTestSuite) TestRandomSeed() {
	for _, tt := range suite.tests {
		suite.Run(tt.name + " - Random Seed", func() {
			suite.runSignAndVerify(tt, nil)
		})
	}
}

func (suite *LegRoastTestSuite) TestSetInvalidPublicKey() {
	lr := NewLegRoast(LegendreFast)
	err := lr.SetPublicKey([]byte("invalid public key"))
	suite.Error(err, "SetPublicKey should return an error")
}

func (suite *LegRoastTestSuite) TestSetInvalidSeed() {
	lr := NewLegRoast(LegendreFast)
	err := lr.Keygen([]byte("invalid seed"))
	suite.Error(err, "Keygen should return an error")
}

func (suite *LegRoastTestSuite) TestInvalidProver() {
	lr := NewLegRoast(LegendreFast)
	lr.(*LegRoast).proverState = nil

	signature, err := lr.Sign([]byte("test message"))
	suite.Nil(signature, "Signature should be nil")
	suite.Error(err, "Signing should return an error")

	err = lr.Verify([]byte("test message"), []byte("signature"))
	suite.Error(err, "Verification should return an error")	
}

func (suite *LegRoastTestSuite) TestVerify() {
	lr := NewLegRoast(LegendreFast)
	var err error
	err = lr.Verify(nil, []byte("signature"))
	suite.Error(err, "Verification should return an error")
	err = lr.Verify([]byte(""), nil)
	suite.Error(err, "Verification should return an error")

	err = lr.Verify([]byte("test message"), nil)
	suite.Error(err, "Verification should return an error")
	err = lr.Verify([]byte("test message"), []byte(""))
	suite.Error(err, "Verification should return an error")

	// invalid signature size
	err = lr.Verify([]byte("test message"), []byte("signature"))
	suite.Error(err, "Verification should return an error")

	// invalid public key
	signature, err := lr.Sign([]byte("test message"))
	suite.NoError(err, "Signing should succeed")
	err = lr.Verify([]byte("test message"), signature)
	suite.Error(err, "Verification should return an error")	
}

func (suite *LegRoastTestSuite) TestInvalidSignature() {
	lr := NewLegRoast(LegendreFast)

	err := lr.Keygen(nil)
	suite.NoError(err, "Keygen should succeed")

	signature, err := lr.Sign([]byte("test message"))
	suite.NoError(err, "Signing should succeed")

	signature[0] = signature[0] + 1
	err = lr.Verify([]byte("test message"), signature)
	suite.Error(err, "Verification should return an error")
}

func TestLegRoastTestSuite(t *testing.T) {
	suite.Run(t, new(LegRoastTestSuite))
}

