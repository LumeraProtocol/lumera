package validator

import (
	"flag"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = flag.Set("test.v", "true")
	os.Exit(m.Run())
}
