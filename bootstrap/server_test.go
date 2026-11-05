package bootstrap

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pole-io/pole-server/pkg/common/utils/valid"
)

func TestSelfRegistrationVersionRespectsInstanceStorageLimit(t *testing.T) {
	short := "v1.2.3"
	if got := selfRegistrationVersion(short); got != short {
		t.Fatalf("short version = %q, want %q", got, short)
	}

	long := strings.Repeat("版", valid.MaxDbInsVersionLength+1)
	got := selfRegistrationVersion(long)
	if utf8.RuneCountInString(got) != valid.MaxDbInsVersionLength {
		t.Fatalf("long version rune count = %d, want %d", utf8.RuneCountInString(got), valid.MaxDbInsVersionLength)
	}
}
