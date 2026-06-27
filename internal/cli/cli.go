package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/amasotti/steno/internal/config"
	"github.com/amasotti/steno/internal/filter"
	"github.com/amasotti/steno/internal/inspect"
	"github.com/amasotti/steno/internal/payload"
	cdprecorder "github.com/amasotti/steno/internal/recorder/cdp"
	"github.com/amasotti/steno/internal/render"
	"github.com/amasotti/steno/internal/replay"
	"github.com/amasotti/steno/internal/summary"
	"github.com/amasotti/steno/internal/timeline"
)

// Version is the build version, injected at release time via
// -ldflags "-X github.com/amasotti/steno/internal/cli.Version=<tag>".
// It stays "dev" for local and CI builds.
var Version = "dev"

type eventRecorder interface {
	Record(context.Context) (<-chan timeline.Event, <-chan error)
}

var newRecorder = func(rawURL string, f *filter.Filter, payloads *payload.Store, includeDocuments, includeHeaders bool) eventRecorder {
	return cdprecorder.New(rawURL, f, payloads, includeDocuments, includeHeaders)
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		_, err := fmt.Fprint(stdout, usage())
		return err
	}

	switch args[0] {
	case "record":
		opts, err := parseRecord(args[1:])
		if err != nil {
			return err
		}
		return runRecord(ctx, opts, stdout)
	case "inspect":
		return runInspect(args[1:], stdout)
	case "replay":
		return runReplay(args[1:], stdout, stderr)
	case "render":
		return runRender(args[1:], stdout)
	case "version", "-v", "--version":
		_, err := fmt.Fprintf(stdout, "steno %s\n", Version)
		return err
	case "help", "-h", "--help":
		_, err := fmt.Fprint(stdout, usage())
		return err
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() string {
	return `Usage:
  steno record --out ./runs/<name> [--url https://example.com] [--config flow.toml]
  steno inspect ./runs/<name> [--origin <substr>] [--status 4xx,5xx] [--seq <n>]
  steno replay ./runs/<name> <seq>
  steno render ./runs/<name>
  steno version
`
}

type RecordOptions struct {
	URL    string
	Out    string
	Config string
}

func parseRecord(args []string) (RecordOptions, error) {
	flags := flag.NewFlagSet("record", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var opts RecordOptions
	flags.StringVar(&opts.URL, "url", "", "URL to record")
	flags.StringVar(&opts.Out, "out", "", "Output directory for the recording")
	flags.StringVar(&opts.Config, "config", "", "Path to a TOML config file (optional)")

	if err := flags.Parse(args); err != nil {
		return RecordOptions{}, err
	}

	if opts.Out == "" {
		return RecordOptions{}, errors.New("missing --out")
	}

	if flags.NArg() != 0 {
		return RecordOptions{}, errors.New("positional arguments not supported")
	}

	return opts, nil
}

func runRecord(ctx context.Context, opts RecordOptions, stdout io.Writer) error {
	cfg := config.Default()
	if opts.Config != "" {
		loaded, err := config.Load(opts.Config)
		if err != nil {
			return err
		}
		cfg = loaded
	}

	// An explicit --url always overrides the config's start_url.
	startURL := cfg.Target.StartURL
	if opts.URL != "" {
		startURL = opts.URL
	}

	if err := os.MkdirAll(opts.Out, 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	file, err := os.Create(filepath.Join(opts.Out, "timeline.jsonl"))
	if err != nil {
		return fmt.Errorf("create timeline: %w", err)
	}
	defer func() { _ = file.Close() }()

	events, errs := newRecorder(startURL, filter.New(cfg.Filter), payload.NewStore(opts.Out), cfg.Payload.IncludeDocuments, cfg.Payload.IncludeHeaders).Record(ctx)
	var recorded []timeline.Event
	timelinePath := filepath.Join(opts.Out, "timeline.jsonl")
	for event := range events {
		if err := timeline.WriteJSONL(file, event); err != nil {
			return fmt.Errorf("write timeline event: %w", err)
		}
		recorded = append(recorded, event)
	}

	if err, ok := <-errs; ok && err != nil {
		return err
	}

	return printSessionSummary(stdout, summary.FromEvents(recorded), timelinePath)
}

// resolveRun accepts either a run directory or a path to its timeline.jsonl
// file and returns the run directory plus the timeline path. Side-files
// (payloads/, flow.mmd) live in the run directory, so both are always needed.
func resolveRun(path string) (runDir, timelinePath string) {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return filepath.Dir(path), path
	}
	return path, filepath.Join(path, "timeline.jsonl")
}

func runInspect(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing run directory")
	}
	runDir := args[0]
	if strings.HasPrefix(runDir, "-") {
		return errors.New("missing run directory")
	}

	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	origin := flags.String("origin", "", "keep only events whose origin contains this substring")
	statusList := flags.String("status", "", "keep only calls with these statuses (e.g. 4xx,5xx,404)")
	seq := flags.Int("seq", -1, "show full detail (headers + payloads) for one event by sequence id")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional argument %q", flags.Arg(0))
	}

	statuses, err := inspect.ParseStatuses(*statusList)
	if err != nil {
		return err
	}

	runDir, timelinePath := resolveRun(runDir)
	file, err := os.Open(timelinePath)
	if err != nil {
		return fmt.Errorf("open timeline: %w", err)
	}
	defer func() { _ = file.Close() }()

	events, err := inspect.ReadTimeline(file)
	if err != nil {
		return err
	}

	if *seq >= 0 {
		event, ok := inspect.Find(events, *seq)
		if !ok {
			return fmt.Errorf("no event with seq %d", *seq)
		}
		return showInspectDetail(stdout, runDir, event)
	}

	events = inspect.Filter{Origin: *origin, Statuses: statuses}.Apply(events)
	for _, event := range events {
		switch event.Kind {
		case timeline.KindCall:
			if _, err := fmt.Fprintf(stdout, "%s  %s  %s  %s%s\n",
				stylePath(fmt.Sprintf("%04d", event.Seq)),
				styleStatus(event.Status, fmt.Sprintf("%d", event.Status)),
				styleLabel(event.Method),
				event.Origin, event.Path); err != nil {
				return err
			}
		case timeline.KindNavigation:
			if _, err := fmt.Fprintf(stdout, "%s  %s  %s%s\n",
				stylePath(fmt.Sprintf("%04d", event.Seq)),
				styleLabel("nav"),
				event.Origin, event.Path); err != nil {
				return err
			}
		}
	}
	return nil
}

