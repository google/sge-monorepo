Package check contains utilities for checker tools.
When you link against this library two flags are registered with the flag library,
`--checker-invocation` and `--checker-invocation-result`.

These point to protos that communicate presubmit results between sgep and the checker tool.
Use `check.MustLoad` to obtain a helper object that assists in loading the invocation input
and writing the checker result.
