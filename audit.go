package logger

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// AuditSink is a tamper-evident sink: every record is hash-chained
// (hash = sha256(prevHash || canonicalRecordBytes)) so any insertion,
// deletion, reordering or edit of a past entry breaks the chain and is
// detectable by VerifyAudit. For security/audit logs that must be provably
// intact.
//
// Line format (greppable, verifier-friendly):
//
//	<hash-hex> <prev-hex> <seq> <canonical-json>\n
//
// The canonical JSON is taken verbatim from the line during verification, so
// the hash covers the exact emitted bytes.
type AuditSink struct {
	w    io.Writer
	enc  Encoder
	mu   sync.Mutex
	seq  uint64
	prev string
}

// GenesisHash is the chain anchor for the first record.
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// NewAuditSink wraps w. The encoder must be deterministic (the built-in
// JSONEncoder is — fields are emitted in slice order).
func NewAuditSink(w io.Writer, enc Encoder) *AuditSink {
	return &AuditSink{w: w, enc: enc, prev: GenesisHash}
}

// Emit implements Sink.
func (a *AuditSink) Emit(r *Record) error {
	buf := getBuffer()
	a.enc.Encode(buf, r)
	canonical := buf.b
	// trim the encoder's trailing newline so the chain covers just the record
	for len(canonical) > 0 && (canonical[len(canonical)-1] == '\n') {
		canonical = canonical[:len(canonical)-1]
	}

	a.mu.Lock()
	h := sha256.New()
	h.Write([]byte(a.prev))
	h.Write(canonical)
	sum := hex.EncodeToString(h.Sum(nil))
	line := fmt.Sprintf("%s %s %d ", sum, a.prev, a.seq)
	_, err := a.w.Write(append([]byte(line), append(canonical, '\n')...))
	a.prev = sum
	a.seq++
	a.mu.Unlock()

	putBuffer(buf)
	return err
}

// Sync implements Sink.
func (a *AuditSink) Sync() error {
	if s, ok := a.w.(interface{ Sync() error }); ok {
		return s.Sync()
	}
	return nil
}

// Close implements Sink.
func (a *AuditSink) Close() error {
	if c, ok := a.w.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// AuditResult reports the outcome of chain verification.
type AuditResult struct {
	Records uint64
	OK      bool
	// BrokenAtSeq is the sequence number where the chain first failed
	// (-1 if OK).
	BrokenAtSeq int64
	Reason      string
}

// VerifyAudit re-walks a hash-chained audit stream and proves it is intact:
// every line's hash must equal sha256(prev || canonical), each line's prev
// must equal the previous line's hash, and seq must be contiguous from 0.
func VerifyAudit(rd io.Reader) AuditResult {
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	prev := GenesisHash
	var n uint64
	for sc.Scan() {
		line := sc.Text()
		parts := strings.SplitN(line, " ", 4)
		if len(parts) != 4 {
			return AuditResult{Records: n, OK: false, BrokenAtSeq: int64(n),
				Reason: "malformed line"}
		}
		gotHash, gotPrev, seqStr, canonical := parts[0], parts[1], parts[2], parts[3]
		seq, err := strconv.ParseUint(seqStr, 10, 64)
		if err != nil || seq != n {
			return AuditResult{Records: n, OK: false, BrokenAtSeq: int64(n),
				Reason: "seq not contiguous (deletion/reorder)"}
		}
		if gotPrev != prev {
			return AuditResult{Records: n, OK: false, BrokenAtSeq: int64(seq),
				Reason: "prev hash mismatch (chain break)"}
		}
		h := sha256.New()
		h.Write([]byte(prev))
		h.Write([]byte(canonical))
		want := hex.EncodeToString(h.Sum(nil))
		if want != gotHash {
			return AuditResult{Records: n, OK: false, BrokenAtSeq: int64(seq),
				Reason: "hash mismatch (record tampered)"}
		}
		prev = gotHash
		n++
	}
	if err := sc.Err(); err != nil {
		return AuditResult{Records: n, OK: false, BrokenAtSeq: int64(n),
			Reason: "read error: " + err.Error()}
	}
	return AuditResult{Records: n, OK: true, BrokenAtSeq: -1}
}
