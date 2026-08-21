package version

import (
	"strings"
	"testing"
)

func TestVersionIsSet(t *testing.T) {
	t.Parallel()

	if Version() == "" {
		t.Error("the release is unset, so a build reports nothing")
	}
}

func TestFullWithAndWithoutACommit(t *testing.T) {
	saved := commit

	defer func() { commit = saved }()

	commit = "abc1234"

	if got, want := Full(), Version()+" (abc1234)"; got != want {
		t.Errorf("want %q, got %q", want, got)
	}

	commit = ""

	if got := Full(); strings.Contains(got, "()") {
		t.Errorf("an empty commit should be omitted entirely, got %q", got)
	}
	if !strings.HasPrefix(Full(), Version()) {
		t.Errorf("want the release to lead, got %q", Full())
	}
}

func TestShortRevisionIsIgnored(t *testing.T) {
	t.Parallel()

	if got := vcsRevision(); got != "" && len(got) != 7 {
		t.Errorf("want either nothing or seven characters, got %q", got)
	}
}
