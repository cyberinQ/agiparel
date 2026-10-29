// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.11 — Durable relay for already gateway-signed WIP scan envelopes.
// NEVER signs or edits a gateway observation; server alone verifies its signature,
// enrollment, carrier binding and accepted-part genealogy. No custody/credit writes.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	scanPath         = "/api/v1/wip/production/scans"
	maxEnvelope      = 16 * 1024
	maxPending       = 5000
	receiptAuthority = "SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF"
)

var (
	codeRE  = regexp.MustCompile(`^[A-Za-z0-9_-]{2,64}$`)
	eventRE = regexp.MustCompile(`^[A-Za-z0-9_-]{12,80}$`)
	uuidRE  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	epcRE   = regexp.MustCompile(`^[0-9A-F]{24,96}$`)
)

type scanEvent struct {
	Version    string `json:"version"`
	TenantID   string `json:"tenant_id"`
	PlantID    string `json:"plant_id"`
	GatewayID  string `json:"gateway_id"`
	StationID  string `json:"station_id"`
	EventID    string `json:"event_id"`
	PartID     string `json:"part_id"`
	CarrierID  string `json:"carrier_id"`
	EPCHex     string `json:"epc_hex"`
	ObservedAt string `json:"observed_at"`
}
type envelope struct {
	Event        scanEvent `json:"event"`
	SignatureHex string    `json:"signature_hex"`
}
type receipt struct {
	EnvelopeSHA256 string    `json:"envelope_sha256"`
	ServerScanID   string    `json:"server_scan_id"`
	Replayed       bool      `json:"replayed"`
	AcknowledgedAt time.Time `json:"acknowledged_at"`
}

type state struct{ root string }

