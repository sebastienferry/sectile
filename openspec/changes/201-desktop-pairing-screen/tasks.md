# Tasks

The implementation is already on `main`. These tasks verify it against the specification rather
than write it; any one that fails is a real gap to fix.

## 1. Verify the shipped behaviour
- [ ] 1.1 Read `desktop/src/main.js` and confirm the connect form carries a `Pairing code` field and
      the API key behind an `Advanced` disclosure, with no required `Server token`.
- [ ] 1.2 Read `desktop/electron/pairing.cjs` and confirm `resolveConnectCredential` gives the code
      priority and raises a message naming both ways in when neither is filled.
- [ ] 1.3 Read `desktop/electron/main.cjs` and confirm the URL is validated before the credential is
      resolved, and that `storeKey` persists with and without a keyring.
- [ ] 1.4 Run `cd desktop && node --test tests/*.test.cjs` and `npm run test:ui`; note that the UI
      tests need `npx vite build` first, since they load `dist/index.html`.

## 2. Close any gap the verification exposes
- [ ] 2.1 For each scenario of `specs/desktop-pairing/spec.md` with no covering test, add one.
- [ ] 2.2 Fix any behaviour that diverges from the specification, in the smallest possible change.

## 3. Manual end-to-end validation — OPEN, owner's call
- [ ] 3.1 Run the packaged desktop application against a real server, pair with a fresh code from the
      web profile, close the application and relaunch it; confirm the second launch connects without
      asking for a code. Automated tests cover the exchange against a stub server only.
