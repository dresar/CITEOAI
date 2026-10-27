# 16 - BENCHMARK AND MEMORY PROFILING

## 1. Memory Verification Standard
CITEOAI is built on the pledge of remaining **under 100 MB RAM** at all times during runtime.

## 2. Profiling Protocol
Every release candidate must undergo memory profiling using native Go tools:

```bash
# 1. Run agent under profiling
go run ./cmd/citeoai/ run "analyze and refactor 10 files" --memprofile=mem.pprof --cpuprofile=cpu.pprof

# 2. Inspect heap allocations
go tool pprof -top mem.pprof

# 3. Check live RSS in PowerShell
Get-Process citeoai | Select-Object WorkingSet64, PrivateMemorySize64
```

## 3. Allocation Limits
- Maximum heap allocations per turn: < 15 MB.
- Maximum persistent heap: < 40 MB.
- Buffers for SSE decoding and string concatenation must use `sync.Pool` or pre-allocated slices.
- Never read unbounded files directly into `[]byte`; use buffered readers (`bufio.Reader` or `io.LimitReader`).