func (s state) sub(name string) string         { return filepath.Join(s.root, name) }
func (s state) file(folder, key string) string { return filepath.Join(s.sub(folder), key+".json") }
func validateEvent(e scanEvent) error {
	if e.Version != "WIP_GATEWAY_SCAN_V1" || !codeRE.MatchString(e.TenantID) || !codeRE.MatchString(e.PlantID) ||
		!codeRE.MatchString(e.GatewayID) || !codeRE.MatchString(e.StationID) || !eventRE.MatchString(e.EventID) ||
		!uuidRE.MatchString(e.PartID) || !uuidRE.MatchString(e.CarrierID) || len(e.EPCHex)%2 != 0 || !epcRE.MatchString(e.EPCHex) {
		return errors.New("invalid signed scan fields (or noncanonical uppercase EPC required)")
	}
	if _, err := time.Parse(time.RFC3339Nano, e.ObservedAt); err != nil {
		return fmt.Errorf("invalid observed_at: %w", err)
	}
	return nil
}
func parseEnvelope(raw []byte) (envelope, []byte, error) {
	var e envelope
	if len(raw) == 0 || len(raw) > maxEnvelope {
		return e, nil, errors.New("envelope must be 1..16384 bytes")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		return e, nil, fmt.Errorf("invalid envelope: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return e, nil, errors.New("trailing JSON content")
	}
	if err := validateEvent(e.Event); err != nil {
		return e, nil, err
	}
	b, err := hex.DecodeString(e.SignatureHex)
	if err != nil || len(b) != ed25519.SignatureSize {
		return e, nil, errors.New("gateway signature must be exactly 64 signed bytes")
	}
	canonical, err := json.Marshal(e)
	return e, canonical, err
}
func identityKey(e scanEvent) string {
	h := sha256.Sum256([]byte(e.TenantID + "\x00" + e.GatewayID + "\x00" + e.EventID))
	return hex.EncodeToString(h[:])
}
func contentHash(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }

func secureDirectory(dir string) error {
	if !filepath.IsAbs(dir) {
		return errors.New("outbox directory must be an absolute local path")
	}
	fi, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		if err = os.Mkdir(dir, 0700); err != nil {
			return err
		}
		fi, err = os.Lstat(dir)
	}
	if err != nil {
		return err
	}
	if !fi.IsDir() || fi.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("directory %q must be a real private directory (0700)", dir)
	}
	return nil
}
func initState(root string) (state, error) {
	s := state{root: filepath.Clean(root)}
	if err := secureDirectory(s.root); err != nil {
		return s, err
	}
	for _, sub := range []string{"pending", "inflight", "accepted", "quarantine"} {
		if err := secureDirectory(s.sub(sub)); err != nil {
			return s, err
		}
	}
	return s, nil
}
func locked(root, name string, fn func() error) error {
	p := filepath.Join(root, "."+name+".lock")
	if fi, err := os.Lstat(p); err == nil && !fi.Mode().IsRegular() {
		return errors.New("refusing nonregular lock file")
	}
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if fstat, err := f.Stat(); err != nil || fstat.Mode().Perm()&0077 != 0 {
		return errors.New("lock file permissions must be private")
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}
func durableWrite(p string, raw []byte) error {
	dir := filepath.Dir(p)
	f, err := os.CreateTemp(dir, ".pending-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, p); err != nil {
		return err
	}
	return syncFolder(dir)
}
func syncFolder(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func durableMove(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	if err := syncFolder(filepath.Dir(dst)); err != nil {
		return err
	}
	return syncFolder(filepath.Dir(src))
}
func filesIn(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			if !e.Type().IsRegular() {
				return nil, errors.New("refusing nonregular outbox entry")
			}
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}
func (s state) enqueue(raw []byte) (string, bool, error) {
	e, canonical, err := parseEnvelope(raw)
	if err != nil {
		return "", false, err
	}
	key := identityKey(e.Event)
	returnKey := key
	replay := false
	err = locked(s.root, "state", func() error {
		for _, folder := range []string{"pending", "inflight", "quarantine", "accepted"} {
			p := s.file(folder, key)
			content, e := os.ReadFile(p)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return e
			}
			if folder == "accepted" {
				var r receipt
				if json.Unmarshal(content, &r) != nil || r.EnvelopeSHA256 != contentHash(canonical) {
					return errors.New("event ID previously used with different content")
				}
			} else {
				_, existing, e := parseEnvelope(content)
				if e != nil || !bytes.Equal(existing, canonical) {
					return errors.New("event ID previously used with different content")
				}
			}
			replay = true
			return nil
		}
		names, e := filesIn(s.sub("pending"))
		if e != nil {
			return e
		}
		active, e := filesIn(s.sub("inflight"))
		if e != nil {
			return e
		}
		if len(names)+len(active) >= maxPending {
			return errors.New("outbox full; no event accepted")
		}
		return durableWrite(s.file("pending", key), canonical)
	})
	return returnKey, replay, err
}

type postResult struct {
	id         string
	replay     bool
	retry      bool
	quarantine string
}

func checkEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != scanPath || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return fmt.Errorf("endpoint must be an HTTPS URL ending exactly in %s", scanPath)
	}
	return nil
}
func postScan(ctx context.Context, client *http.Client, endpoint string, canonical []byte) postResult {
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(canonical))
	if err != nil {
		return postResult{retry: true}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return postResult{retry: true}
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return postResult{retry: true}
	}
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		var ack struct {
			ID               string `json:"id"`
			Replayed         *bool  `json:"replayed"`
			Authority        string `json:"authority"`
			ProductionCredit *bool  `json:"production_credit"`
		}
		if json.Unmarshal(responseBody, &ack) != nil || !uuidRE.MatchString(ack.ID) || ack.Replayed == nil || ack.Authority != receiptAuthority || ack.ProductionCredit == nil || *ack.ProductionCredit {
			return postResult{retry: true} // Ambiguous acknowledgement: NEVER delete the scan.
		}
		return postResult{id: ack.ID, replay: *ack.Replayed}
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		return postResult{retry: true}
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return postResult{quarantine: fmt.Sprintf("SERVER_HTTP_%d", resp.StatusCode)}
	}
	return postResult{retry: true}
}

type flushResult struct{ delivered, retried, quarantined int }

