package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/devr-tools/cleanr/cleanr/core"
)

var methodOrder = map[string]int{
	"GET":     0,
	"POST":    1,
	"PUT":     2,
	"PATCH":   3,
	"DELETE":  4,
	"HEAD":    5,
	"OPTIONS": 6,
	"TRACE":   7,
}

type document struct {
	Paths map[string]pathItem
}

type pathItem map[string]operation

type operation struct {
	OperationID string
	Tags        []string
	Deprecated  bool
	Parameters  []parameter
	RequestBody requestBody
	Responses   map[string]response
}

type parameter struct {
	Name     string
	In       string
	Required bool
	Schema   map[string]any
	Example  any
}

type requestBody struct {
	Required bool
	Content  map[string]mediaType
}

type response struct {
	Description string
	Content     map[string]mediaType
}

type mediaType struct {
	Schema map[string]any
}

func GenerateScenarios(ctx context.Context, cfg core.Config, client *http.Client) ([]core.Scenario, error) {
	doc, err := loadDocument(ctx, cfg.OpenAPI.Source, client)
	if err != nil {
		return nil, err
	}

	filterMethods := methodSet(cfg.OpenAPI.ScenarioGeneration.IncludeMethods)
	filterTags := stringSet(cfg.OpenAPI.ScenarioGeneration.IncludeTags)
	ops := sortedOperations(doc)

	scenarios := make([]core.Scenario, 0, len(ops))
	for _, item := range ops {
		if len(filterMethods) > 0 && !filterMethods[item.Method] {
			continue
		}
		if !cfg.OpenAPI.ScenarioGeneration.IncludeDeprecated && item.Operation.Deprecated {
			continue
		}
		if len(filterTags) > 0 && !matchesTag(filterTags, item.Operation.Tags) {
			continue
		}
		scenarios = append(scenarios, buildScenario(item.Path, item.Method, item.Operation))
	}
	return scenarios, nil
}

func DiffContracts(ctx context.Context, cfg core.Config, client *http.Client) (core.OpenAPIContractDiff, error) {
	current, err := loadDocument(ctx, cfg.OpenAPI.Source, client)
	if err != nil {
		return core.OpenAPIContractDiff{}, err
	}
	baseline, err := loadDocument(ctx, cfg.OpenAPI.ContractDiff.Baseline, client)
	if err != nil {
		return core.OpenAPIContractDiff{}, err
	}

	currentOps := operationMap(sortedOperations(current))
	baselineOps := operationMap(sortedOperations(baseline))

	diff := core.OpenAPIContractDiff{}

	for key, candidate := range currentOps {
		base, ok := baselineOps[key]
		if !ok {
			diff.Changes = append(diff.Changes, operationAddedChange(candidate))
			diff.Summary.OperationsAdded++
			continue
		}
		changes := diffOperationChange(base, candidate)
		diff.Changes = append(diff.Changes, changes...)
		if len(changes) > 0 {
			diff.Summary.OperationsChanged++
		}
	}

	for key, base := range baselineOps {
		if _, ok := currentOps[key]; ok {
			continue
		}
		diff.Changes = append(diff.Changes, operationRemovedChange(base))
		diff.Summary.OperationsRemoved++
	}

	summarizeDiffChanges(&diff)
	sortContractChanges(diff.Changes)

	return diff, nil
}

func operationAddedChange(candidate operationItem) core.OpenAPIContractChange {
	return core.OpenAPIContractChange{
		Kind:        "operation_added",
		Level:       "non_breaking",
		Method:      candidate.Method,
		Path:        candidate.Path,
		OperationID: candidate.Operation.OperationID,
		Detail:      "operation added in candidate contract",
	}
}

func operationRemovedChange(base operationItem) core.OpenAPIContractChange {
	return core.OpenAPIContractChange{
		Kind:        "operation_removed",
		Level:       "breaking",
		Method:      base.Method,
		Path:        base.Path,
		OperationID: base.Operation.OperationID,
		Detail:      "operation removed from candidate contract",
	}
}

func diffOperationChange(base, candidate operationItem) []core.OpenAPIContractChange {
	changes := requiredParameterChanges(base, candidate)
	if responseSchemaChanged(base.Operation.Responses, candidate.Operation.Responses) {
		changes = append(changes, core.OpenAPIContractChange{
			Kind:        "response_schema_changed",
			Level:       "breaking",
			Method:      candidate.Method,
			Path:        candidate.Path,
			OperationID: candidate.Operation.OperationID,
			Location:    "responses",
			Detail:      "response schema changed",
		})
	}
	return changes
}

