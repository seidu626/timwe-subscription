// subscription-processor submits explicit MSISDN feeds to the existing batch API.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runCLI(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "subscription-processor:", err)
		os.Exit(1)
	}
}

func runCLI(ctx context.Context, args []string, stdin io.Reader, output io.Writer) error {
	flags := flag.NewFlagSet("subscription-processor", flag.ContinueOnError)
	flags.SetOutput(output)
	configPath := flags.String("config", "config.json", "JSON configuration path")
	source := flags.String("source", "", "MSISDN file, HTTP(S) URL, database, or - for stdin (overrides config)")
	format := flags.String("format", "", "auto, text, csv, or json (overrides config)")
	dryRun := flags.Bool("dry-run", false, "validate input and report counts without calling the subscription API or writing progress")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	c, err := loadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if *source != "" {
		c.Source = *source
	}
	if *format != "" {
		c.Format = *format
	}
	if err := c.validate(); err != nil {
		return err
	}
	secret := strings.TrimSpace(os.Getenv("INTERNAL_API_SECRET"))
	if !*dryRun && secret == "" {
		return fmt.Errorf("INTERNAL_API_SECRET is required")
	}
	// A redirect must not forward credentials or transform a subscription POST into a GET.
	client := &http.Client{Timeout: c.timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	if !*dryRun {
		unlock, err := lockCheckpoint(c.StateFile)
		if err != nil {
			return err
		}
		defer unlock()
	}
	numbers, duplicates, err := readSource(ctx, c, client, stdin)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "Source validated: unique=%d duplicates_removed=%d batches=%d\n", len(numbers), duplicates, 1+(len(numbers)-1)/c.BatchSize)
	if *dryRun {
		return nil
	}
	state, err := loadCheckpoint(c, numbers)
	if err != nil {
		return err
	}
	p := processor{config: c, client: client, secret: secret, output: output}
	return p.run(ctx, numbers, state)
}