func (s state) flush(ctx context.Context, client *http.Client, endpoint string, maxAge time.Duration, batch int, now func() time.Time) (flushResult, error) {
	var results flushResult
	if err := checkEndpoint(endpoint); err != nil {
		return results, err
	}
	if maxAge < time.Minute || maxAge > 15*time.Minute {
		return results, errors.New("max-age must be between 1m and 15m")
	}
	if batch < 1 || batch > 500 {
		return results, errors.New("batch must be between 1 and 500")
	}
	// One flusher across processes. Enqueue uses a DIFFERENT short-held lock.
	err := locked(s.root, "flush", func() error {
		err := locked(s.root, "state", func() error {
			// A previous process may have crashed after submission but before ACK.
			// Requeue and use server's immutable event-ID deduplication on retry.
			inflight, e := filesIn(s.sub("inflight"))
			if e != nil {
				return e
			}
			for _, n := range inflight {
				if e = durableMove(filepath.Join(s.sub("inflight"), n), filepath.Join(s.sub("pending"), n)); e != nil {
					return e
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		pending, err := filesIn(s.sub("pending"))
		if err != nil {
			return err
		}
		for i, name := range pending {
			if i >= batch {
				break
			}
			key := strings.TrimSuffix(name, ".json")
			var data []byte
			err = locked(s.root, "state", func() error {
				src := s.file("pending", key)
				if e := durableMove(src, s.file("inflight", key)); e != nil {
					return e
				}
				var e error
				data, e = os.ReadFile(s.file("inflight", key))
				return e
			})
			if err != nil {
				return err
			}
			env, canonical, e := parseEnvelope(data)
			reason := ""
			var outcome postResult
			if e != nil {
				reason = "MALFORMED_LOCAL_RECORD"
			} else {
				observed, _ := time.Parse(time.RFC3339Nano, env.Event.ObservedAt)
				current := now().UTC()
				switch {
				case observed.After(current.Add(2 * time.Minute)):
					reason = "FUTURE_OBSERVATION_CLOCK_SKEW"
				case current.Sub(observed) > maxAge:
					reason = "SCAN_EXPIRED_REQUIRES_EXCEPTION_REVIEW"
				default:
					outcome = postScan(ctx, client, endpoint, canonical)
					reason = outcome.quarantine
				}
			}
			if outcome.retry && reason == "" { // retain for a retry, preserve original signed bytes
				if e = locked(s.root, "state", func() error { return durableMove(s.file("inflight", key), s.file("pending", key)) }); e != nil {
					return e
				}
				results.retried++
				continue
			}
			if reason != "" {
				if e = locked(s.root, "state", func() error {
					if e := durableMove(s.file("inflight", key), s.file("quarantine", key)); e != nil {
						return e
					}
					return durableWrite(filepath.Join(s.sub("quarantine"), key+".reason"), []byte(reason+"\n"))
				}); e != nil {
					return e
				}
				results.quarantined++
				continue
			}
			if e = locked(s.root, "state", func() error {
				ack := receipt{EnvelopeSHA256: contentHash(canonical), ServerScanID: outcome.id, Replayed: outcome.replay, AcknowledgedAt: now().UTC()}
				b, e := json.Marshal(ack)
				if e != nil {
					return e
				}
				if e = durableWrite(s.file("accepted", key), b); e != nil {
					return e
				}
				if e = os.Remove(s.file("inflight", key)); e != nil {
					return e
				}
				return syncFolder(s.sub("inflight"))
			}); e != nil {
				return e
			}
			results.delivered++
		}
		return nil
	})
	return results, err
}
func (s state) status() (map[string]int, error) {
	counts := map[string]int{}
	err := locked(s.root, "state", func() error {
		for _, folder := range []string{"pending", "inflight", "accepted", "quarantine"} {
			names, err := filesIn(s.sub(folder))
			if err != nil {
				return err
			}
			counts[folder] = len(names)
		}
		return nil
	})
	return counts, err
}
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "scanrelay:", err)
		os.Exit(1)
	}
}
func run(args []string, out io.Writer) error {
	if len(args) < 1 {
		return errors.New("usage: scanrelay enqueue|flush|status [options]")
	}
	op := args[0]
	f := flag.NewFlagSet(op, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	dir := f.String("dir", "", "absolute private outbox directory (0700)")
	file := f.String("file", "", "already-signed gateway JSON envelope")
	endpoint := f.String("endpoint", "", "HTTPS production ingress URL")
	maxAge := f.Duration("max-age", 12*time.Minute, "local delivery deadline (maximum 15m)")
	batch := f.Int("batch", 50, "maximum deliveries per invocation")
	if op != "enqueue" && op != "flush" && op != "status" {
		return errors.New("unknown operation")
	}
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 || *dir == "" {
		return errors.New("--dir required and unexpected positional arguments rejected")
	}
	s, err := initState(*dir)
	if err != nil {
		return err
	}
	switch op {
	case "enqueue":
		if *file == "" {
			return errors.New("enqueue requires --file")
		}
		if *endpoint != "" {
			return errors.New("enqueue must not specify --endpoint")
		}
		b, err := os.ReadFile(*file)
		if err != nil {
			return err
		}
		key, replay, err := s.enqueue(b)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "event_key=%s already_queued=%t server_accepted=false\n", key, replay)
	case "flush":
		if *endpoint == "" {
			return errors.New("flush requires --endpoint")
		}
		client := &http.Client{Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		res, err := s.flush(context.Background(), client, *endpoint, *maxAge, *batch, time.Now)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "delivered=%d retry_pending=%d quarantined=%d no_custody_credit=true\n", res.delivered, res.retried, res.quarantined)
		if res.retried > 0 || res.quarantined > 0 {
			return errors.New("delivery incomplete; check status and review quarantine; no synthetic WIP")
		}
	case "status":
		counts, err := s.status()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "pending=%d inflight=%d accepted=%d quarantine=%d\n", counts["pending"], counts["inflight"], counts["accepted"], counts["quarantine"])
	}
	return nil
}
