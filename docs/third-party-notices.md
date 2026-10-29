# Third-party notice maintenance

Sectile Desktop bundles a Rust application, a React/JavaScript production
bundle, and Go service executables. The packaged app includes a top-level
notice index and license files in its Tauri resources. Go service resources
also carry their own matching notice set.

## JavaScript

After changing production dependencies, install from the lockfile and
regenerate the package inventory and copied license texts:

```sh
npm ci
npm run licenses:javascript
```

Review `THIRD_PARTY_NOTICES_JS.md` and the files under
`third_party_licenses/javascript/` in the same change as `package-lock.json`.

## Rust

`src-tauri/about.toml` records the license acceptance policy and the arm64 and
x86_64 macOS target graphs. `licenses/rust-notices.hbs` is the local report
template. Install `cargo-about` with its CLI feature and regenerate the report:

```sh
cargo install cargo-about --locked --features cli
cargo about generate licenses/rust-notices.hbs \
  --config src-tauri/about.toml \
  --manifest-path src-tauri/Cargo.toml \
  --locked \
  -o THIRD_PARTY_NOTICES_RUST.html
```

The report lists each resolved crate and discovered license text. Review the
crate list and SPDX expressions in `THIRD_PARTY_NOTICES_RUST.html` after
changing `Cargo.lock`.

## Go services

The service dependency inventory is maintained in the separate Go repository's
`THIRD_PARTY_NOTICES.md` and `third_party_licenses/`. It covers the server,
agent and companion command graphs, plus the Go standard library. Go notice
generation uses `go-licenses report` and `go-licenses save`; the custom BSD
license on `modernc.org/mathutil` is included manually because the scanner does
not classify its valid `LICENSE` file.

When preparing Tauri services, `scripts/build-services.mjs` verifies the
notice's `go.mod` SHA-256 against the selected Go checkout and synchronizes its
notice and license files into the Tauri repository and service resources. It
normalizes copied file permissions so incremental Tauri builds can replace
staged resources. Commit any notice changes made by this command alongside the
matching Go dependency update.

## Package verification

The Tauri bundle configuration includes the top-level notices, the Rust
report, JavaScript and Go indexes, and their license files. Verify the files in
`Contents/Resources/` and `Contents/Resources/binaries/` of a macOS `.app`
after packaging.
