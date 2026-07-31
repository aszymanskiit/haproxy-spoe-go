# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

## [1.0.8] - 2026-07-31

### Features

- Support for decoding IPv6 addresses (9e3b6d6)

- Max connection duration (f55a162)


### Other

- Initial (1c3fe63)

- Add changelog (49dfa32)

- Append a missed return statement (16a2443)

- Add some tests (e8b19bd)

- Add github workflow for tests (b52d865)

- Add badges to readme (26459cb)

- Create LICENSE (0bf2296)

- Update readme (e488977)

- Update readme.md (b641b8d)

- Remove debug print (bf7d9c7)

- Add a basic spop client to be used in tests (10d9242)

- Add a test with concurrency to exhibit race condition error (5dc4542)

- Add a makefile to ease tests running (2740b16)

- Remove race condition (e5b304b)

- Disable pipelining in client (46bdff4)

- Add comments to Client (52e0ec9)

- Return error if trying to encode Message (8b2fade)

- Return error in case of frameid or streamid mismatch (af4895a)

- By default frame don't have actions and doesn't have ownership on it (d93a08a)

- Add simple bench with client exhib memory issue (adcf463)

- Move buffer out of loop to reduce mem usage (0ebbda0)

- Fix random error under load (39d366a)

- Add tests for frame read/encode (35e442d)

- Use plain Action array rather than array of pooled pointers (4203a8a)

- Introduce generic logger interface (ed3387f)

- Update to Go 1.19 (78e4d31)

- Fix typo in readme.md (9a8c9fb)

- Some refactoring and remove dependencies (dcff814)

- `false` value has incorrect encoding value - invert flag and value (56dca65)

- Wait for in-flight Notify handlers before closing (c6fc777)

- Bound SPOP frame reads and harden binary parsers (c0dc0bc)

- Trigger GitHub Actions (f9042da)

- Use maxframesize announce by haproxy (74df2a1)


### Bug Fixes

- Fix bug for decode binary data (d44e5d5)

- Fix encode binary data, add length (6bdf2f4)

- **worker:** Treat connection reset as normal close (d9cefa4)

- **typeddata:** Use 10-byte varint buffer for 64-bit values (0df5ad7)

- **ci:** Fix git-cliff crash on first release due to tag name passed as OID range (1872e91)

- **ci:** Fix git-cliff (6df573c)


### Documentation

- Refresh README, examples, and CI for the maintained fork (db1d41f)

- Update README.md (0a75e5a)


### Tests

- Test frame type before allocating buffers to avoid potential OOM, use static scratch buffers to reduce allocation (27bf99d)


### CI/CD

- Add Conventional Commits validation and release changelog automation (da5dab2)

- Create RELEASE_NOTES.md (a58a150)


### Miscellaneous

- Reuse read buffer across pooled frames to reduce allocation churn (c47b4cf)

- **go:** Upgrade project to Go 1.25 (9d29a60)

- Remove duplicated readme.md (9c6fc57)

