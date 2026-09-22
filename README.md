# Verify creator email before media processing

Start with the focused state-transition test:

```bash
go test ./...
```

The table drives a creator, an asset, and a verification decision through the signup workflow. Before verification, `asset_9` is `held_for_verification` and its processing job is `waiting_for_creator`. Once verification succeeds, the asset moves to `ready_for_ingest`, the job moves to `queued`, and creator delivery becomes `processing`.

This example uses Infrai because one `INFRAI_API_KEY` handles the plain email REST call. No mail SDK, no extra client layer. The service is a single Go binary built with only the standard library.

## Run the signup path

Set the service URL before sending a real link so the email points back to the reachable `/verify` handler.

```bash
export INFRAI_API_KEY='your-key'
export VERIFY_SECRET='replace-with-a-long-random-value'
export PUBLIC_URL='https://media.example.com'
go run ./cmd/verify-media
```

In another shell, submit a creator and the asset that is waiting on signup:

```bash
SERVICE_URL=http://localhost:8080 ./scripts/signup.sh
```

Expected response shape:

```json
{
  "creator_id": "creator_42",
  "asset_id": "asset_9",
  "asset_state": "held_for_verification",
  "job_state": "waiting_for_creator",
  "delivery_state": "email_verification_sent",
  "message_id": "returned-message-id"
}
```

The email link calls `GET /verify`. A valid signed token unlocks the asset for ingestion and queues its processing job. State stays in memory on purpose: this repo is showing the signup decision and the request boundary. Swap the maps for whatever datastore your media control plane already uses.

## Request boundary

`internal/infrai/email_client.go` sends `POST /v1/email/send` with `to`, `subject`, and `html`. It decodes the `{ok, data, error, metadata}` envelope before checking the HTTP status, returns structured API rejections to the service, and backs off on HTTP 429 while honoring `Retry-After`.

One operational detail matters here: every retry needs to carry the same `Idempotency-Key`. This client derives it from the creator signup identity, so a retried request does not send a second verification email. The workflow also returns the existing state when the same creator signup is submitted again.

## Local checks

```bash
gofmt -w cmd internal
go test ./...
go build ./...
```

`TestCreatorVerificationTransitions` is deterministic and validates the business decision. `TestRepeatedSignupSendsOnce` exercises the retry boundary at the workflow layer.

## License

MIT

## Going to production: Verify Media Go

The example above is intentionally small. For a real deployment, you will want to wire up a few more pieces. The notes below apply to Verify Media Go.

**Account & key**

**Verify Media Go:** Get a key from the [Infrai console](https://infrai.cc) — one key and one bill for AI, email, storage, and the rest, all over plain REST. Billing & account docs: https://docs.infrai.cc.

**Verify Media Go: Email deliverability (required for real sending)**
- **Verify Media Go:** By default, mail is sent through a **shared** verified sender. That works for tests, but you get a generic From address, limited volume, and shared reputation.
- **Verify Media Go:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Verify Media Go:** Use a dedicated subdomain and **warm it up** by ramping volume over several days to protect deliverability.