#!/usr/bin/env python3
"""Verify the macOS app and DMG embed the prepared Go services and notices."""

import hashlib
import json
import os
import pathlib
import plistlib
import subprocess
import sys


ROOT = pathlib.Path(__file__).resolve().parents[1]
APP_NAME = "Sectile Tauri.app"
LOCAL_RUNTIME_PORT = 52963


def run(args, capture=False):
    return subprocess.run(
        args,
        check=True,
        stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.PIPE if capture else None,
        text=True if capture else None,
    )


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_json(path):
    with path.open(encoding="utf-8") as source:
        return json.load(source)


def verify_bundle(app, expected_manifest, config):
    info_path = app / "Contents/Info.plist"
    with info_path.open("rb") as source:
        info = plistlib.load(source)
    if info.get("CFBundleIdentifier") != config["identifier"]:
        raise ValueError(f"Unexpected app identifier in {app}")
    if info.get("CFBundleShortVersionString") != config["version"]:
        raise ValueError(f"Unexpected app version in {app}")

    executable = app / "Contents/MacOS" / info["CFBundleExecutable"]
    architectures = run(["lipo", "-archs", str(executable)], capture=True).stdout.split()
    if architectures != ["arm64"]:
        raise ValueError(f"Expected an arm64-only app, found {architectures}")

    resources = app / "Contents/Resources"
    binaries = resources / "binaries"
    embedded_manifest = load_json(binaries / "build-manifest.json")
    if embedded_manifest != expected_manifest:
        raise ValueError(f"Packaged Go manifest does not match src-tauri/binaries in {app}")
    for artifact in expected_manifest.get("services", []):
        binary = binaries / artifact["name"]
        if not binary.is_file() or sha256(binary) != artifact["sha256"]:
            raise ValueError(f"Packaged service hash mismatch: {artifact['name']}")
        if binary.stat().st_size != artifact["size"] or not os.access(binary, os.X_OK):
            raise ValueError(f"Packaged service metadata mismatch: {artifact['name']}")

    runtime = json.loads(
        run([str(binaries / "sectile-companion"), "--local-runtime-info"], capture=True).stdout
    )
    port = runtime.get("port")
    if (
        runtime.get("protocol") != 1
        or not isinstance(port, int)
        or not 1 <= port <= 65535
        or port != LOCAL_RUNTIME_PORT
        or runtime.get("origin") != f"http://127.0.0.1:{port}"
    ):
        raise ValueError(f"Invalid packaged companion runtime contract in {app}")

    required = [
        resources / "THIRD_PARTY_NOTICES.md",
        resources / "THIRD_PARTY_NOTICES_GO.md",
        resources / "THIRD_PARTY_NOTICES_JS.md",
        resources / "THIRD_PARTY_NOTICES_RUST.html",
        resources / "third_party_licenses/go/sectile/LICENSE",
        resources / "third_party_licenses/tauri/LICENSE",
    ]
    for path in required:
        if not path.is_file() or path.stat().st_size == 0:
            raise ValueError(f"Required notice/license file is missing: {path.name}")

    license_count = sum(
        path.is_file()
        for path in (resources / "third_party_licenses").rglob("LICENSE*")
    )
    if license_count == 0:
        raise ValueError(f"No dependency license texts were packaged in {app}")
    go_license = (resources / "third_party_licenses/go/sectile/LICENSE").read_text(
        encoding="utf-8"
    )
    if not go_license.startswith("MIT License"):
        raise ValueError("The bundled Go source license is not MIT")
    return {"app": app.name, "architectures": architectures, "goPort": port, "licenseFiles": license_count}


def mounted_app(image, expected_manifest, config):
    run(["hdiutil", "verify", str(image)])
    attached = run(["hdiutil", "attach", "-readonly", "-nobrowse", "-plist", str(image)], capture=True)
    details = plistlib.loads(attached.stdout.encode())
    mounts = [
        pathlib.Path(entity["mount-point"])
        for entity in details.get("system-entities", [])
        if entity.get("mount-point")
    ]
    try:
        if len(mounts) != 1:
            raise ValueError(f"Expected one DMG mount point, found {len(mounts)}")
        mount = mounts[0]
        apps = list(mount.glob("*.app"))
        if len(apps) != 1 or apps[0].name != APP_NAME:
            raise ValueError("The DMG does not contain exactly one expected Sectile app")
        return verify_bundle(apps[0], expected_manifest, config)
    finally:
        for mount in mounts:
            run(["hdiutil", "detach", "-quiet", str(mount)])


def main():
    if sys.platform != "darwin":
        raise SystemExit("macOS bundle verification must run on macOS.")
    config = load_json(ROOT / "src-tauri/tauri.conf.json")
    expected_manifest = load_json(ROOT / "src-tauri/binaries/build-manifest.json")
    if expected_manifest["source"].get("dirty"):
        raise SystemExit("Refusing to package Go services from a dirty source tree.")
    app = ROOT / "src-tauri/target/release/bundle/macos" / APP_NAME
    image = ROOT / "src-tauri/target/release/bundle/dmg" / (
        f"Sectile Tauri_{config['version']}_aarch64.dmg"
    )
    if not app.is_dir() or not image.is_file():
        raise SystemExit("Build the macOS app and DMG before running package verification.")
    app_result = verify_bundle(app, expected_manifest, config)
    dmg_result = mounted_app(image, expected_manifest, config)
    print(json.dumps({
        "app": app_result,
        "dmg": dmg_result,
        "dmgBytes": image.stat().st_size,
        "dmgSha256": sha256(image),
        "goCommit": expected_manifest["source"]["commit"],
        "goDirty": expected_manifest["source"]["dirty"],
        "releaseEligible": expected_manifest["releaseEligible"],
    }, indent=2))


if __name__ == "__main__":
    main()
