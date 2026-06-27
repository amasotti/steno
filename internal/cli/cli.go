package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/amasotti/steno/internal/inspect"
	cdprecorder "github.com/amasotti/steno/internal/recorder/cdp"
	"github.com/amasotti/steno/internal/summary"
	"github.com/amasotti/steno/internal/timeline"
)

type eventRecorder interface {
	Record(context.Context) (<-chan timeline.Event, <-chan error)
}

var newRecorder = func(rawURL string) eventRecorder {
	return cdprecorder.New(rawURL)
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
		_, err := fmt.Fprintln(stdout, "render: not implemented yet")
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
  steno record --out ./runs/<name> [--url https://example.com]
  steno inspect ./runs/<name>
  steno render ./runs/<name>
`
}

type RecordOptions struct {
	URL string
	Out string
}

func parseRecord(args []string) (RecordOptions, error) {
	flags := flag.NewFlagSet("record", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var opts RecordOptions
	flags.StringVar(&opts.URL, "url", "", "URL to record")
	flags.StringVar(&opts.Out, "out", "", "Output directory for the recording")

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
	if err := os.MkdirAll(opts.Out, 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	file, err := os.Create(filepath.Join(opts.Out, "timeline.jsonl"))
	if err != nil {
		return fmt.Errorf("create timeline: %w", err)
	}
	defer func() { _ = file.Close() }()

	events, errs := newRecorder(opts.URL).Record(ctx)
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

func runInspect(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing run directory")
	}
	if len(args) > 1 {
		return fmt.Errorf("unexpected positional argument %q", args[1])
	}

	file, err := os.Open(filepath.Join(args[0], "timeline.jsonl"))
	if err != nil {
		return fmt.Errorf("open timeline: %w", err)
	}
	defer func() { _ = file.Close() }()

	events, err := inspect.ReadTimeline(file)
	if err != nil {
		return err
	}

	for _, event := range events {
		if event.Kind != timeline.KindCall {
			continue
		}
		if _, err := fmt.Fprintf(stdout, "%04d  %d  %s  %s%s\n", event.Seq, event.Status, event.Method, event.Origin, event.Path); err != nil {
			return err
		}
	}
	return nil
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
