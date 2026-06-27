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
	"github.com/amasotti/steno/internal/timeline"
)

func Run(_ context.Context, args []string, stdout io.Writer, _ io.Writer) error {
	if len(args) == 0 {
		_, err := fmt.Fprint(stdout, usage())
		return err
	}

	switch args[0] {
	case "record":
		_, err := parseRecord(args[1:])
		return err
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
