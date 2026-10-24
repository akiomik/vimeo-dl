# Synthetic download fixtures

These fixtures are hand-written test data. They are not copies or redactions of
any private Vimeo video, manifest, request, or media file. All identifiers,
paths, signatures, durations, and byte counts are invented. Valid base64 strings
decode to short non-media byte sequences.

- `master-unencoded-init.json` models the path-like `init_segment` form seen
  without `base64_init=1`. Its init strings are intentionally not base64.
- `repeated-parcel.json` models the `base64_init=1` form: independent video
  segments, but DASH audio segments with empty `url` values that all resolve to
  the same MP4 resource.
- `master-ranged.json` models the corresponding
  `base64_init=1&query_string_ranges=1` form, where DASH audio segments have
  distinct query-string byte ranges.
- `playlist-segments.json` models a manifest whose segments are separate `.m4s`
  requests. Its `pathsig` values are the literal placeholder `synthetic-only`.
- `html-response.html` is a made-up HTML response for testing manifest fetch
  failures, including a successful HTTP status with a non-JSON body.

The three `master` fixtures reproduce only structural differences observed in
local private JSON files. They deliberately reduce many segments and tracks to
two segments and one track of each kind; no identifying source values, source
segment counts, or media bytes are retained. `playlist-segments.json` is based
on public issue evidence, not on the private playback session.

Future tests should serve these files from a local HTTP server and set response
status and headers explicitly. No test should fetch a live Vimeo URL or need a
browser session, cookie, password, or video download.
