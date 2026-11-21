package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type acceptancePreparationInput struct {
	Runs         []acceptanceRun         `json:"runs"`
	Dispositions []acceptanceDisposition `json:"dispositions"`
}

func locateAcceptanceBrief(root, ref string) (*BriefFile, error) {
	local, err := resolveLocalBriefRef(root, ref)
	if err != nil {
		return nil, err
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		return nil, err
	}
	stream, number, ok := strings.Cut(local, "/")
	if !ok || stream == "" || number == "" {
		return nil, fmt.Errorf("acceptance adjudication: invalid brief reference %q", ref)
	}
	var found *BriefFile
	for _, s := range streams {
		if s.Name != stream {
			continue
		}
		row := findRow(s, number)
		if row == nil || (row.Status != "implemented" && row.Status != "verified") {
			return nil, fmt.Errorf("acceptance adjudication: %s is neither implemented nor verified", local)
		}
		for _, path := range briefFilePaths(s) {
			if id, _, ok := expectedBriefID(path); !ok || id != local {
				continue
			}
			bf, ok, err := parseBriefFile(path)
			if err != nil {
				return nil, err
			}
			if ok && bf.Gate == "human" {
				if found != nil {
					return nil, fmt.Errorf("acceptance adjudication: multiple brief files match %s", local)
				}
				found = bf
			}
		}
	}
	if found == nil {
		return nil, fmt.Errorf("acceptance adjudication: cannot locate human-gated brief %s", local)
	}
	return found, nil
}

func prepareAcceptanceManifest(root string, bf *BriefFile, input acceptancePreparationInput) (acceptanceManifest, error) {
	source, err := readAcceptanceSource(root, bf.Path)
	if err != nil {
		return acceptanceManifest{}, err
	}
	def, err := readAcceptanceDefinition(root, source)
	if err != nil {
		return acceptanceManifest{}, err
	}
	m := acceptanceManifest{
		Source: source, AcceptanceVersion: def.version, Original: def.original,
		ProtectedVerifyHash: def.tableHash, Runs: input.Runs, Dispositions: input.Dispositions,
	}
	if err := m.checkDispositions(source, false); err != nil {
		return acceptanceManifest{}, err
	}
	latest, err := flipLatestPass(bf.Evidence)
	if err != nil {
		return acceptanceManifest{}, err
	}
	if err := checkAcceptanceLatestRun(latest.Run, input.Runs); err != nil {
		return acceptanceManifest{}, err
	}
	if err := creditAcceptanceRuns(bf.Path, def.verify, input.Runs, false); err != nil {
		return acceptanceManifest{}, err
	}
	return m, nil
}

// runAcceptanceRoute renders preparation or a card, or applies a verified
// close locally. Only the canonical workflow publishes or pushes results.
func runAcceptanceRoute(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 && (args[0] == "issue" || args[0] == "close") {
		return runAcceptanceRouteWorkflow(args, stdout, stderr)
	}
	if len(args) == 0 || args[0] != "prepare" {
		fmt.Fprintln(stderr, "usage: statusgen acceptance-route {prepare|issue|close} --root DIR --brief REF")
		return 2
	}
	fs := flag.NewFlagSet("acceptance-route prepare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "canonical repository checkout")
	ref := fs.String("brief", "", "canonical brief reference")
	input := fs.String("input", "", "JSON file with selected runs and proposed dispositions")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 || *ref == "" || *input == "" {
		fmt.Fprintln(stderr, "acceptance-route prepare requires --brief and --input")
		return 2
	}
	path, err := filepath.Abs(*root)
	if err == nil {
		err = writeAcceptancePreparation(path, *ref, *input, stdout)
	}
	if err != nil {
		fmt.Fprintln(stderr, "statusgen:", err)
		return 1
	}
	return 0
}

func writeAcceptancePreparation(root, ref, inputPath string, out io.Writer) error {
	in, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer in.Close()
	dec := json.NewDecoder(in)
	dec.DisallowUnknownFields()
	var input acceptancePreparationInput
	if err := dec.Decode(&input); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("acceptance preparation: expected exactly one input document")
	}
	bf, err := locateAcceptanceBrief(root, ref)
	if err != nil {
		return err
	}
	m, err := prepareAcceptanceManifest(root, bf, input)
	if err != nil {
		return err
	}
	body, err := acceptancePreparationBody(m)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, body)
	return err
}
