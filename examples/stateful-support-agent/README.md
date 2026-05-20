# Stateful Support Agent Example

This example demonstrates a real end-to-end Phase 2 workflow:

- an HTTP-based agent emits normalized workflow evidence through the generic adapter
- `cleanr` enforces action-level release policy
- exact expected state changes are verified from `state_changes`, not only file diffs
- the backing file store is still observable through `shadow_state`

## Run the Agent

From the repository root:

```bash
go run ./examples/stateful-support-agent
```

The server listens on `http://localhost:8091`.

## Run cleanr

In another terminal:

```bash
./dist/cleanr validate -config examples/stateful-support-agent/cleanr.yaml
./dist/cleanr run -config examples/stateful-support-agent/cleanr.yaml
```

## What It Proves

The sample agent:

- looks up the customer
- performs a read-only SQL lookup
- updates the support case
- drafts but does not send the confirmation email
- records approval evidence
- returns normalized `tool_calls`, `approvals`, and `state_changes`

The `release_policy` suite then gates that workflow with declarative rules instead of custom code.
