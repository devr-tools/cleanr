package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"cleanr/cleanr/core"
)

func Write(w io.Writer, report core.Report, format string) error {
	switch strings.ToLower(format) {
	case "", "text":
		_, err := fmt.Fprint(w, renderText(report, textPaletteForWriter(w)))
		return err
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	case "junit":
		return writeJUnit(w, report)
	default:
		return fmt.Errorf("unsupported report format: %s", format)
	}
}

func Text(report core.Report) string {
	return renderText(report, plainTextPalette())
}
