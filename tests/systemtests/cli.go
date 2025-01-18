package system

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"golang.org/x/exp/slices"

	"github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type (
	// blocks until next block is minted
	awaitNextBlock func(t *testing.T, timeout ...time.Duration) int64
	// RunErrorAssert is custom type that is satisfies by testify matchers as well
	RunErrorAssert func(t assert.TestingT, err error, msgAndArgs ...interface{}) (ok bool)
)

// PasteldCli wraps the command line interface
type PasteldCli struct {
	t              *testing.T
	nodeAddress    string
	chainID        string
	homeDir        string
	fees           string
	Debug          bool
	assertErrorFn  RunErrorAssert
	awaitNextBlock awaitNextBlock
	expTXCommitted bool
	execBinary     string
	nodesCount     int
}

// NewPasteldCLI constructor
func NewPasteldCLI(t *testing.T, sut *SystemUnderTest, verbose bool) *PasteldCli {
	return NewPasteldCLIx(
		t,
		sut.ExecBinary,
		sut.rpcAddr,
		sut.chainID,
		sut.AwaitNextBlock,
		sut.nodesCount,
		filepath.Join(WorkDir, sut.outputDir),
		"1"+sdk.DefaultBondDenom,
		verbose,
		assert.NoError,
		true,
	)
}

// NewPasteldCLIx extended constructor
func NewPasteldCLIx(
	t *testing.T,
	execBinary string,
	nodeAddress string,
	chainID string,
	awaiter awaitNextBlock,
	nodesCount int,
	homeDir string,
	fees string,
	debug bool,
	assertErrorFn RunErrorAssert,
	expTXCommitted bool,
) *PasteldCli {
	if strings.TrimSpace(execBinary) == "" {
		panic("executable binary name must not be empty")
	}
	return &PasteldCli{
		t:              t,
		execBinary:     execBinary,
		nodeAddress:    nodeAddress,
		chainID:        chainID,
		homeDir:        homeDir,
		Debug:          debug,
		awaitNextBlock: awaiter,
		nodesCount:     nodesCount,
		fees:           fees,
		assertErrorFn:  assertErrorFn,
		expTXCommitted: expTXCommitted,
	}
}

// WithRunErrorsIgnored does not fail on any error
func (c PasteldCli) WithRunErrorsIgnored() PasteldCli {
	return c.WithRunErrorMatcher(func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool {
		return true
	})
}

// WithRunErrorMatcher assert function to ensure run command error value
func (c PasteldCli) WithRunErrorMatcher(f RunErrorAssert) PasteldCli {
	return *NewPasteldCLIx(
		c.t,
		c.execBinary,
		c.nodeAddress,
		c.chainID,
		c.awaitNextBlock,
		c.nodesCount,
		c.homeDir,
		c.fees,
		c.Debug,
		f,
		c.expTXCommitted,
	)
}

func (c PasteldCli) WithNodeAddress(nodeAddr string) PasteldCli {
	return *NewPasteldCLIx(
		c.t,
		c.execBinary,
		nodeAddr,
		c.chainID,
		c.awaitNextBlock,
		c.nodesCount,
		c.homeDir,
		c.fees,
		c.Debug,
		c.assertErrorFn,
		c.expTXCommitted,
	)
}

func (c PasteldCli) WithAssertTXUncommitted() PasteldCli {
	return *NewPasteldCLIx(
		c.t,
		c.execBinary,
		c.nodeAddress,
		c.chainID,
		c.awaitNextBlock,
		c.nodesCount,
		c.homeDir,
		c.fees,
		c.Debug,
		c.assertErrorFn,
		false,
	)
}

// CustomCommand main entry for executing pasteld cli commands.
// When configured, method blocks until tx is committed.
func (c PasteldCli) CustomCommand(args ...string) string {
	if c.fees != "" && !slices.ContainsFunc(args, func(s string) bool {
		return strings.HasPrefix(s, "--fees")
	}) {
		args = append(args, "--fees="+c.fees) // add default fee
	}
	args = c.withTXFlags(args...)
	execOutput, ok := c.run(args)
	if !ok {
		return execOutput
	}
	rsp, committed := c.awaitTxCommitted(execOutput, DefaultWaitTime)
	c.t.Logf("tx committed: %v", committed)
	require.Equal(c.t, c.expTXCommitted, committed, "expected tx committed: %v", c.expTXCommitted)
	return rsp
}

// wait for tx committed on chain
func (c PasteldCli) awaitTxCommitted(submitResp string, timeout ...time.Duration) (string, bool) {
	RequireTxSuccess(c.t, submitResp)
	txHash := gjson.Get(submitResp, "txhash")
	require.True(c.t, txHash.Exists())
	var txResult string
	for i := 0; i < 3; i++ { // max blocks to wait for a commit
		txResult = c.WithRunErrorsIgnored().CustomQuery("q", "tx", txHash.String())
		if code := gjson.Get(txResult, "code"); code.Exists() {
			if code.Int() != 0 { // 0 = success code
				c.t.Logf("+++ got error response code: %s\n", txResult)
			}
			return txResult, true
		}
		c.awaitNextBlock(c.t, timeout...)
	}
	return "", false
}

