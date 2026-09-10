package testsuite

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"time"
)

const (
	FormatText  = "text"
	FormatJSON  = "json"
	FormatJUnit = "junit"
)

// WriteReport emits a deterministic human- or CI-readable report.
func WriteReport(writer io.Writer, format string, report Report) error {
	switch format {
	case FormatText:
		for _, result := range report.Results {
			if _, err := fmt.Fprintf(writer, "%s %s (%s)", result.Status, result.Case.ID, result.Duration.Round(time.Millisecond)); err != nil {
				return err
			}
			if result.Error != "" {
				if _, err := fmt.Fprintf(writer, ": %s", result.Error); err != nil {
					return err
				}
			}
			for _, frame := range result.Stack {
				if _, err := fmt.Fprintf(writer, "\n  at %s.%s (%s:%d:%d)", frame.Module, frame.Routine, frame.Path, frame.Line, frame.Column); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(writer, "tests: %d passed, %d failed; coverage: %.1f%% (%d/%d lines)\n",
			report.Passed, report.Failed, report.Coverage.Percent, report.Coverage.Covered, report.Coverage.Total)
		return err
	case FormatJSON:
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	case FormatJUnit:
		return writeJUnit(writer, report)
	default:
		return fmt.Errorf("unsupported test report format %q", format)
	}
}

type junitSuite struct {
	XMLName  xml.Name    `xml:"testsuite"`
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Time     string      `xml:"time,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

func writeJUnit(writer io.Writer, report Report) error {
	suite := junitSuite{Name: "MetaLab BSL", Tests: len(report.Results), Failures: report.Failed, Time: seconds(report.Duration)}
	for _, result := range report.Results {
		item := junitCase{Name: result.Case.Routine, Classname: result.Case.Module, Time: seconds(result.Duration)}
		if result.Status == Failed {
			item.Failure = &junitFailure{Message: result.Error, Text: failureText(result)}
		}
		suite.Cases = append(suite.Cases, item)
	}
	if _, err := io.WriteString(writer, xml.Header); err != nil {
		return err
	}
	encoder := xml.NewEncoder(writer)
	encoder.Indent("", "  ")
	if err := encoder.Encode(suite); err != nil {
		return err
	}
	_, err := io.WriteString(writer, "\n")
	return err
}

func failureText(result Result) string {
	text := result.Error
	for _, frame := range result.Stack {
		text += fmt.Sprintf("\n  at %s.%s (%s:%d:%d)", frame.Module, frame.Routine, frame.Path, frame.Line, frame.Column)
	}
	return text
}

func seconds(value time.Duration) string {
	return fmt.Sprintf("%.6f", value.Seconds())
}
