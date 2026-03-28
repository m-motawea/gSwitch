# L2 Hub (L2)

The `Hub` is the simplest Layer 2 control structure designed to blanket replicate physical frames seamlessly validating baseline architecture execution correctly over interfaces logically bypassing intricate processing mapping operations securely.

## Behavior
By design, the Hub lacks persistence and dynamically floods packets synchronously.

- **Ingress (`HubInProc`)**: Selects ALL available Switch Ports natively via enumerating `msgContent.ParentSwitch.Ports` recursively (excluding the original ingress port seamlessly) and appends them directly sequentially updating `msgContent.OutPorts` accurately mimicking primitive network switch flooding routing behaviors locally effectively.
- **Egress (`HubOutProc`)**: Instructs the Pipeline execution to gracefully finish processing pipelines halting logic immediately natively skipping additional reverse egress iteration logic (`msg.Finished = true`).

## Central Registry Access
```go
msgContent, _ := controlplane.FetchMessage(msg.Content)
```
