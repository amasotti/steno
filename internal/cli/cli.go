package cli

import (
	"context"
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
		_, err := fmt.Fprintln(stdout, "record: not implemented yet")
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
  steno record --config flow.toml --out ./runs/<name>
  steno inspect ./runs/<name>
  steno render ./runs/<name>
`
}