// keys CLI command
func (c PasteldCli) Keys(args ...string) string {
	args = c.withKeyringFlags(args...)
	out, _ := c.run(args)
	return out
}

// CustomQuery main entrypoint for pasteld CLI queries
func (c PasteldCli) CustomQuery(args ...string) string {
	args = c.withQueryFlags(args...)
	out, _ := c.run(args)
	return out
}

// execute shell command
func (c PasteldCli) run(args []string) (output string, ok bool) {
	return c.runWithInput(args, nil)
}

func (c PasteldCli) runWithInput(args []string, input io.Reader) (output string, ok bool) {
	if c.Debug {
		c.t.Logf("+++ running `%s %s`", c.execBinary, strings.Join(args, " "))
	}
	gotOut, gotErr := func() (out []byte, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("recovered from panic: %v", r)
			}
		}()
		cmd := exec.Command(locateExecutable(c.execBinary), args...) //nolint:gosec
		cmd.Dir = WorkDir
		cmd.Stdin = input
		return cmd.CombinedOutput()
	}()
	ok = c.assertErrorFn(c.t, gotErr, string(gotOut))
	return strings.TrimSpace(string(gotOut)), ok
}

func (c PasteldCli) withQueryFlags(args ...string) []string {
	args = append(args, "--output", "json")
	return c.withChainFlags(args...)
}

func (c PasteldCli) withTXFlags(args ...string) []string {
	args = append(args,
		"--broadcast-mode", "sync",
		"--output", "json",
		"--yes",
		"--chain-id", c.chainID,
	)
	args = c.withKeyringFlags(args...)
	return c.withChainFlags(args...)
}

func (c PasteldCli) withKeyringFlags(args ...string) []string {
	r := append(args,
		"--home", c.homeDir,
		"--keyring-backend", "test",
	)
	for _, v := range args {
		if v == "-a" || v == "--address" { // show address only
			return r
		}
	}
	return append(r, "--output", "json")
}

func (c PasteldCli) withChainFlags(args ...string) []string {
	return append(args,
		"--node", c.nodeAddress,
	)
}

// AddKey add key to default keyring. Returns address
func (c PasteldCli) AddKey(name string) string {
	cmd := c.withKeyringFlags("keys", "add", name, "--no-backup")
	out, _ := c.run(cmd)
	addr := gjson.Get(out, "address").String()
	require.NotEmpty(c.t, addr, "got %q", out)
	return addr
}

// AddKeyFromSeed recovers the key from given seed and add it to default keyring. Returns address
func (c PasteldCli) AddKeyFromSeed(name, mnemoic string) string {
	cmd := c.withKeyringFlags("keys", "add", name, "--recover")
	out, _ := c.runWithInput(cmd, strings.NewReader(mnemoic))
	addr := gjson.Get(out, "address").String()
	require.NotEmpty(c.t, addr, "got %q", out)
	return addr
}

// GetKeyAddr returns address
func (c PasteldCli) GetKeyAddr(name string) string {
	cmd := c.withKeyringFlags("keys", "show", name, "-a")
	out, _ := c.run(cmd)
	addr := strings.Trim(out, "\n")
	require.NotEmpty(c.t, addr, "got %q", out)
	return addr
}

const defaultSrcAddr = "node0"

// FundAddress sends the token amount to the destination address
func (c PasteldCli) FundAddress(destAddr, amount string) string {
	require.NotEmpty(c.t, destAddr)
	require.NotEmpty(c.t, amount)
	cmd := []string{"tx", "bank", "send", defaultSrcAddr, destAddr, amount}
	rsp := c.CustomCommand(cmd...)
	RequireTxSuccess(c.t, rsp)
	return rsp
}

func (c PasteldCli) QueryBalances(addr string) string {
	return c.CustomQuery("q", "bank", "balances", addr)
}

// QueryBalance returns balance amount for given denom.
// 0 when not found
func (c PasteldCli) QueryBalance(addr, denom string) int64 {
	raw := c.CustomQuery("q", "bank", "balance", addr, denom)
	require.Contains(c.t, raw, "amount", raw)
	return gjson.Get(raw, "balance.amount").Int()
}

// QueryTotalSupply returns total amount of tokens for a given denom.
// 0 when not found
func (c PasteldCli) QueryTotalSupply(denom string) int64 {
	raw := c.CustomQuery("q", "bank", "total-supply")
	require.Contains(c.t, raw, "amount", raw)
	return gjson.Get(raw, fmt.Sprintf("supply.#(denom==%q).amount", denom)).Int()
}

