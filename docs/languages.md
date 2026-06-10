# Languages

goboxd supports the following languages. All are configured via
`configs/languages.yaml`; adding a new language requires no Go changes.

## In-scope languages (Stage 1 spec §02)

| ID | Name | Toolchain | Source file | Build |
|----|------|-----------|-------------|-------|
| `bash` | Bash | `/bin/bash` | `solution.sh` | None (interpreted) |
| `c` | C | `/usr/bin/gcc` | `solution.c` | `gcc -o solution solution.c -lm` |
| `cpp` | C++ | `/usr/bin/g++` | `solution.cpp` | `g++ -o solution solution.cpp` |
| `java` | Java | `/usr/bin/javac` + `/usr/bin/java` | client-supplied (e.g. `Main.java`) | `javac Main.java` |
| `javascript` | JavaScript (Node.js) | `/usr/bin/node` | `solution.js` | None (interpreted) |
| `py3` | Python 3 | `/usr/bin/python3` | `solution.py` | None (interpreted) |
| `verilog` | Verilog (Icarus) | `/usr/bin/iverilog` + `/usr/bin/vvp` | `solution.v` | `iverilog -o solution.vvp solution.v` |

## Beyond-seven languages (bonus)

| ID | Name | Toolchain | Source file | Build |
|----|------|-----------|-------------|-------|
| `go` | Go | `/usr/bin/go` | `solution.go` | None (`go run`) |
| `lua` | Lua | `/usr/bin/lua5.4` | `solution.lua` | None (interpreted) |
| `ruby` | Ruby | `/usr/bin/ruby` | `solution.rb` | None (interpreted) |
| `rust` | Rust | `/usr/bin/rustc` | `solution.rs` | `rustc -o solution solution.rs` |

## Java note

Java requires `source_filename_strategy: from_request`. The client must
supply `source_filename` matching the public class name (e.g. `Main.java`).
The artifact field defaults to `Main` (the class name without `.java`).

## Demo-day language add (≤10 min drill)

To add a new language (e.g. `kotlin`):

1. Add a YAML block to `configs/languages.yaml`:

```yaml
- id: kotlin
  name: Kotlin
  source_filename: solution.kt
  artifact: solution.jar
  build:
    cmd: /usr/bin/kotlinc
    args: ["{{source}}", "-include-runtime", "-d", "{{artifact}}"]
    limits: { wall_time_s: 60, memory_kb: 1048576, max_processes: 200 }
  run:
    cmd: /usr/bin/java
    args: ["-jar", "{{artifact}}"]
    limits: { wall_time_s: 10, memory_kb: 524288, max_processes: 100 }
  smoke_probe_cmd: /usr/bin/kotlinc
  smoke_probe: ["-version"]
```

2. Add `scripts/lang_install/kotlin.sh`:

```sh
#!/bin/sh
set -e
apt-get install -y --no-install-recommends kotlin
kotlinc -version
```

3. Rebuild the Docker image:

```
make docker-build
make docker-run
curl :8080/readyz | python3 -m json.tool
```

No Go code changes. The new language appears in `/readyz` and `/info`
automatically. Boot validation will abort if the toolchain is missing.

## Flag policy

Each compiled language defines a `flag_allowlist` in `languages.yaml`.
Additionally, the following tokens are unconditionally denied across all
languages regardless of the allowlist:

`-fplugin`, `-x`, `-B`, `--specs`, `-Wl,`, `@` (response files), `-I/`
(absolute include paths).

A disallowed flag returns `HTTP 400` with code `flag_not_allowed`.
