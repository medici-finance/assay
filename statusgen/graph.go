package main

// graph.go — the derived-only graph export (`--graph dot|jsonl`).
//
// Briefs, findings, intake entries and issues are already a typed graph
// statusgen parses every run; this mode emits that latent graph WITHOUT
// building any new store, cache, or graph DB. It is read-only: it never reads
// or writes STATUS.md and never touches any generated register view. Both
// formats are byte-deterministic across runs (same discipline as the register
// views) so a consumer can diff two exports.
//
// Entity IDs are the existing typed IDs — a stream node is its stream name, a
// brief node is `<stream>/<NN>`, a finding node is its `F-<slug>` (or legacy
// `F-NN`) id, an intake node is its `I-<slug>` id, an issue node is `#<num>`.
// No parallel ID scheme is invented; these are the same IDs the board and the
// register spec already use.
//
// Edges:
//   - contains  stream → brief   (membership; connects each stream node to the
//                                  briefs it owns, so stream nodes are not
//                                  orphaned in the render)
//   - depends   brief  → brief   (brief-v1 depends:)
//   - unblocks  brief  → brief   (brief-v1 unblocks:)
//   - affects   finding→ brief   (a finding's affects: entry naming a brief;
//                                  a bare-stream affects points at the stream)
//   - sources   brief  → finding/intake (a brief-v1 sources: entry that names a
//                                  register entry by its typed id)
//   - issues    stream/brief → issue (issues: frontmatter + a brief's
//                                  decision-issue)
//
// Issue nodes exist only "as referenced" — one is emitted the first time an
// issues edge points at it.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// graphNodeType is the closed set of node kinds. Ordered here for a stable
// primary sort key so the output ordering is a property of the type, not of
// map-iteration order.
const (
	graphTypeStream  = "stream"
	graphTypeBrief   = "brief"
	graphTypeFinding = "finding"
	graphTypeIntake  = "intake"
	graphTypeIssue   = "issue"
)

// graphTypeRank gives each node/edge-endpoint type a deterministic sort rank.
var graphTypeRank = map[string]int{
	graphTypeStream:  0,
	graphTypeBrief:   1,
	graphTypeFinding: 2,
	graphTypeIntake:  3,
	graphTypeIssue:   4,
}

type graphNode struct {
	ID    string
	Type  string
	Label string
}

type graphEdge struct {
	Type string
	From string
	To   string
}

// graphModel is the derived graph for one root: a deduped node set keyed by id
// and a deduped edge list. It holds no file handles and owns no I/O — building
// it is pure over the already-parsed tree.
type graphModel struct {
	nodes   map[string]graphNode
	edgeSet map[string]bool
	edges   []graphEdge
}

func newGraphModel() *graphModel {
	return &graphModel{nodes: map[string]graphNode{}, edgeSet: map[string]bool{}}
}

// addNode inserts a node, or upgrades its label if a later reference carries a
// non-empty one (the first, lazily-created issue reference has no label; a
// later explicit one would). Type is never rewritten.
func (g *graphModel) addNode(id, typ, label string) {
	if id == "" {
		return
	}
	if existing, ok := g.nodes[id]; ok {
		if existing.Label == "" && label != "" {
			existing.Label = label
			g.nodes[id] = existing
		}
		return
	}
	g.nodes[id] = graphNode{ID: id, Type: typ, Label: label}
}

// addEdge records a directed edge, deduped on (type, from, to). Endpoints must
// already be nodes; a dangling endpoint is dropped rather than inventing a node
// for it (a stale affects/sources reference is out of scope for the export).
func (g *graphModel) addEdge(typ, from, to string) {
	if from == "" || to == "" {
		return
	}
	if _, ok := g.nodes[from]; !ok {
		return
	}
	if _, ok := g.nodes[to]; !ok {
		return
	}
	key := typ + "\x00" + from + "\x00" + to
	if g.edgeSet[key] {
		return
	}
	g.edgeSet[key] = true
	g.edges = append(g.edges, graphEdge{Type: typ, From: from, To: to})
}

