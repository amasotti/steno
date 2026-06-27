package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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
		_, err := fmt.Fprintln(stdout, "inspect: not implemented yet")
		return err
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
  steno record --config flow.toml --out ./runs/<name> [--url https://example.com]
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