func showInspectDetail(stdout io.Writer, runDir string, event timeline.Event) error {
	if event.Kind == timeline.KindNavigation {
		_, err := fmt.Fprintf(stdout, "%s %s  %s  %s%s\n", styleLabel("seq"), stylePath(fmt.Sprintf("%04d", event.Seq)), styleLabel("nav"), event.Origin, event.Path)
		return err
	}

	if _, err := fmt.Fprintf(stdout, "%s %s  %s  %s  %s%s\n",
		styleLabel("seq"),
		stylePath(fmt.Sprintf("%04d", event.Seq)),
		styleLabel(event.Method),
		styleStatus(event.Status, fmt.Sprintf("%d", event.Status)),
		event.Origin, event.Path); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "%s %dms  %s %dB\n", styleLabel("duration"), event.DurationMS, styleLabel("size"), event.SizeBytes); err != nil {
		return err
	}

	sections := []struct {
		label string
		ref   string
	}{
		{"request headers", event.RequestHeadersRef},
		{"request body", event.RequestRef},
		{"response headers", event.ResponseHeadersRef},
		{"response body", event.ResponseRef},
	}
	for _, s := range sections {
		if s.ref == "" {
			continue
		}
		body, err := inspect.ReadArtifact(runDir, s.ref)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "\n%s %s\n%s\n", styleHeader(s.label), stylePath("("+s.ref+"):"), body); err != nil {
			return err
		}
	}
	return nil
}

