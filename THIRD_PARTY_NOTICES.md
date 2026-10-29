# Third-party notices

This file covers the Go executables built from this repository: the server, workstation agent, and Local companion. It records the module dependency graph used by those commands. The license texts are retained under [`third_party_licenses/`](third_party_licenses/).

The inventory was generated from the command dependency graphs on 2026-10-29. The analyzed `go.mod` SHA-256 is `aee7fa8103544ef843a80ebee2b4c7569f6435938522e56f7fbad149b5a7d876`. `go-licenses` could not classify `modernc.org/mathutil` automatically even though its module contains a BSD-style `LICENSE`; that license was inspected and included manually. Go assembly/C source caused scanner warnings because it cannot inspect non-Go files, so module-level license files were included for the reported modules.

## Go standard library

- Go go1.26.6 darwin/arm64: BSD-style license in [`third_party_licenses/golang.org/go/LICENSE`](third_party_licenses/golang.org/go/LICENSE).

## Go modules

| Module | Version | License | License text |
| --- | --- | --- | --- |
| `github.com/aymanbagabas/go-pty` | `v0.2.3` | MIT | [`LICENSE`](third_party_licenses/github.com/aymanbagabas/go-pty/LICENSE) |
| `github.com/beorn7/perks` | `v1.0.1` | MIT | [`LICENSE`](third_party_licenses/github.com/beorn7/perks/quantile/LICENSE) |
| `github.com/cespare/xxhash/v2` | `v2.3.0` | MIT | [`LICENSE.txt`](third_party_licenses/github.com/cespare/xxhash/v2/LICENSE.txt) |
| `github.com/creack/pty` | `v1.1.24` | MIT | [`LICENSE`](third_party_licenses/github.com/creack/pty/LICENSE) |
| `github.com/dustin/go-humanize` | `v1.0.1` | MIT | [`LICENSE`](third_party_licenses/github.com/dustin/go-humanize/LICENSE) |
| `github.com/google/jsonschema-go` | `v0.4.3` | MIT | [`LICENSE`](third_party_licenses/github.com/google/jsonschema-go/jsonschema/LICENSE) |
| `github.com/google/uuid` | `v1.6.0` | BSD-3-Clause | [`LICENSE`](third_party_licenses/github.com/google/uuid/LICENSE) |
| `github.com/gorilla/websocket` | `v1.5.3` | BSD-2-Clause | [`LICENSE`](third_party_licenses/github.com/gorilla/websocket/LICENSE) |
| `github.com/jackc/pgpassfile` | `v1.0.0` | MIT | [`LICENSE`](third_party_licenses/github.com/jackc/pgpassfile/LICENSE) |
| `github.com/jackc/pgservicefile` | `v0.0.0-20240606120523-5a60cdf6a761` | MIT | [`LICENSE`](third_party_licenses/github.com/jackc/pgservicefile/LICENSE) |
| `github.com/jackc/pgx/v5` | `v5.7.2` | MIT | [`LICENSE`](third_party_licenses/github.com/jackc/pgx/v5/LICENSE) |
| `github.com/jackc/puddle/v2` | `v2.2.2` | MIT | [`LICENSE`](third_party_licenses/github.com/jackc/puddle/v2/LICENSE) |
| `github.com/mattn/go-isatty` | `v0.0.24` | MIT | [`LICENSE`](third_party_licenses/github.com/mattn/go-isatty/LICENSE) |
| `github.com/modelcontextprotocol/go-sdk` | `v1.7.0` | Apache-2.0 | [`LICENSE`](third_party_licenses/github.com/modelcontextprotocol/go-sdk/LICENSE) |
| `github.com/munnerz/goautoneg` | `v0.0.0-20191010083416-a7dc8b61c822` | BSD-3-Clause | [`LICENSE`](third_party_licenses/github.com/munnerz/goautoneg/LICENSE) |
| `github.com/ncruces/go-strftime` | `v1.0.0` | MIT | [`LICENSE`](third_party_licenses/github.com/ncruces/go-strftime/LICENSE) |
| `github.com/pelletier/go-toml/v2` | `v2.2.4` | MIT | [`LICENSE`](third_party_licenses/github.com/pelletier/go-toml/v2/LICENSE) |
| `github.com/prometheus/client_golang` | `v1.23.2` | BSD-3-Clause | [`LICENSE`](third_party_licenses/github.com/prometheus/client_golang/internal/github.com/golang/gddo/httputil/LICENSE) |
| `github.com/prometheus/client_golang` | `v1.23.2` | Apache-2.0 | [`LICENSE`](third_party_licenses/github.com/prometheus/client_golang/prometheus/LICENSE) |
| `github.com/prometheus/client_model` | `v0.6.2` | Apache-2.0 | [`LICENSE`](third_party_licenses/github.com/prometheus/client_model/go/LICENSE) |
| `github.com/prometheus/common` | `v0.66.1` | Apache-2.0 | [`LICENSE`](third_party_licenses/github.com/prometheus/common/LICENSE) |
| `github.com/remyoudompheng/bigfft` | `v0.0.0-20230129092748-24d4a6f8daec` | BSD-3-Clause | [`LICENSE`](third_party_licenses/github.com/remyoudompheng/bigfft/LICENSE) |
| `github.com/segmentio/asm` | `v1.1.3` | MIT | [`LICENSE`](third_party_licenses/github.com/segmentio/asm/LICENSE) |
| `github.com/segmentio/encoding` | `v0.5.4` | MIT | [`LICENSE`](third_party_licenses/github.com/segmentio/encoding/LICENSE) |
| `github.com/u-root/u-root` | `v0.16.0` | BSD-3-Clause | [`LICENSE`](third_party_licenses/github.com/u-root/u-root/pkg/termios/LICENSE) |
| `github.com/yosida95/uritemplate/v3` | `v3.0.2` | BSD-3-Clause | [`LICENSE`](third_party_licenses/github.com/yosida95/uritemplate/v3/LICENSE) |
| `github.com/yuin/goldmark` | `v1.8.6` | MIT | [`LICENSE`](third_party_licenses/github.com/yuin/goldmark/LICENSE) |
| `go.yaml.in/yaml/v2` | `v2.4.2` | Apache-2.0 | [`LICENSE`](third_party_licenses/go.yaml.in/yaml/v2/LICENSE) |
| `golang.org/x/crypto` | `v0.51.0` | BSD-3-Clause | [`LICENSE`](third_party_licenses/golang.org/x/crypto/LICENSE) |
| `golang.org/x/oauth2` | `v0.35.0` | BSD-3-Clause | [`LICENSE`](third_party_licenses/golang.org/x/oauth2/LICENSE) |
| `golang.org/x/sync` | `v0.21.0` | BSD-3-Clause | [`LICENSE`](third_party_licenses/golang.org/x/sync/LICENSE) |
| `golang.org/x/sys` | `v0.48.0` | BSD-3-Clause | [`LICENSE`](third_party_licenses/golang.org/x/sys/LICENSE) |
| `golang.org/x/term` | `v0.46.0` | BSD-3-Clause | [`LICENSE`](third_party_licenses/golang.org/x/term/LICENSE) |
| `golang.org/x/text` | `v0.37.0` | BSD-3-Clause | [`LICENSE`](third_party_licenses/golang.org/x/text/LICENSE) |
| `golang.org/x/time` | `v0.15.0` | BSD-3-Clause | [`LICENSE`](third_party_licenses/golang.org/x/time/rate/LICENSE) |
| `google.golang.org/protobuf` | `v1.36.8` | BSD-3-Clause | [`LICENSE`](third_party_licenses/google.golang.org/protobuf/LICENSE) |
| `modernc.org/libc` | `v1.74.4` | MIT | [`LICENSE-3RD-PARTY.md`](third_party_licenses/modernc.org/libc/LICENSE-3RD-PARTY.md) |
| `modernc.org/mathutil` | `v1.7.1` | BSD-3-Clause | [`LICENSE`](third_party_licenses/modernc.org/mathutil/LICENSE) |
| `modernc.org/memory` | `v1.11.0` | BSD-3-Clause | [`LICENSE-GO`](third_party_licenses/modernc.org/memory/LICENSE-GO) |
| `modernc.org/sqlite` | `v1.57.0` | BSD-3-Clause | [`LICENSE`](third_party_licenses/modernc.org/sqlite/LICENSE) |

## Product license

Sectile source code in this repository is licensed under MIT; see [`LICENSE`](LICENSE).
