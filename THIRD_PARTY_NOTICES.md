# Third-party software notices

Sectile Tauri distributes the native desktop shell, its production JavaScript bundle, and three Go service executables. This index identifies each dependency set and links to its attribution and license texts.

- **Go server, agent, and companion:** [`THIRD_PARTY_NOTICES_GO.md`](THIRD_PARTY_NOTICES_GO.md), including module versions and copied license files.
- **Rust native application:** [`THIRD_PARTY_NOTICES_RUST.html`](THIRD_PARTY_NOTICES_RUST.html), generated with `cargo-about` from the locked macOS arm64 and x86_64 Cargo dependency graphs. It lists each crate and the discovered license text.
- **JavaScript production bundle:** [`THIRD_PARTY_NOTICES_JS.md`](THIRD_PARTY_NOTICES_JS.md), generated from the production entries in `package-lock.json`.

The license texts are retained under [`third_party_licenses/`](third_party_licenses/). Sectile's own desktop source is MIT-licensed; see [`third_party_licenses/tauri/LICENSE`](third_party_licenses/tauri/LICENSE). The separate Go source repository is also MIT-licensed, with its license included in the Go notice set.
