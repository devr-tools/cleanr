package trends

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"time"
)

func renderHTML(analysis Analysis) ([]byte, error) {
	tmpl, err := template.New("trend-analysis").Funcs(template.FuncMap{
		"dur":   formatTrendDuration,
		"pct":   formatTrendPercent,
		"json":  formatTrendJSON,
		"delta": formatTrendDelta,
	}).Parse(trendHTMLTemplate)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, analysis); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func formatTrendDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	return d.String()
}

func formatTrendPercent(v float64) string {
	return fmt.Sprintf("%.1f%%", v*100)
}

func formatTrendJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}

func formatTrendDelta(v int) string {
	if v > 0 {
		return fmt.Sprintf("+%d", v)
	}
	return fmt.Sprintf("%d", v)
}

const trendHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>cleanr trends: {{.Target}}</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #0f1720;
      --bg-2: #172535;
      --panel: #1f2d3d;
      --panel-2: #243b53;
      --ink: #f7fafc;
      --muted: #a0aec0;
      --line: #4a5568;
      --accent: #f6e05e;
    }
    body {
      margin: 0;
      font-family: "Avenir Next", "Trebuchet MS", sans-serif;
      background:
        radial-gradient(circle at top left, rgba(246, 224, 94, 0.12), transparent 26rem),
        radial-gradient(circle at top right, rgba(43, 108, 176, 0.16), transparent 24rem),
        linear-gradient(180deg, var(--bg-2), var(--bg));
      color: var(--ink);
    }
    main { max-width: 1080px; margin: 0 auto; padding: 2rem 1.25rem 4rem; }
    .hero, .panel {
      background: linear-gradient(180deg, rgba(36, 59, 83, 0.94), rgba(31, 45, 61, 0.94));
      border: 1px solid var(--line);
      border-radius: 22px;
      box-shadow: 0 20px 50px rgba(0, 0, 0, 0.22);
    }
    .hero { padding: 1.5rem; }
    .eyebrow {
      display: inline-block;
      margin-bottom: 0.65rem;
      color: var(--accent);
      font: 700 0.8rem/1.1 "SFMono-Regular", Consolas, monospace;
      letter-spacing: 0.18em;
      text-transform: uppercase;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
      gap: 0.9rem;
      margin-top: 1rem;
    }
    .metric {
      border: 1px solid var(--line);
      border-radius: 16px;
      padding: 0.9rem;
      background: rgba(15, 23, 32, 0.28);
    }
    .metric strong { display: block; font-size: 1.6rem; margin-top: 0.35rem; }
    .panels {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
      gap: 1rem;
      margin-top: 1rem;
    }
    .panel { padding: 1.1rem 1.15rem; }
    table { width: 100%; border-collapse: collapse; }
    th, td {
      text-align: left;
      padding: 0.55rem 0;
      border-bottom: 1px solid rgba(23, 37, 84, 0.12);
      vertical-align: top;
    }
    th {
      font-size: 0.78rem;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      color: var(--accent);
    }
    pre {
      white-space: pre-wrap;
      word-break: break-word;
      background: #0b1118;
      color: #f7fafc;
      padding: 0.8rem;
      border-radius: 14px;
      overflow-x: auto;
    }
    ul { margin: 0; padding-left: 1.1rem; }
  </style>
</head>
<body>
<main>
  <section class="hero">
    <span class="eyebrow">devr-tools / cleanr</span>
    <h1>Trend Dashboard: {{.Target}}</h1>
    <p>Static cleanr trend dashboard. Window {{.WindowSize}} of {{.TotalRetainedRuns}} retained runs. Latest build {{if .Latest.BuildID}}{{.Latest.BuildID}}{{else}}n/a{{end}}.</p>
    <div class="grid">
      <div class="metric">Pass Rate<strong>{{pct .PassRate}}</strong></div>
      <div class="metric">Failed Runs<strong>{{.FailedRuns}}</strong></div>
      <div class="metric">Avg Duration<strong>{{dur .AverageDuration}}</strong></div>
      <div class="metric">Latest Failed Cases<strong>{{.Latest.FailedCases}}</strong></div>
    </div>
  </section>

  <section class="panels">
    {{if .Delta}}
    <article class="panel">
      <h2>Delta</h2>
      <table>
        <tr><th>Failed Suites Δ</th><td>{{delta .Delta.FailedSuitesDelta}}</td></tr>
        <tr><th>Failed Cases Δ</th><td>{{delta .Delta.FailedCasesDelta}}</td></tr>
        <tr><th>Duration Δ</th><td>{{dur .Delta.DurationDelta}}</td></tr>
        <tr><th>Regressed Suites</th><td>{{.Delta.RegressedSuites}}</td></tr>
        <tr><th>Improved Suites</th><td>{{.Delta.ImprovedSuites}}</td></tr>
      </table>
    </article>
    {{end}}

    {{if .Drift}}
    <article class="panel">
      <h2>Drift Window</h2>
      <table>
        <tr><th>Average Semantic Drift</th><td>{{printf "%.3f" .Drift.AverageSemanticDrift}}</td></tr>
        <tr><th>Max Semantic Drift</th><td>{{printf "%.3f" .Drift.MaxSemanticDrift}}</td></tr>
        <tr><th>Latest Semantic Drift</th><td>{{printf "%.3f" .Drift.LatestSemanticDrift}}</td></tr>
        <tr><th>Latest Baseline Semantic Drift</th><td>{{printf "%.3f" .Drift.LatestBaselineSemanticDrift}}</td></tr>
      </table>
    </article>
    {{end}}

    {{if .Load}}
    <article class="panel">
      <h2>Load Window</h2>
      <table>
        <tr><th>Average Error Rate</th><td>{{printf "%.1f%%" .Load.AverageErrorRatePct}}</td></tr>
        <tr><th>Average P95</th><td>{{printf "%.1f ms" .Load.AverageP95LatencyMS}}</td></tr>
        <tr><th>Average Throughput</th><td>{{printf "%.3f rps" .Load.AverageThroughputRPS}}</td></tr>
        <tr><th>Latest Throughput</th><td>{{printf "%.3f rps" .Load.LatestThroughputRPS}}</td></tr>
      </table>
    </article>
    {{end}}
  </section>

  {{if .RecentRuns}}
  <section class="panel" style="margin-top:1rem;">
    <h2>Recent Runs</h2>
    <table>
      <thead>
        <tr><th>Build</th><th>Generated</th><th>Status</th><th>Failed Cases</th><th>Duration</th></tr>
      </thead>
      <tbody>
        {{range .RecentRuns}}
        <tr>
          <td>{{if .BuildID}}{{.BuildID}}{{else}}n/a{{end}}</td>
          <td>{{.GeneratedAt.Format "2006-01-02 15:04:05 MST"}}</td>
          <td>{{if .Passed}}pass{{else}}fail{{end}}</td>
          <td>{{.FailedCases}}</td>
          <td>{{dur .Duration}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
  </section>
  {{end}}

  {{if .FailureBuckets}}
  <section class="panel" style="margin-top:1rem;">
    <h2>Failure Buckets</h2>
    <ul>
      {{range .FailureBuckets}}
      <li><strong>{{.Signature}}</strong> ({{.Count}})</li>
      {{end}}
    </ul>
  </section>
  {{end}}

  {{if .BuildDiff}}
  <section class="panel" style="margin-top:1rem;">
    <h2>Build Diff</h2>
    <pre>{{json .BuildDiff}}</pre>
  </section>
  {{end}}
</main>
</body>
</html>`
