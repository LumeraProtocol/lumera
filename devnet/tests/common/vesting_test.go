package common

import "testing"

func vcontains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestVestingCreateArgsContinuous(t *testing.T) {
	args := VestingCreateArgs("faucet", "lumera1to", "5000000ulume", 1800000000, false)
	for _, want := range []string{"vesting", "create-vesting-account", "lumera1to", "5000000ulume", "1800000000"} {
		if !vcontains(args, want) {
			t.Errorf("continuous vesting args missing %q: %v", want, args)
		}
	}
	if !vcontains(args, "--from") || !vcontains(args, "faucet") {
		t.Errorf("must sign with funder: %v", args)
	}
	if vcontains(args, "--delayed") {
		t.Errorf("continuous vesting must NOT pass --delayed: %v", args)
	}
}

func TestVestingCreateArgsDelayed(t *testing.T) {
	args := VestingCreateArgs("faucet", "lumera1to", "5000000ulume", 1800000000, true)
	if !vcontains(args, "--delayed") {
		t.Errorf("delayed vesting must pass --delayed: %v", args)
	}
}

func TestPermanentLockedArgs(t *testing.T) {
	args := PermanentLockedArgs("faucet", "lumera1to", "5000000ulume")
	for _, want := range []string{"vesting", "create-permanent-locked-account", "lumera1to", "5000000ulume"} {
		if !vcontains(args, want) {
			t.Errorf("permanent-locked args missing %q: %v", want, args)
		}
	}
	if !vcontains(args, "--from") || !vcontains(args, "faucet") {
		t.Errorf("must sign with funder: %v", args)
	}
}
