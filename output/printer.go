// Package output prints results for people (a table) or for scripts (-o json), and turns the API's error
// codes into sentences.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

const (
	FormatTable = "table"
	FormatJson  = "json"
)

// Printer writes results to Out and messages meant for the person at the terminal (prompts, warnings) to
// Err, so `-o json` output stays parseable.
type Printer struct {
	Format string
	Out    io.Writer
	Err    io.Writer
}

// Field is one row of a single object printed as a table.
type Field struct {
	Name  string
	Value string
}

func ValidFormat(format string) error {
	if format == FormatTable || format == FormatJson {
		return nil
	}
	return fmt.Errorf("unknown output format %q: use %q or %q", format, FormatTable, FormatJson)
}

// Object prints one object: as "Name: value" rows, or as value encoded to JSON.
func (p *Printer) Object(fields []Field, value any) error {
	if p.Format == FormatJson {
		encoder := json.NewEncoder(p.Out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	writer := tabwriter.NewWriter(p.Out, 0, 0, 2, ' ', 0)
	for _, field := range fields {
		fmt.Fprintf(writer, "%s:\t%s\n", field.Name, field.Value)
	}
	return writer.Flush()
}

// Table prints a list: a header row and one row per item, or value encoded to JSON.
func (p *Printer) Table(headers []string, rows [][]string, value any) error {
	if p.Format == FormatJson {
		encoder := json.NewEncoder(p.Out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	writer := tabwriter.NewWriter(p.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(writer, strings.Join(row, "\t"))
	}
	return writer.Flush()
}

// Message is a line for the person at the terminal.
func (p *Printer) Message(format string, args ...any) {
	fmt.Fprintf(p.Err, format+"\n", args...)
}
