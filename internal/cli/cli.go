package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/amasotti/steno/internal/config"
	"github.com/amasotti/steno/internal/filter"
	"github.com/amasotti/steno/internal/inspect"
	"github.com/amasotti/steno/internal/payload"
	cdprecorder "github.com/amasotti/steno/internal/recorder/cdp"
	"github.com/amasotti/steno/internal/render"
	"github.com/amasotti/steno/internal/summary"
	"github.com/amasotti/steno/internal/timeline"
)

type eventRecorder interface {
	Record(context.Context) (<-chan timeline.Event, <-chan error)
}

var newRecorder = func(rawURL string, f *filter.Filter, payloads *payload.Store, includeDocuments, includeHeaders bool) eventRecorder {
	return cdprecorder.New(rawURL, f, payloads, includeDocuments, includeHeaders)
}

func Run(ctx context.Context, args []string, stdout io.Writer, _ io.Writer) error {
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
	case "render":
		return runRender(args[1:], stdout)
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
  steno render ./runs/<name>
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
			if _, err := fmt.Fprintf(stdout, "%04d  %d  %s  %s%s\n", event.Seq, event.Status, event.Method, event.Origin, event.Path); err != nil {
				return err
			}
		case timeline.KindNavigation:
			if _, err := fmt.Fprintf(stdout, "%04d  nav  %s%s\n", event.Seq, event.Origin, event.Path); err != nil {
				return err
			}
		}
	}
	return nil
}

func showInspectDetail(stdout io.Writer, runDir string, event timeline.Event) error {
	if event.Kind == timeline.KindNavigation {
		_, err := fmt.Fprintf(stdout, "seq %04d  nav  %s%s\n", event.Seq, event.Origin, event.Path)
		return err
	}

	if _, err := fmt.Fprintf(stdout, "seq %04d  %s  %d  %s%s\n", event.Seq, event.Method, event.Status, event.Origin, event.Path); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "duration %dms  size %dB\n", event.DurationMS, event.SizeBytes); err != nil {
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
		if _, err := fmt.Fprintf(stdout, "\n%s (%s):\n%s\n", s.label, s.ref, body); err != nil {
			return err
		}
	}
	return nil
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

	_, err = fmt.Fprintf(stdout, "Rendered: %s\n", flowPath)
	return err
}

func printSessionSummary(w io.Writer, result summary.Summary, timelinePath string) error {
	if _, err := fmt.Fprintln(w, "Session summary"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Calls: %d\n", result.TotalCalls); err != nil {
		return err
	}
	if len(result.Origins) == 0 {
		if _, err := fmt.Fprintln(w, "Origins: none"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(w, "Origins:"); err != nil {
			return err
		}
		for _, origin := range result.Origins {
			if _, err := fmt.Fprintf(w, "  - %s\n", origin); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(w, "Status: 2xx=%d 3xx=%d 4xx=%d 5xx=%d\n", result.Status2xx, result.Status3xx, result.Status4xx, result.Status5xx); err != nil {
		return err
	}
	if result.HasDurations {
		if _, err := fmt.Fprintf(w, "Duration: avg=%dms min=%dms max=%dms\n", result.AverageDurationMS, result.MinDurationMS, result.MaxDurationMS); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(w, "Duration: no requests captured"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "Timeline: %s\n", timelinePath)
	return err
}
