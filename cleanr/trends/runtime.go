package trends

import (
	"errors"
	"os"
	"strings"

	"cleanr/cleanr/core"
)

func AttachAndPersist(report *core.Report, path, buildID string, limit int) error {
	if report == nil {
		return errors.New("nil report")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	history, err := loadOrInit(path, report.Name)
	if err != nil {
		return err
	}
	previous := LatestRun(history)
	current := BuildRun(*report, buildID)
	historyLength := len(history.Runs) + 1
	if limit > 0 && historyLength > limit {
		historyLength = limit
	}
	report.Trend = Compare(current, previous, historyLength)
	updated := AppendRun(history, current, limit)
	return WriteFile(path, updated)
}

func loadOrInit(path, target string) (HistoryFile, error) {
	history, err := LoadFile(path)
	if err == nil {
		if history.Version == "" {
			history.Version = "v1alpha1"
		}
		if history.Target == "" {
			history.Target = target
		}
		return history, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return NewHistory(target), nil
	}
	return HistoryFile{}, err
}
