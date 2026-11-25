package acctest

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRandomName(t *testing.T) {
	t.Parallel()
	name := RandomName("basic-aws")

	require.Regexp(t, regexp.MustCompile(`^tf-acc-test-basic-aws-[a-z0-9]{8}$`), name)
	require.NotEqual(t, name, RandomName("basic-aws"))
}