func (c PasteldCli) GetCometBFTValidatorSet() cmtservice.GetLatestValidatorSetResponse {
	args := []string{"q", "comet-validator-set"}
	got := c.CustomQuery(args...)

	// Create interface registry and proto codec
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	marshaler := codec.NewProtoCodec(interfaceRegistry)

	var res cmtservice.GetLatestValidatorSetResponse
	err := marshaler.UnmarshalJSON([]byte(got), &res)
	require.NoError(c.t, err, got)
	return res
}

// IsInCometBftValset returns true when the given pub key is in the current active tendermint validator set
func (c PasteldCli) IsInCometBftValset(valPubKey cryptotypes.PubKey) (cmtservice.GetLatestValidatorSetResponse, bool) {
	valResult := c.GetCometBFTValidatorSet()
	var found bool
	for _, v := range valResult.Validators {
		if v.PubKey.Equal(valPubKey) {
			found = true
			break
		}
	}
	return valResult, found
}

// SubmitGovProposal submit a gov v1 proposal
func (c PasteldCli) SubmitGovProposal(proposalJson string, args ...string) string {
	if len(args) == 0 {
		args = []string{"--from=" + defaultSrcAddr}
	}

	pathToProposal := filepath.Join(c.t.TempDir(), "proposal.json")
	err := os.WriteFile(pathToProposal, []byte(proposalJson), os.FileMode(0o744))
	require.NoError(c.t, err)
	c.t.Log("Submit upgrade proposal")
	return c.CustomCommand(append([]string{"tx", "gov", "submit-proposal", pathToProposal}, args...)...)
}

// SubmitAndVoteGovProposal submit proposal, let all validators vote yes and return proposal id
func (c PasteldCli) SubmitAndVoteGovProposal(proposalJson string, args ...string) string {
	rsp := c.SubmitGovProposal(proposalJson, args...)
	RequireTxSuccess(c.t, rsp)
	raw := c.CustomQuery("q", "gov", "proposals", "--depositor", c.GetKeyAddr(defaultSrcAddr))
	proposals := gjson.Get(raw, "proposals.#.id").Array()
	require.NotEmpty(c.t, proposals, raw)
	ourProposalID := proposals[len(proposals)-1].String() // last is ours
	for i := 0; i < c.nodesCount; i++ {
		go func(i int) { // do parallel
			c.t.Logf("Voting: validator %d\n", i)
			rsp = c.CustomCommand("tx", "gov", "vote", ourProposalID, "yes", "--from", c.GetKeyAddr(fmt.Sprintf("node%d", i)))
			RequireTxSuccess(c.t, rsp)
		}(i)
	}
	return ourProposalID
}

// Version returns the current version of the client binary
func (c PasteldCli) Version() string {
	v, ok := c.run([]string{"version"})
	require.True(c.t, ok)
	return v
}

// RequireTxSuccess require the received response to contain the success code
func RequireTxSuccess(t *testing.T, got string) {
	t.Helper()
	code, details := parseResultCode(t, got)
	require.Equal(t, int64(0), code, "non success tx code : %s", details)
}

// RequireTxFailure require the received response to contain any failure code and the passed msgsgs
func RequireTxFailure(t *testing.T, got string, containsMsgs ...string) {
	t.Helper()
	code, details := parseResultCode(t, got)
	require.NotEqual(t, int64(0), code, details)
	for _, msg := range containsMsgs {
		require.Contains(t, details, msg)
	}
}

func parseResultCode(t *testing.T, got string) (int64, string) {
	code := gjson.Get(got, "code")
	require.True(t, code.Exists(), "got response: %s", got)

	details := got
	if log := gjson.Get(got, "raw_log"); log.Exists() {
		details = log.String()
	}
	return code.Int(), details
}

var (
	// ErrOutOfGasMatcher requires error with "out of gas" message
	ErrOutOfGasMatcher RunErrorAssert = func(t assert.TestingT, err error, args ...interface{}) bool {
		const oogMsg = "out of gas"
		return expErrWithMsg(t, err, args, oogMsg)
	}
	// ErrTimeoutMatcher requires time out message
	ErrTimeoutMatcher RunErrorAssert = func(t assert.TestingT, err error, args ...interface{}) bool {
		const expMsg = "timed out waiting for tx to be included in a block"
		return expErrWithMsg(t, err, args, expMsg)
	}
	// ErrPostFailedMatcher requires post failed
	ErrPostFailedMatcher RunErrorAssert = func(t assert.TestingT, err error, args ...interface{}) bool {
		const expMsg = "post failed"
		return expErrWithMsg(t, err, args, expMsg)
	}
)

func expErrWithMsg(t assert.TestingT, err error, args []interface{}, expMsg string) bool {
	if ok := assert.Error(t, err, args); !ok {
		return false
	}
	var found bool
	for _, v := range args {
		if strings.Contains(fmt.Sprintf("%s", v), expMsg) {
			found = true
			break
		}
	}
	assert.True(t, found, "expected %q but got: %s", expMsg, args)
	return false // always abort
}
