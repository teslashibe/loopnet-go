# loopnet-go

Private Go client + MCP server for [www.loopnet.com](https://www.loopnet.com)
(CoStar Group). There is no public end-user API.

## Bot protection — read this first

LoopNet is behind **Akamai Bot Manager**. Plain `curl` against
`/`, `/account/login/`, or any other path returns `HTTP 403
AkamaiGHost` immediately. Akamai's `_abck` / `bm_sz` / `ak_bmsc`
cookies cannot be produced by a non-browser caller.

**Working path:** paste your **entire** browser `cookie:` request
header (including the Akamai cookies AND the LoopNet auth cookies
set after login). Use Chrome → log in at https://www.loopnet.com →
DevTools → Network → any request to `www.loopnet.com` → Request
Headers → copy `cookie:` → paste into `LOOPNET_COOKIE_HEADER` (or
`~/.loopnet-mcp/config.json`). Also set `LOOPNET_USER_AGENT` to the
exact UA from the same browser tab — Akamai fingerprints UA + cookie
together.

The pasted cookie set typically remains valid for several hours.
When it expires, login probes return `ErrBotChallenge`; refresh.

## Operational caveat

CoStar (LoopNet's owner) actively monitors and litigates automated
access. This client is intentionally:

- **Read-only** (no writes, no booking, no broadcasting)
- **Rate-limited** at one request per 1.5s by default
- **Scoped to your own authenticated account** (no wide-area
  listing-search scraping endpoints are wrapped)

Do not raise the rate limit, parallelize, or use this against
accounts you do not own.

## Supported operations (v0.1)

| Area  | Methods                              |
|-------|--------------------------------------|
| Auth  | `Login`, `GetMe`, `AuthSnapshot`     |
| Pages | `GetPath`                            |

Useful `GetPath` values:

- `/myloopnet/`
- `/myloopnet/saved-searches/`
- `/myloopnet/saved-listings/`
- `/myloopnet/saved-comparables/`
- `/myloopnet/dashboard/activity`
- `/myloopnet/messages`
- `/myloopnet/profile/`
- `/account/preferences/`

## TODO

- [ ] Typed parsers for saved-searches / saved-listings dashboards.
- [ ] Identify the JSON XHR endpoints fired by /myloopnet/* (DevTools
      → Network → XHR filter while clicking through dashboards) and
      add typed wrappers around them — much cleaner than HTML scrape.
- [ ] Cookie freshness probe + clear error when `_abck` expires.

## MCP server

```bash
go install github.com/teslashibe/loopnet-go/cmd/loopnet-mcp@latest
```

Register with Cursor:

```json
{
  "mcpServers": {
    "loopnet": { "command": "/Users/you/go/bin/loopnet-mcp" }
  }
}
```

## License

Private. Internal use only.
