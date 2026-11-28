package deskkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStandaloneChangeClientExposesOnlyScopedOperations(t *testing.T) {
	typ := reflect.TypeOf(NewStandaloneChanges).Out(0)
	var methods []string
	for i := 0; i < typ.NumMethod(); i++ {
		methods = append(methods, typ.Method(i).Name)
	}
	want := []string{"AddReaction", "Change", "ChangedFiles", "ChangesUpdatedSince", "Comments", "OpenChangeRows", "OpenChanges", "PublishComment"}
	if typ != reflect.TypeOf((*StandaloneChanges)(nil)) || !reflect.DeepEqual(methods, want) {
		t.Fatalf("standalone constructor exposed %v with methods %v; want only %v", typ, methods, want)
	}
}

type standaloneTestTransport func(*http.Request) (*http.Response, error)

func (f standaloneTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestStandaloneTransportRejectsOtherOrigins(t *testing.T) {
	calls := 0
	client := standaloneBoundHTTP(&http.Client{Transport: standaloneTestTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}, GitHubAPIBase)
	resp, err := client.Get("https://other.example/fixture")
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "left its configured HTTPS origin") || calls != 0 {
		t.Fatalf("off-origin request reached transport %d times: %v", calls, err)
	}
}

func TestStandaloneTransportRejectsRedirectResponse(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://api.github.com/fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	transport := standaloneOriginTransport{host: "api.github.com", next: standaloneTestTransport(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusTemporaryRedirect,
			Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	response, err := transport.RoundTrip(request)
	if response != nil {
		_ = response.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "forge returned a redirect") || response != nil {
		t.Fatalf("redirect reached the provider: %v", err)
	}
}

// standaloneEntrypointSites finds both qualified and unqualified references to
// the standalone entry points in shipped code. The names are reserved.
func standaloneEntrypointSites(t *testing.T, root string) map[string]int {
	t.Helper()
	sites := map[string]int{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case "testdata", "vendor", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		ast.Inspect(file, func(node ast.Node) bool {
			if id, ok := node.(*ast.Ident); ok {
				switch id.Name {
				case "NewStandaloneChanges", "MintStandaloneGitHubToken":
					sites[rel+":"+id.Name]++
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return sites
}

func TestStandaloneEntrypointsConfined(t *testing.T) {
	want := map[string]int{
		"internal/deskkit/standalonechanges.go:NewStandaloneChanges":         1,
		"internal/deskkit/standalonecredential.go:MintStandaloneGitHubToken": 1,
		"forgechange/client.go:NewStandaloneChanges":                         1,
		"forgechange/credential.go:MintStandaloneGitHubToken":                1,
	}
	got := standaloneEntrypointSites(t, deskTreeRoot)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("standalone entry points must be confined to their definitions and the forgechange facade: got %v, want %v", got, want)
	}
}

func TestStandaloneEntrypointScannerSeesRenamedImport(t *testing.T) {
	root := t.TempDir()
	const source = `package witness
import kit "github.com/medici-finance/assay/tools/desk/internal/deskkit"
var open = kit.NewStandaloneChanges
var mint = kit.MintStandaloneGitHubToken
`
	if err := os.WriteFile(filepath.Join(root, "caller.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"caller.go:NewStandaloneChanges": 1, "caller.go:MintStandaloneGitHubToken": 1}
	if got := standaloneEntrypointSites(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("scan missed planted standalone entry points: got %v, want %v", got, want)
	}
}
