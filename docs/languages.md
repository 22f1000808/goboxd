# Languages Configuration

GoBoxd supports plug-and-play language execution. The language registry (`configs/languages.yaml`) is the single configuration seam. Adding a language requires **zero** Go code changes.

## Adding a Language

1. **Update YAML Block**: Add a new block to `configs/languages.yaml` detailing the language ID, source file name strategy, execution phases (run and/or build), and resource limits.
2. **Update Dockerfile**: Add the required runtime or toolchain installation package to the `toolchains` layer of the `Dockerfile`.
3. **Rebuild**: Run `docker compose up --build`.

### Example Configuration

```yaml
  - id: cpp
    name: C++
    source_filename: solution.cpp
    source_filename_strategy: fixed
    artifact: solution
    build:
      cmd: /usr/bin/g++
      args: ["{{flags}}", "-o", "{{artifact}}", "{{source}}"]
      limits: { wall_time_s: 3, memory_kb: 1048576, max_processes: 100 }
      flag_allowlist: ["-O0","-O1","-O2","-O3","-Wall","-Wextra","-std=*"]
    run:
      cmd: ./{{artifact}}
      limits: { wall_time_s: 3, memory_kb: 524288, max_processes: 64 }
    smoke_probe_cmd: /usr/bin/g++
    smoke_probe: ["--version"]
```

## Template Expansion
- `{{source}}`: Expands to the path of the source file inside the jail.
- `{{artifact}}`: Expands to the path of the build artifact.
- `{{flags}}`: A **splice expansion** of validated flags. The flags are injected safely as individual elements rather than a single string, enabling multi-flag compilation without shell splitting vulnerability.
