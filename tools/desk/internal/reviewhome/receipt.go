// Package reviewhome ties a confirmed review to the per-worktree Git administration
// directory created by dispatch. A later checkout at the same path gets a new stamp.
package reviewhome

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const ReceiptFile = "assay-review-finished.json"
const ExitFile = "assay-review-exited"

type Stamp struct{ Key, Repo, Store, Attempt, ReviewAttempt, HandoffSourceRef string }

func ReadStamp(admin string) (Stamp, error) {
	data, err := os.ReadFile(filepath.Join(admin, "config.worktree"))
	if errors.Is(err, fs.ErrNotExist) {
		return Stamp{}, nil
	}
	if err != nil {
		return Stamp{}, err
	}
	cfg := config.New()
	if err := config.NewDecoder(bytes.NewReader(data)).Decode(cfg); err != nil {
		return Stamp{}, fmt.Errorf("cannot parse worktree config")
	}
	sec := cfg.Section("assay")
	get := func(k string) string { return strings.TrimSpace(sec.Option(k)) }
	return Stamp{get("runKey"), get("claimRepo"), get("claimStore"), get("dispatchRef"), get("reviewAttempt"), get("handoffSourceRef")}, nil
}

type Receipt struct {
	Home         string `json:"home"`
	Repo         string `json:"repo"`
	PR           int    `json:"pr"`
	Key          string `json:"claim"`
	Attempt      string `json:"dispatch_ref"`
	Head         string `json:"review_head"`
	CheckoutHead string `json:"checkout_head"`
	Review       string `json:"confirmed_review"`
}

// Identity verifies both sides of the registration, the exact dispatch and its
// repo/PR. Read HEAD directly: a symbolic branch can never be a disposable review.
func (r Receipt) Identity() (string, string, error) {
	if !filepath.IsAbs(r.Home) ||
		r.PR <= 0 || !deskkit.ValidDispatchRef(r.Key, r.Attempt) || deskkit.ValidateReviewClaimKey(r.Key, r.Repo, r.PR) != nil {
		return "", "", fmt.Errorf("invalid review home, claim or dispatch attempt")
	}
	admin, err := gitcore.GitDir(r.Home)
	if err != nil {
		return "", "", err
	}
	common, err := gitcore.CommonDir(r.Home)
	if err != nil || filepath.Dir(admin) != filepath.Join(common, "worktrees") {
		return "", "", fmt.Errorf("expected a linked review checkout")
	}
	back, err := os.ReadFile(filepath.Join(admin, "gitdir"))
	if err != nil {
		return "", "", err
	}
	p := strings.TrimSpace(string(back))
	if !filepath.IsAbs(p) {
		p = filepath.Join(admin, p)
	}
	p, err = filepath.EvalSymlinks(p)
	if err != nil || p != filepath.Join(r.Home, ".git") {
		return "", "", fmt.Errorf("review checkout registration changed")
	}
	stamp, err := ReadStamp(admin)
	if err != nil {
		return "", "", err
	}
	if stamp.Key != r.Key || stamp.Attempt != r.Attempt || stamp.ReviewAttempt != r.Attempt || !strings.EqualFold(stamp.Repo, r.Repo) {
		return "", "", fmt.Errorf("review dispatch stamp changed")
	}
	b, err := os.ReadFile(filepath.Join(admin, "HEAD"))
	if err != nil {
		return "", "", err
	}
	head := strings.TrimSpace(string(b))
	if !fullOID(head) {
		return "", "", fmt.Errorf("review checkout is not detached")
	}
	return admin, head, nil
}

func fullOID(s string) bool {
	return (len(s) == 40 || len(s) == 64) && strings.Trim(s, "0123456789abcdef") == ""
}

// Confirm writes before claim release, so an interrupted finish can be retried.
// The forge has already read back the review with this verdict body at this head.
func (r Receipt) Confirm() error {
	admin, head, err := r.Identity()
	if err != nil {
		return err
	}
	r.CheckoutHead = head
	if !fullOID(r.Head) || r.Review == "" {
		return fmt.Errorf("missing confirmed review")
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return writeOnce(admin, ReceiptFile, b)
}

func Read(admin string) (Receipt, error) {
	var r Receipt
	b, err := os.ReadFile(filepath.Join(admin, ReceiptFile))
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(b, &r)
	if err == nil && (!fullOID(r.Head) || !fullOID(r.CheckoutHead) || r.Review == "") {
		err = fmt.Errorf("incomplete confirmed review")
	}
	return r, err
}

func (r Receipt) Acknowledge(admin string) error {
	return writeOnce(admin, ExitFile, []byte(r.Attempt))
}

// Publish a complete record without replacing a concurrent attempt or resetting backlog age.
func writeOnce(admin, name string, b []byte) error {
	tmp, err := os.CreateTemp(admin, ".review-handoff-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(b)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	path := filepath.Join(admin, name)
	err = os.Link(tmp.Name(), path)
	if !errors.Is(err, fs.ErrExist) {
		return err
	}
	old, err := os.ReadFile(path)
	if err == nil && !bytes.Equal(old, b) {
		return fmt.Errorf("different review already recorded in %s", name)
	}
	return err
}
