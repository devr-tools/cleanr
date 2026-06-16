package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
	imgpkg "github.com/devr-tools/cleanr/img"
)

type htmlReportView struct {
	Report    core.Report
	LogoASCII string
}

func renderHTMLReport(report core.Report) ([]byte, error) {
	view := htmlReportView{
		Report:    report,
		LogoASCII: imgpkg.Banner(),
	}
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"dur":      formatHTMLDuration,
		"detail":   formatHTMLDetail,
		"severity": htmlSeverityClass,
		"delta":    formatSignedInt,
	}).Parse(reportHTMLTemplate)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, view); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func formatHTMLDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	return d.String()
}

func formatHTMLJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}

func formatHTMLDetail(v any) template.HTML {
	return template.HTML(renderHTMLValue(v, 0))
}

func renderHTMLValue(v any, depth int) string {
	switch value := v.(type) {
	case nil:
		return `<span class="detail-empty">none</span>`
	case map[string]any:
		if len(value) == 0 {
			return `<span class="detail-empty">empty object</span>`
		}
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteString(`<div class="detail-grid">`)
		for _, key := range keys {
			b.WriteString(`<div class="detail-row"><div class="detail-key">`)
			b.WriteString(template.HTMLEscapeString(key))
			b.WriteString(`</div><div class="detail-value">`)
			b.WriteString(renderHTMLValue(value[key], depth+1))
			b.WriteString(`</div></div>`)
		}
		b.WriteString(`</div>`)
		return b.String()
	case []any:
		if len(value) == 0 {
			return `<span class="detail-empty">empty list</span>`
		}
		var b strings.Builder
		b.WriteString(`<ul class="detail-list">`)
		for _, item := range value {
			b.WriteString(`<li>`)
			b.WriteString(renderHTMLValue(item, depth+1))
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ul>`)
		return b.String()
	case []string:
		if len(value) == 0 {
			return `<span class="detail-empty">empty list</span>`
		}
		var b strings.Builder
		b.WriteString(`<ul class="detail-list">`)
		for _, item := range value {
			b.WriteString(`<li><span class="detail-chip">`)
			b.WriteString(template.HTMLEscapeString(item))
			b.WriteString(`</span></li>`)
		}
		b.WriteString(`</ul>`)
		return b.String()
	case string:
		if value == "" {
			return `<span class="detail-empty">empty</span>`
		}
		return `<span class="detail-text">` + template.HTMLEscapeString(value) + `</span>`
	case bool:
		if value {
			return `<span class="detail-chip detail-chip-true">true</span>`
		}
		return `<span class="detail-chip detail-chip-false">false</span>`
	case time.Duration:
		return `<span class="detail-chip">` + template.HTMLEscapeString(value.String()) + `</span>`
	case fmt.Stringer:
		return `<span class="detail-text">` + template.HTMLEscapeString(value.String()) + `</span>`
	default:
		if depth > 4 {
			return `<span class="detail-text">` + template.HTMLEscapeString(formatHTMLJSON(v)) + `</span>`
		}
		return `<span class="detail-text">` + template.HTMLEscapeString(fmt.Sprint(v)) + `</span>`
	}
}

func htmlSeverityClass(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical":
		return "sev-critical"
	case "high":
		return "sev-high"
	case "medium":
		return "sev-medium"
	default:
		return "sev-low"
	}
}

func formatSignedInt(v int) string {
	if v > 0 {
		return fmt.Sprintf("+%d", v)
	}
	return fmt.Sprintf("%d", v)
}

const reportHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>cleanr report: {{.Report.Name}}</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #0f1720;
      --bg-2: #172535;
      --panel: #1b2a3a;
      --panel-2: #243b53;
      --ink: #f7fafc;
      --muted: #a0aec0;
      --line: #4a5568;
      --accent: #f6e05e;
      --accent-2: #8ec5ff;
      --pass: #38a169;
      --fail: #e53e3e;
      --critical: #f56565;
      --high: #f6ad55;
      --medium: #63b3ed;
      --low: #cbd5e0;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Avenir Next", "Segoe UI", sans-serif;
      background:
        radial-gradient(circle at top left, rgba(246, 224, 94, 0.12), transparent 26rem),
        radial-gradient(circle at top right, rgba(43, 108, 176, 0.16), transparent 24rem),
        linear-gradient(180deg, var(--bg-2) 0%, var(--bg) 100%);
      color: var(--ink);
    }
    main {
      max-width: 1180px;
      margin: 0 auto;
      padding: 2rem 1.25rem 4rem;
    }
    h1, h2, h3 {
      font-family: "Avenir Next Condensed", "Franklin Gothic Medium", sans-serif;
      letter-spacing: 0.03em;
      margin: 0;
    }
    h1 {
      font-size: clamp(2.2rem, 4vw, 3.5rem);
      line-height: 0.95;
    }
    h2 { font-size: 1.2rem; }
    h3 { font-size: 1rem; }
    p, li { line-height: 1.5; }
    a { color: var(--accent-2); }
    .hero, .panel, .suite-card {
      background: linear-gradient(180deg, rgba(36, 59, 83, 0.94), rgba(31, 45, 61, 0.94));
      border: 1px solid var(--line);
      border-radius: 20px;
    }
    .hero { padding: 1.4rem; }
    .topbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      flex-wrap: wrap;
      margin-bottom: 1.4rem;
    }
    .brand {
      display: flex;
      align-items: center;
      gap: 0.9rem;
    }
    .brand-mark {
      margin: 0;
      padding: 0.55rem 0.7rem;
      border-radius: 12px;
      border: 1px solid rgba(246, 224, 94, 0.22);
      background: rgba(11, 17, 24, 0.34);
      color: var(--accent);
      font: 700 0.42rem/1 "SFMono-Regular", Consolas, monospace;
      letter-spacing: 0;
      white-space: pre;
      overflow: hidden;
      flex: 0 0 auto;
    }
    .brand-text {
      display: flex;
      flex-direction: column;
      gap: 0.2rem;
    }
    .eyebrow {
      color: var(--accent);
      font: 700 0.8rem/1.1 "SFMono-Regular", Consolas, monospace;
      letter-spacing: 0.18em;
      text-transform: uppercase;
    }
    .subtitle {
      color: var(--muted);
      font-size: 0.92rem;
    }
    .status {
      display: inline-flex;
      align-items: center;
      padding: 0.42rem 0.78rem;
      border-radius: 8px;
      font: 700 0.9rem/1 "Avenir Next Condensed", "Franklin Gothic Medium", sans-serif;
      text-transform: uppercase;
      letter-spacing: 0.08em;
      border: 1px solid transparent;
    }
    .status-pass { background: rgba(56, 161, 105, 0.16); color: var(--pass); border-color: rgba(56, 161, 105, 0.34); }
    .status-fail { background: rgba(229, 62, 62, 0.16); color: var(--fail); border-color: rgba(229, 62, 62, 0.34); }
    .status-compact {
      padding: 0.3rem 0.58rem;
      font-size: 0.76rem;
      letter-spacing: 0.07em;
    }
    .hero-grid {
      display: grid;
      grid-template-columns: minmax(0, 1.4fr) minmax(320px, 0.8fr);
      gap: 1rem;
      align-items: start;
    }
    .headline {
      display: flex;
      flex-direction: column;
      gap: 0.8rem;
    }
    .kicker {
      color: var(--muted);
      font-size: 0.9rem;
      margin-top: 0.15rem;
    }
    .hero-note {
      color: var(--muted);
      font-size: 0.95rem;
      max-width: 48rem;
    }
    .metrics {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 0.75rem;
    }
    .metric {
      background: rgba(15, 23, 32, 0.28);
      border: 1px solid var(--line);
      border-radius: 16px;
      padding: 0.85rem 0.9rem;
    }
    .metric-label {
      color: var(--muted);
      font: 700 0.8rem/1 "Avenir Next Condensed", "Franklin Gothic Medium", sans-serif;
      text-transform: uppercase;
      letter-spacing: 0.08em;
    }
    .metric-value {
      display: block;
      font-size: 1.55rem;
      margin-top: 0.35rem;
    }
    .layout {
      display: grid;
      grid-template-columns: minmax(0, 1.55fr) minmax(280px, 0.8fr);
      gap: 1rem;
      margin-top: 1.2rem;
    }
    .stack {
      display: grid;
      gap: 1rem;
      align-content: start;
    }
    .panel { padding: 1.1rem 1.15rem; }
    .panel-head {
      display: flex;
      justify-content: space-between;
      gap: 0.75rem;
      align-items: center;
      margin-bottom: 0.9rem;
    }
    .plain-list {
      margin: 0;
      padding-left: 1.1rem;
    }
    .plain-list li + li { margin-top: 0.45rem; }
    .finding {
      border-left: 4px solid var(--line);
      padding-left: 0.8rem;
      margin-bottom: 0.75rem;
    }
    .sev-critical { border-color: var(--critical); }
    .sev-high { border-color: var(--high); }
    .sev-medium { border-color: var(--medium); }
    .sev-low { border-color: var(--low); }
    .muted { color: var(--muted); }
    .suite-list {
      display: grid;
      gap: 0.9rem;
    }
    .suite-card { padding: 1rem 1.05rem; }
    .suite-header {
      display: flex;
      justify-content: space-between;
      gap: 1rem;
      align-items: flex-start;
      flex-wrap: wrap;
      margin-bottom: 0.75rem;
    }
    .suite-meta {
      display: flex;
      align-items: center;
      gap: 0.55rem;
      flex-wrap: wrap;
      margin-top: 0.35rem;
    }
    .suite-meta-copy {
      color: var(--muted);
      font-size: 0.92rem;
    }
    .case-list {
      display: grid;
      gap: 0.7rem;
    }
    .case {
      padding: 0.8rem 0.85rem;
      border-radius: 16px;
      border: 1px solid rgba(160, 174, 192, 0.18);
      background: rgba(15, 23, 32, 0.26);
    }
    .case-head {
      display: flex;
      justify-content: space-between;
      gap: 0.75rem;
      margin-bottom: 0.45rem;
      align-items: center;
      flex-wrap: wrap;
    }
    .case-stats {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      flex-wrap: wrap;
    }
    .case-stats-copy {
      color: var(--muted);
      font-size: 0.88rem;
    }
    details {
      margin-top: 0.65rem;
      border-top: 1px solid rgba(160, 174, 192, 0.16);
      padding-top: 0.65rem;
    }
    summary {
      cursor: pointer;
      color: var(--accent-2);
      font: 700 0.85rem/1.2 "Avenir Next Condensed", "Franklin Gothic Medium", sans-serif;
      letter-spacing: 0.06em;
      text-transform: uppercase;
      list-style: none;
    }
    summary::-webkit-details-marker { display: none; }
    summary::before {
      content: "+ ";
      color: var(--accent);
    }
    details[open] summary::before {
      content: "− ";
    }
    pre {
      white-space: pre-wrap;
      word-break: break-word;
      background: #0b1118;
      color: #f7fafc;
      padding: 0.85rem;
      border-radius: 14px;
      overflow-x: auto;
      font-size: 0.86rem;
    }
    table {
      width: 100%;
      border-collapse: collapse;
    }
    th, td {
      text-align: left;
      padding: 0.55rem 0;
      border-bottom: 1px solid var(--line);
      vertical-align: top;
    }
    th {
      color: var(--accent);
      font: 700 0.8rem/1 "Avenir Next Condensed", "Franklin Gothic Medium", sans-serif;
      text-transform: uppercase;
      letter-spacing: 0.08em;
    }
    code {
      font-family: "SFMono-Regular", Consolas, monospace;
      font-size: 0.95em;
    }
    .detail-grid {
      display: grid;
      gap: 0.55rem;
      margin-top: 0.75rem;
    }
    .detail-row {
      display: grid;
      grid-template-columns: minmax(140px, 180px) minmax(0, 1fr);
      gap: 0.8rem;
      padding: 0.7rem 0;
      border-top: 1px solid rgba(160, 174, 192, 0.14);
    }
    .detail-row:first-child {
      border-top: 0;
      padding-top: 0;
    }
    .detail-key {
      color: var(--accent);
      font: 700 0.78rem/1.2 "Avenir Next Condensed", "Franklin Gothic Medium", sans-serif;
      text-transform: uppercase;
      letter-spacing: 0.08em;
    }
    .detail-value {
      min-width: 0;
      color: var(--ink);
    }
    .detail-list {
      margin: 0;
      padding-left: 1rem;
      display: grid;
      gap: 0.4rem;
    }
    .detail-chip {
      display: inline-flex;
      align-items: center;
      padding: 0.18rem 0.45rem;
      border-radius: 6px;
      background: rgba(142, 197, 255, 0.12);
      border: 1px solid rgba(142, 197, 255, 0.22);
      color: var(--ink);
      font: 600 0.84rem/1.2 "SFMono-Regular", Consolas, monospace;
    }
    .detail-chip-true {
      background: rgba(56, 161, 105, 0.14);
      border-color: rgba(56, 161, 105, 0.3);
      color: var(--pass);
    }
    .detail-chip-false {
      background: rgba(229, 62, 62, 0.14);
      border-color: rgba(229, 62, 62, 0.3);
      color: var(--fail);
    }
    .detail-text {
      word-break: break-word;
    }
    .detail-empty {
      color: var(--muted);
      font-style: italic;
    }
    @media (max-width: 960px) {
      .hero-grid, .layout {
        grid-template-columns: 1fr;
      }
      .metrics {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
      .detail-row {
        grid-template-columns: 1fr;
        gap: 0.35rem;
      }
    }
    @media (max-width: 640px) {
      main { padding: 1rem 0.9rem 2rem; }
      .metrics {
        grid-template-columns: 1fr;
      }
      .brand {
        align-items: flex-start;
      }
    }
  </style>
</head>
<body>
<main>
  <section class="hero">
    <div class="topbar">
      <div class="brand">
        {{if .LogoASCII}}<pre class="brand-mark" aria-label="cleanr ascii logo">{{.LogoASCII}}</pre>{{end}}
        <div class="brand-text">
          <span class="eyebrow">devr-tools / cleanr</span>
          <span class="subtitle">Static cleanr report dashboard</span>
        </div>
      </div>
      <span class="status {{if .Report.Passed}}status-pass{{else}}status-fail{{end}}">{{if .Report.Passed}}Pass{{else}}Fail{{end}}</span>
    </div>

    <div class="hero-grid">
      <div class="headline">
        <div>
          <h1>{{.Report.Name}}</h1>
          <div class="kicker">Overview</div>
        </div>
        <div class="hero-note">Generated {{.Report.GeneratedAt.Format "2006-01-02 15:04:05 MST"}} with {{.Report.TotalSuites}} suites and {{.Report.TotalCases}} cases.</div>
      </div>
      <div class="metrics">
        <div class="metric"><span class="metric-label">Duration</span><span class="metric-value">{{dur .Report.Duration}}</span></div>
        <div class="metric"><span class="metric-label">Failed Suites</span><span class="metric-value">{{.Report.FailedSuites}}</span></div>
        <div class="metric"><span class="metric-label">Failed Cases</span><span class="metric-value">{{.Report.FailedCases}}</span></div>
        {{if .Report.Metadata}}<div class="metric"><span class="metric-label">Build</span><span class="metric-value">{{if .Report.Metadata.BuildID}}{{.Report.Metadata.BuildID}}{{else}}n/a{{end}}</span></div>{{end}}
      </div>
    </div>
  </section>

  <section class="layout">
    <div class="stack">
      <section class="panel">
        <div class="panel-head">
          <h2>Suites</h2>
          <div class="muted">Primary evaluation results</div>
        </div>
        <div class="suite-list">
          {{range .Report.Suites}}
          <article class="suite-card">
            <div class="suite-header">
              <div>
                <h3>{{.Name}}</h3>
                <div class="suite-meta">
                  <span class="status status-compact {{if .Passed}}status-pass{{else}}status-fail{{end}}">{{if .Passed}}Pass{{else}}Fail{{end}}</span>
                  <span class="suite-meta-copy">{{dur .Duration}} · {{len .Cases}} cases</span>
                </div>
              </div>
            </div>
            {{if .Findings}}
            <div>
              {{range .Findings}}
              <div class="finding {{severity .Severity}}">
                <strong>{{.Severity}}</strong>
                <div>{{.Message}}</div>
              </div>
              {{end}}
            </div>
            {{end}}
            <div class="case-list">
              {{range .Cases}}
              <div class="case">
                <div class="case-head">
                  <strong>{{.Name}}</strong>
                  <div class="case-stats">
                    <span class="status status-compact {{if .Passed}}status-pass{{else}}status-fail{{end}}">{{if .Passed}}Pass{{else}}Fail{{end}}</span>
                    <span class="case-stats-copy">{{dur .Duration}}</span>
                  </div>
                </div>
                {{if .Findings}}
                  {{range .Findings}}<div class="finding {{severity .Severity}}"><strong>{{.Severity}}</strong>: {{.Message}}</div>{{end}}
                {{else}}
                  <div class="muted">No findings.</div>
                {{end}}
                {{if .Details}}
                <details>
                  <summary>View case details</summary>
                  {{detail .Details}}
                </details>
                {{end}}
              </div>
              {{end}}
            </div>
            {{if .Meta}}
            <details>
              <summary>View suite metadata</summary>
              {{detail .Meta}}
            </details>
            {{end}}
          </article>
          {{end}}
        </div>
      </section>
    </div>

    <aside class="stack">
      {{if .Report.Trend}}
      <section class="panel">
        <div class="panel-head">
          <h2>Trend</h2>
          <div class="muted">{{.Report.Trend.HistoryLength}} retained runs</div>
        </div>
        <table>
          <tr><th>Current</th><td>{{if .Report.Trend.CurrentBuildID}}{{.Report.Trend.CurrentBuildID}}{{else}}n/a{{end}}</td></tr>
          <tr><th>Previous</th><td>{{if .Report.Trend.PreviousBuildID}}{{.Report.Trend.PreviousBuildID}}{{else}}n/a{{end}}</td></tr>
          <tr><th>Failed Suites Δ</th><td>{{delta .Report.Trend.Summary.FailedSuitesDelta}}</td></tr>
          <tr><th>Failed Cases Δ</th><td>{{delta .Report.Trend.Summary.FailedCasesDelta}}</td></tr>
          <tr><th>Duration Δ</th><td>{{dur .Report.Trend.Summary.DurationDelta}}</td></tr>
        </table>
        {{if .Report.Trend.BuildDiff}}
        <details>
          <summary>View build diff</summary>
          {{detail .Report.Trend.BuildDiff}}
        </details>
        {{end}}
      </section>
      {{end}}

      {{if .Report.TrendGate}}
      <section class="panel">
        <div class="panel-head">
          <h2>Trend Gate</h2>
          <div class="muted">{{.Report.TrendGate.AvailableWindow}} / {{.Report.TrendGate.RequiredWindow}} runs</div>
        </div>
        <div class="status {{if .Report.TrendGate.Passed}}status-pass{{else}}status-fail{{end}}">{{if .Report.TrendGate.Passed}}Passing{{else}}Failing{{end}}</div>
        {{if .Report.TrendGate.Findings}}
        <div style="margin-top:0.8rem;">
          {{range .Report.TrendGate.Findings}}
          <div class="finding {{severity .Severity}}">
            <strong>{{.Severity}}</strong>
            <div>{{.Message}}</div>
          </div>
          {{end}}
        </div>
        {{end}}
      </section>
      {{end}}

      {{if .Report.Recommendations}}
      <section class="panel">
        <div class="panel-head">
          <h2>Recommendations</h2>
        </div>
        <ul class="plain-list">
          {{range .Report.Recommendations}}<li>{{.}}</li>{{end}}
        </ul>
      </section>
      {{end}}

      {{if .Report.Integrations}}
      <section class="panel">
        <div class="panel-head">
          <h2>Integrations</h2>
        </div>
        {{if .Report.Integrations.TrendSources}}
        <h3 style="margin-bottom:0.55rem;">Trend Sources</h3>
        <ul class="plain-list">
          {{range .Report.Integrations.TrendSources}}
          <li><strong>{{.Name}}</strong>: {{.Status}}{{if .ViewURL}} (<a href="{{.ViewURL}}">view</a>){{end}}</li>
          {{end}}
        </ul>
        {{end}}
        {{if .Report.Integrations.ResultSinks}}
        <h3 style="margin:1rem 0 0.55rem;">Result Sinks</h3>
        <ul class="plain-list">
          {{range .Report.Integrations.ResultSinks}}
          <li><strong>{{.Name}}</strong>: {{.Message}}{{if .RunURL}} (<a href="{{.RunURL}}">run</a>){{end}}</li>
          {{end}}
        </ul>
        {{end}}
        {{if .Report.Integrations.Summaries}}
        <h3 style="margin:1rem 0 0.55rem;">Artifacts</h3>
        <ul class="plain-list">
          {{range .Report.Integrations.Summaries}}
          <li><strong>{{.Name}}</strong>: <code>{{.Output}}</code></li>
          {{end}}
        </ul>
        {{end}}
      </section>
      {{end}}
    </aside>
  </section>
</main>
</body>
</html>`