// sortedNodes returns the node set ordered by (type-rank, id) — a total order
// independent of insertion/map order, so the byte output is deterministic.
func (g *graphModel) sortedNodes() []graphNode {
	out := make([]graphNode, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		if graphTypeRank[out[i].Type] != graphTypeRank[out[j].Type] {
			return graphTypeRank[out[i].Type] < graphTypeRank[out[j].Type]
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// sortedEdges returns edges ordered by (type, from, to).
func (g *graphModel) sortedEdges() []graphEdge {
	out := make([]graphEdge, len(g.edges))
	copy(out, g.edges)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

// graphRegisterTokenRe matches a typed register id (F-… / I-…) embedded in
// free-text sources prose. The slug alphabet is [a-z0-9-]; the legacy numeric
// form F-NN / I-NN is covered by the same shape. Matched tokens are kept only
// when they name a KNOWN register entry, so prose that merely mentions an
// id-shaped string produces no edge.
var graphRegisterTokenRe = regexp.MustCompile(`[FI]-[a-z0-9][a-z0-9-]*`)

// buildGraph derives the graph for one root from the already-parsed tree. It is
// read-only and never consults STATUS.md or a generated register view.
//
// Three-state discipline: an unreadable streams tree or intake register is
// surfaced as an error (could-not-check), never rendered as an empty graph.
func buildGraph(root string) (*graphModel, error) {
	streams, findings, err := loadStreams(root)
	if err != nil {
		return nil, err
	}
	intake, err := loadIntake(root)
	if err != nil {
		return nil, err
	}

	g := newGraphModel()

	// Index the node namespaces so edge targets can be resolved to real nodes.
	streamNames := map[string]bool{}
	briefIDs := map[string]bool{}

	// Streams + their briefs (nodes), and the contains membership edge.
	for _, s := range streams {
		g.addNode(s.Name, graphTypeStream, s.Name)
		streamNames[s.Name] = true
		for _, b := range s.Briefs {
			id := s.Name + "/" + b.Num
			g.addNode(id, graphTypeBrief, b.Title)
			briefIDs[id] = true
		}
	}
	// contains edges after all brief nodes exist (addEdge requires both ends).
	for _, s := range streams {
		for _, b := range s.Briefs {
			g.addEdge("contains", s.Name, s.Name+"/"+b.Num)
		}
		for _, iss := range s.Issues {
			issueID := graphIssueID(iss)
			g.addNode(issueID, graphTypeIssue, "")
			g.addEdge("issues", s.Name, issueID)
		}
	}

	// Findings (nodes). Affects edges are added after the whole node set exists.
	for _, f := range findings {
		g.addNode(f.ID, graphTypeFinding, f.Title)
	}

	// Intake entries (nodes). A referenced decision-issue becomes an issues edge.
	knownRegisterIDs := map[string]bool{}
	for _, f := range findings {
		knownRegisterIDs[f.ID] = true
	}
	for _, e := range intake {
		g.addNode(e.ID, graphTypeIntake, e.Title)
		knownRegisterIDs[e.ID] = true
		if e.DecisionIssue != "" {
			issueID := "#" + strings.TrimPrefix(strings.TrimSpace(e.DecisionIssue), "#")
			g.addNode(issueID, graphTypeIssue, "")
			g.addEdge("issues", e.ID, issueID)
		}
	}

	// finding → brief/stream affects edges.
	for _, f := range findings {
		for _, a := range f.Affects {
			if target, ok := graphAffectsTarget(a, briefIDs, streamNames); ok {
				g.addEdge("affects", f.ID, target)
			}
		}
	}

	// Brief-file edges: depends / unblocks / sources / issues. Parsed straight
	// from the brief files (the authoritative typed-edge source); a legacy or
	// opted-out brief simply yields no typed edges.
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok || bf == nil {
				continue // parse failures are the lint's job, not the export's
			}
			from := bf.Brief
			if !briefIDs[from] {
				continue // a brief file whose id has no README row: skip its edges
			}
			for _, dep := range bf.Depends {
				g.addEdge("depends", from, dep)
			}
			for _, unb := range bf.Unblocks {
				g.addEdge("unblocks", from, unb)
			}
			for _, src := range bf.Sources {
				for _, tok := range graphRegisterTokenRe.FindAllString(src, -1) {
					if knownRegisterIDs[tok] {
						g.addEdge("sources", from, tok)
					}
				}
			}
			for _, iss := range bf.Issues {
				issueID := graphIssueID(iss)
				g.addNode(issueID, graphTypeIssue, "")
				g.addEdge("issues", from, issueID)
			}
			if bf.DecisionIssue != 0 {
				issueID := graphIssueID(bf.DecisionIssue)
				g.addNode(issueID, graphTypeIssue, "")
				g.addEdge("issues", from, issueID)
			}
		}
	}

	return g, nil
}

// graphIssueID renders a GitHub issue number as its node id.
func graphIssueID(n int) string {
	return fmt.Sprintf("#%d", n)
}

// graphAffectsTarget resolves a finding's affects: entry to an existing node.
// Accepted forms: `<stream>/<NN>`, `<stream>/brief-<NN>` (a brief), or bare
// `<stream>` (the stream). Returns ok=false when the named node does not exist
// — a stale affects reference is dropped, not materialized.
func graphAffectsTarget(affects string, briefIDs, streamNames map[string]bool) (string, bool) {
	a := strings.TrimSpace(affects)
	if a == "" {
		return "", false
	}
	if i := strings.Index(a, "/"); i >= 0 {
		stream := a[:i]
		num := strings.TrimPrefix(a[i+1:], "brief-")
		id := stream + "/" + num
		if briefIDs[id] {
			return id, true
		}
		return "", false
	}
	if streamNames[a] {
		return a, true
	}
	return "", false
}

// ----- emitters -----

type graphJSONLNode struct {
	Kind  string `json:"kind"` // "node"
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label,omitempty"`
}

type graphJSONLEdge struct {
	Kind string `json:"kind"` // "edge"
	Type string `json:"type"`
	From string `json:"from"`
	To   string `json:"to"`
}

// emitGraphJSONL writes one JSON object per line: every node (sorted) then
// every edge (sorted). Deterministic across runs.
func emitGraphJSONL(w io.Writer, g *graphModel) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, n := range g.sortedNodes() {
		if err := enc.Encode(graphJSONLNode{Kind: "node", ID: n.ID, Type: n.Type, Label: n.Label}); err != nil {
			return err
		}
	}
	for _, e := range g.sortedEdges() {
		if err := enc.Encode(graphJSONLEdge{Kind: "edge", Type: e.Type, From: e.From, To: e.To}); err != nil {
			return err
		}
	}
	return nil
}

// graphNodeShape maps a node type to a Graphviz shape so a rendered DOT graph
// is legible without any external styling. Unknown types fall back to a plain
// box. Pure text output — no Graphviz library and no local `dot` binary is
// required to produce or to test it.
func graphNodeShape(typ string) string {
	switch typ {
	case graphTypeStream:
		return "box3d"
	case graphTypeBrief:
		return "box"
	case graphTypeFinding:
		return "octagon"
	case graphTypeIntake:
		return "note"
	case graphTypeIssue:
		return "ellipse"
	default:
		return "box"
	}
}

// dotQuote escapes a string for use as a DOT double-quoted id/label.
func dotQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", " ")
	return "\"" + s + "\""
}

