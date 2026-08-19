package docparse

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// AnyDoc builds the subprocess adapter for the anydoc CLI. The binary path is
// injected by the caller; the supply-chain pinning (version + checksum) is an
// open decision (AI docs index §5 item 6) and lives outside this package.
// When the binary is missing or fails, the parser reports ErrParserUnavailable
// — it never fabricates a parse.
func AnyDoc(binPath string, timeout time.Duration) DocumentParser {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return anydocParser{bin: binPath, timeout: timeout}
}

type anydocParser struct {
	bin     string
	timeout time.Duration
}

func (p anydocParser) Parse(ctx context.Context, src Source) (ParsedDocument, error) {
	if err := ctx.Err(); err != nil {
		return ParsedDocument{}, err
	}
	if err := CheckSize(src); err != nil {
		return ParsedDocument{}, err
	}
	if p.bin == "" {
		return ParsedDocument{}, ErrParserUnavailable
	}
	if _, err := os.Stat(p.bin); err != nil {
		return ParsedDocument{}, fmt.Errorf("%w: %v", ErrParserUnavailable, err)
	}

	dir, err := os.MkdirTemp("", "docparse-anydoc-")
	if err != nil {
		return ParsedDocument{}, fmt.Errorf("%w: %v", ErrParserUnavailable, err)
	}
	defer os.RemoveAll(dir)

	inPath := filepath.Join(dir, "input")
	if err := os.WriteFile(inPath, src.Data, 0o600); err != nil {
		return ParsedDocument{}, fmt.Errorf("%w: %v", ErrParserUnavailable, err)
	}

	runCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// The CLI contract is pinned when the anydoc binary supply chain lands
	// (W5); until then the adapter invokes `anydoc <file>` and reads GFM from
	// stdout.
	cmd := exec.CommandContext(runCtx, p.bin, inPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return ParsedDocument{}, fmt.Errorf("%w: timed out", ErrParserUnavailable)
		}
		return ParsedDocument{}, fmt.Errorf("%w: anydoc exited: %v (%s)", ErrParserUnavailable, err, stderr.String())
	}

	format := DetectFormat(src.Filename, src.Data)
	mode := EvidenceQuote
	if format == "pdf" {
		// First-round PDF text comes from anydoc without coordinates; evidence
		// becomes available only after a PaddleOCR pass (lazy evidence).
		mode = EvidenceUnavailable
	}

	doc := ParsedDocument{
		Markdown:     stdout.String(),
		Format:       format,
		EvidenceMode: mode,
	}
	if stderr.Len() > 0 {
		doc.Warnings = append(doc.Warnings, stderr.String())
	}
	return doc, nil
}