func requiredParameterChanges(base, candidate operationItem) []core.OpenAPIContractChange {
	changes := make([]core.OpenAPIContractChange, 0)
	for _, param := range candidate.Operation.Parameters {
		if !isRequiredRequestParameter(param) || hasMatchingParameter(base.Operation.Parameters, param) {
			continue
		}
		changes = append(changes, core.OpenAPIContractChange{
			Kind:        "parameter_added",
			Level:       "breaking",
			Method:      candidate.Method,
			Path:        candidate.Path,
			OperationID: candidate.Operation.OperationID,
			Location:    fmt.Sprintf("%s.%s", param.In, param.Name),
			Detail:      "required parameter added",
		})
	}
	return changes
}

func isRequiredRequestParameter(param parameter) bool {
	location := strings.ToLower(strings.TrimSpace(param.In))
	return param.Required && (location == "query" || location == "path")
}

func summarizeDiffChanges(diff *core.OpenAPIContractDiff) {
	for _, change := range diff.Changes {
		if change.Level == "breaking" {
			diff.Breaking = true
			diff.Summary.BreakingChanges++
			continue
		}
		diff.Summary.NonBreakingChanges++
	}
}

func sortContractChanges(changes []core.OpenAPIContractChange) {
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		if changes[i].Method != changes[j].Method {
			return changes[i].Method < changes[j].Method
		}
		return changes[i].Kind < changes[j].Kind
	})
}

type operationItem struct {
	Path      string
	Method    string
	Operation operation
}

func sortedOperations(doc document) []operationItem {
	items := make([]operationItem, 0)
	for pathValue, pathItem := range doc.Paths {
		for method, op := range pathItem {
			upperMethod := strings.ToUpper(strings.TrimSpace(method))
			if _, ok := methodOrder[upperMethod]; !ok {
				continue
			}
			items = append(items, operationItem{Path: pathValue, Method: upperMethod, Operation: op})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if methodOrder[items[i].Method] != methodOrder[items[j].Method] {
			return methodOrder[items[i].Method] < methodOrder[items[j].Method]
		}
		if items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		return items[i].Operation.OperationID < items[j].Operation.OperationID
	})
	return items
}

func buildScenario(pathValue, method string, op operation) core.Scenario {
	metadata := map[string]string{
		"openapi.method": method,
	}

	resolvedPath := pathValue
	queryPairs := make([]string, 0)
	for _, param := range op.Parameters {
		value := parameterExample(param)
		switch strings.ToLower(strings.TrimSpace(param.In)) {
		case "path":
			resolvedPath = strings.ReplaceAll(resolvedPath, "{"+param.Name+"}", urlEncodePath(value))
		case "query":
			queryPairs = append(queryPairs, neturl.QueryEscape(param.Name)+"="+neturl.QueryEscape(value))
		}
	}
	metadata["openapi.path"] = resolvedPath
	if len(queryPairs) > 0 {
		metadata["openapi.query"] = strings.Join(queryPairs, "&")
	}

	input, contentType := requestBodyExample(op.RequestBody)
	if contentType != "" {
		metadata["openapi.content_type"] = contentType
	}

	assertions := []core.Assertion{}
	if statusCode := firstResponseStatus(op.Responses); statusCode != 0 {
		statusValue := statusCode
		assertions = append(assertions, core.Assertion{Type: "status_code", IntValue: &statusValue})
	}
	if schema := firstResponseSchema(op.Responses); schema != nil {
		assertions = append(assertions, core.Assertion{Type: "json_schema", Schema: schema})
	}

	name := strings.TrimSpace(op.OperationID)
	if name == "" {
		name = scenarioName(method, pathValue)
	}

	return core.Scenario{
		Name:       name,
		Input:      input,
		Metadata:   metadata,
		Assertions: assertions,
	}
}

func firstResponseStatus(responses map[string]response) int {
	codes := make([]string, 0, len(responses))
	for code := range responses {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		if n, err := strconv.Atoi(code); err == nil {
			return n
		}
	}
	return 0
}

func firstResponseSchema(responses map[string]response) map[string]any {
	codes := make([]string, 0, len(responses))
	for code := range responses {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		contentTypes := sortedMediaTypes(responses[code].Content)
		for _, contentType := range contentTypes {
			schema := responses[code].Content[contentType].Schema
			if schema != nil {
				return schema
			}
		}
	}
	return nil
}