func runReplay(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing run directory")
	}
	if len(args) < 2 {
		return errors.New("missing seq; usage: steno replay <run> <seq>")
	}
	if len(args) > 2 {
		return fmt.Errorf("unexpected positional argument %q", args[2])
	}
	seq, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid seq %q", args[1])
	}

	runDir, timelinePath := resolveRun(args[0])
	file, err := os.Open(timelinePath)
	if err != nil {
		return fmt.Errorf("open timeline: %w", err)
	}
	defer func() { _ = file.Close() }()

	events, err := inspect.ReadTimeline(file)
	if err != nil {
		return err
	}

	event, ok := inspect.Find(events, seq)
	if !ok {
		return fmt.Errorf("no event with seq %d", seq)
	}
	if event.Kind != timeline.KindCall {
		return fmt.Errorf("seq %d is a %s, not a call; only calls can be replayed", seq, event.Kind)
	}

	var headers map[string]string
	if event.RequestHeadersRef != "" {
		raw, err := inspect.ReadArtifact(runDir, event.RequestHeadersRef)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &headers); err != nil {
			return fmt.Errorf("parse request headers %q: %w", event.RequestHeadersRef, err)
		}
	}

	var body []byte
	if event.RequestRef != "" {
		body, err = inspect.ReadArtifact(runDir, event.RequestRef)
		if err != nil {
			return err
		}
	}

	cmd, warnings := replay.Curl(event, headers, body)
	for _, w := range warnings {
		if _, err := fmt.Fprintf(stderr, "steno replay: %s\n", w); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(stdout, cmd)
	return err
}

func runRender(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing run directory")
	}
	if len(args) > 1 {
		return fmt.Errorf("unexpected positional argument %q", args[1])
	}

	runDir, timelinePath := resolveRun(args[0])
	timelineFile, err := os.Open(timelinePath)
	if err != nil {
		return fmt.Errorf("open timeline: %w", err)
	}
	defer func() { _ = timelineFile.Close() }()

	events, err := inspect.ReadTimeline(timelineFile)
	if err != nil {
		return err
	}

	flowPath := filepath.Join(runDir, "flow.mmd")
	flowFile, err := os.Create(flowPath)
	if err != nil {
		return fmt.Errorf("create flow diagram: %w", err)
	}
	defer func() { _ = flowFile.Close() }()

	if err = render.Mermaid(flowFile, events); err != nil {
		return fmt.Errorf("render flow diagram: %w", err)
	}

	_, err = fmt.Fprintf(stdout, "%s %s\n", styleLabel("Rendered:"), stylePath(flowPath))
	return err
}

func printSessionSummary(w io.Writer, result summary.Summary, timelinePath string) error {
	if _, err := fmt.Fprintln(w, styleHeader("Session summary")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s %d\n", styleLabel("Calls:"), result.TotalCalls); err != nil {
		return err
	}
	if len(result.Origins) == 0 {
		if _, err := fmt.Fprintf(w, "%s none\n", styleLabel("Origins:")); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(w, styleLabel("Origins:")); err != nil {
			return err
		}
		for _, origin := range result.Origins {
			if _, err := fmt.Fprintf(w, "  - %s\n", stylePath(origin)); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(w, "%s %s %s %s %s\n",
		styleLabel("Status:"),
		styleStatus(200, fmt.Sprintf("2xx=%d", result.Status2xx)),
		styleStatus(300, fmt.Sprintf("3xx=%d", result.Status3xx)),
		styleStatus(400, fmt.Sprintf("4xx=%d", result.Status4xx)),
		styleStatus(500, fmt.Sprintf("5xx=%d", result.Status5xx)),
	); err != nil {
		return err
	}
	if result.HasDurations {
		if _, err := fmt.Fprintf(w, "%s avg=%dms min=%dms max=%dms\n", styleLabel("Duration:"), result.AverageDurationMS, result.MinDurationMS, result.MaxDurationMS); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(w, "%s no requests captured\n", styleLabel("Duration:")); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s %s\n", styleLabel("Timeline:"), stylePath(timelinePath))
	return err
}
