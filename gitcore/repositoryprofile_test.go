package gitcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// The expected classifications follow pinned go-git's actual extension table;
// unsupported declarations must be observed without making Open tolerate them.
func TestRepositoryReadProfile(t *testing.T) {
	for _, c := range []struct {
		name, version, extension string
		wantNative               bool
	}{
		{"plain", "0", "", false},
		{"noop", "0", "noop = true", false},
		{"noop-v1", "1", "noop-v1 = true", false},
		{"worktree-shim", "0", "worktreeConfig = true", false},
		{"reftable-declaration", "1", "refStorage = reftable", true},
		{"files-declaration-still-unsupported", "1", "refStorage = files", true},
		{"partial-clone-declaration", "1", "partialClone = origin", true},
		{"future-extension", "1", "futureStorage = enabled", true},
		{"noop-v1-in-format0", "0", "noop-v1 = true", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := gittest.NewFixture(t)
			cfg := "[core]\nrepositoryformatversion = " + c.version + "\nbare = false\n"
			if c.extension != "" {
				cfg += "[extensions]\n" + c.extension + "\n"
			}
			if err := os.WriteFile(filepath.Join(f.Dir, ".git", "config"), []byte(cfg), 0644); err != nil {
				t.Fatal(err)
			}
			why, err := NativeRepositoryReadReason(f.Dir)
			if err != nil || (why != "") != c.wantNative {
				t.Fatalf("pre-open profile=%q err=%v wantNative=%v", why, err, c.wantNative)
			}
			_, openErr := Open(f.Dir)
			if c.wantNative && openErr == nil {
				t.Fatal("fixture does not demonstrate the selected reader's unsupported extension")
			}
			if !c.wantNative && openErr != nil {
				t.Fatalf("supported declaration stopped opening: %v", openErr)
			}
		})
	}
}

func TestRepositoryReadProfileErrorsStayErrors(t *testing.T) {
	f := gittest.NewFixture(t)
	path := filepath.Join(f.Dir, ".git", "config")
	if err := os.WriteFile(path, []byte("[extensions\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if why, err := NativeRepositoryReadReason(f.Dir); err == nil || why != "" {
		t.Fatalf("malformed config demoted to native: %q %v", why, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	if why, err := NativeRepositoryReadReason(f.Dir); err == nil || why != "" {
		t.Fatalf("config I/O failure demoted to native: %q %v", why, err)
	}
}

func TestRepositoryReadProfileRefreshes(t *testing.T) {
	f := gittest.NewFixture(t)
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.Dir, ".git", "config")
	if err := os.WriteFile(path, []byte("[core]\nrepositoryformatversion = 1\n[extensions]\nrefStorage = reftable\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if why, err := r.NativeDiscoveryReason(); err != nil || !strings.Contains(why, "refstorage") {
		t.Fatalf("cached handle ignored current unsupported declaration: %q %v", why, err)
	}
}

func TestRepositoryReadEmptyFormat(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing-default", true: "explicit-empty"}[explicit], func(t *testing.T) {
			f := gittest.NewFixture(t)
			cfg := "[core]\nbare = false\n"
			if explicit {
				cfg += "repositoryformatversion =\n"
			}
			if err := os.WriteFile(filepath.Join(f.Dir, ".git", "config"), []byte(cfg), 0644); err != nil {
				t.Fatal(err)
			}
			_, nativeErr := f.Git("rev-parse", "HEAD")
			if (nativeErr != nil) != explicit {
				t.Fatalf("native format fixture: explicit=%v err=%v", explicit, nativeErr)
			}
			why, err := NativeRepositoryReadReason(f.Dir)
			if err != nil || (why != "") != explicit {
				t.Fatalf("empty numeric format lost native behavior: explicit=%v native=%q err=%v", explicit, why, err)
			}
		})
	}
}