func requestBodyExample(body requestBody) (string, string) {
	contentTypes := sortedMediaTypes(body.Content)
	for _, contentType := range contentTypes {
		schema := body.Content[contentType].Schema
		if schema == nil {
			continue
		}
		value := schemaExample(schema)
		data, err := json.Marshal(value)
		if err != nil {
			continue
		}
		return string(data), contentType
	}
	return "", ""
}

func sortedMediaTypes(content map[string]mediaType) []string {
	keys := make([]string, 0, len(content))
	for key := range content {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func parameterExample(param parameter) string {
	if value := valueExample(param.Example); value != "" {
		return value
	}
	if value := valueExample(param.Schema["example"]); value != "" {
		return value
	}
	if value := valueExample(param.Schema["default"]); value != "" {
		return value
	}
	return valueExample(schemaExample(param.Schema))
}

func valueExample(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		if value == nil {
			return ""
		}
		data, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

func schemaExample(schema map[string]any) any {
	if schema == nil {
		return nil
	}
	if value := schema["example"]; value != nil {
		return value
	}
	if value := schema["default"]; value != nil {
		return value
	}
	typeName, _ := schema["type"].(string)
	switch typeName {
	case "object":
		props, _ := schema["properties"].(map[string]any)
		keys := make([]string, 0, len(props))
		for key := range props {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := map[string]any{}
		for _, key := range keys {
			propSchema, _ := props[key].(map[string]any)
			out[key] = schemaExample(propSchema)
		}
		return out
	case "array":
		items, _ := schema["items"].(map[string]any)
		return []any{schemaExample(items)}
	case "boolean":
		return true
	case "integer":
		return 1
	case "number":
		return 1.0
	default:
		return "example"
	}
}

func scenarioName(method, pathValue string) string {
	name := strings.ToLower(method + "-" + pathValue)
	replacer := strings.NewReplacer("/", "-", "{", "", "}", "", "_", "-")
	name = replacer.Replace(name)
	name = strings.Trim(name, "-")
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	return name
}

func methodSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	set := map[string]bool{}
	for _, value := range values {
		set[strings.ToUpper(strings.TrimSpace(value))] = true
	}
	return set
}

func stringSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	set := map[string]bool{}
	for _, value := range values {
		set[strings.TrimSpace(value)] = true
	}
	return set
}

func matchesTag(tags map[string]bool, values []string) bool {
	for _, value := range values {
		if tags[strings.TrimSpace(value)] {
			return true
		}
	}
	return false
}

func urlEncodePath(value string) string {
	return strings.ReplaceAll(neturl.PathEscape(value), "+", "%20")
}

func operationMap(items []operationItem) map[string]operationItem {
	out := make(map[string]operationItem, len(items))
	for _, item := range items {
		out[item.Method+" "+item.Path] = item
	}
	return out
}

func hasMatchingParameter(params []parameter, target parameter) bool {
	for _, param := range params {
		if strings.EqualFold(strings.TrimSpace(param.Name), strings.TrimSpace(target.Name)) &&
			strings.EqualFold(strings.TrimSpace(param.In), strings.TrimSpace(target.In)) {
			return true
		}
	}
	return false
}

func responseSchemaChanged(baseResponses, candidateResponses map[string]response) bool {
	baseSchema := firstResponseSchema(baseResponses)
	candidateSchema := firstResponseSchema(candidateResponses)
	return !jsonEqual(baseSchema, candidateSchema)
}

func jsonEqual(left, right any) bool {
	leftData, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightData, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return bytesEqual(leftData, rightData)
}

func bytesEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func loadDocument(ctx context.Context, source core.OpenAPISource, client *http.Client) (document, error) {
	raw, err := loadSourceBytes(ctx, source, client)
	if err != nil {
		return document{}, err
	}
	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return document{}, err
	}
	if doc.Paths == nil {
		doc.Paths = map[string]pathItem{}
	}
	return doc, nil
}

func loadSourceBytes(ctx context.Context, source core.OpenAPISource, client *http.Client) ([]byte, error) {
	switch {
	case source.Inline != nil:
		return json.Marshal(source.Inline)
	case strings.TrimSpace(source.Path) != "":
		return os.ReadFile(strings.TrimSpace(source.Path))
	case strings.TrimSpace(source.URL) != "":
		httpClient := client
		if httpClient == nil {
			httpClient = http.DefaultClient
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(source.URL), nil)
		if err != nil {
			return nil, err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	default:
		return nil, fmt.Errorf("openapi source is empty")
	}
}