// emitGraphDOT writes a Graphviz digraph: node declarations (sorted, one per
// line, carrying type + label) then edges (sorted, labelled with the edge
// type). Deterministic across runs.
func emitGraphDOT(w io.Writer, g *graphModel) error {
	var b strings.Builder
	b.WriteString("digraph assay {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [fontsize=10];\n")
	for _, n := range g.sortedNodes() {
		label := n.Label
		if label == "" {
			label = n.ID
		}
		fmt.Fprintf(&b, "  %s [type=%s shape=%s label=%s];\n",
			dotQuote(n.ID), n.Type, graphNodeShape(n.Type), dotQuote(label))
	}
	for _, e := range g.sortedEdges() {
		fmt.Fprintf(&b, "  %s -> %s [label=%s];\n",
			dotQuote(e.From), dotQuote(e.To), dotQuote(e.Type))
	}
	b.WriteString("}\n")
	_, err := io.WriteString(w, b.String())
	return err
}

// runGraph is the `--graph <format>` entry point: a self-contained, read-only,
// STATUS.md-free sub-command. Returns a process exit code.
func runGraph(root, format string) int {
	g, err := buildGraph(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: --graph:", err)
		return 1
	}
	switch format {
	case "dot":
		if err := emitGraphDOT(os.Stdout, g); err != nil {
			fmt.Fprintln(os.Stderr, "statusgen: --graph:", err)
			return 1
		}
	case "jsonl":
		if err := emitGraphJSONL(os.Stdout, g); err != nil {
			fmt.Fprintln(os.Stderr, "statusgen: --graph:", err)
			return 1
		}
	default:
		fmt.Fprintf(os.Stderr, "statusgen: --graph wants \"dot\" or \"jsonl\", got %q\n", format)
		return 2
	}
	return 0
}
