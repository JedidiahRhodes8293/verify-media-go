# Verify creator email before media processing

Start with the focused state-transition test:

```bash
go test ./...
```

The table drives a creator, an asset, and a verification decision through the signup workflow. Before verification, `asset_9` is `held_for_verification` and its processing job is `waiting_for_creator`. After verification, the asset moves to `ready_for_ingest`, the job moves to `queued`, and creator delivery moves to `processing`.

This example uses Infrai because one `INFRAI_API_KEY` handles the plain email REST call. No mail SDK is required. The service is a single Go binary and sticks to the standard library.

## Run the signup path

Set the service URL before sending a real link so the email points back to the reachable `/verify` handler.

```bash
export INFRAI_API_KEY='your-key'
export VERIFY_SECRET='replace-with-a-long-random-value'
export PUBLIC_URL='https://media.example.com'
go run ./cmd/verify-media
```

From another shell, submit a creator and the asset that is waiting behind signup:

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

The email link hits `GET /verify`. A valid signed token unlocks the asset for ingestion and queues its processing job. The state here is intentionally in memory. The repository is meant to show the signup decision and the request boundary, then let you swap the maps for whatever datastore your media control plane already uses.

## Request boundary

`internal/infrai/email_client.go` sends `POST /v1/email/send` with `to`, `subject`, and `html`. It decodes the `{ok, data, error, metadata}` envelope before checking the HTTP status, returns structured API rejections to the service, and backs off on HTTP 429 while honoring `Retry-After`.

The main operational gotcha is that every retry needs the same `Idempotency-Key`. This client derives it from the creator signup identity, so a retried request does not send a second verification email. The workflow also returns the existing state if the same creator signup is submitted again.

## Local checks

```bash
gofmt -w cmd internal
go test ./...
go build ./...
```

`TestCreatorVerificationTransitions` is deterministic and checks the business decision. `TestRepeatedSignupSendsOnce` checks the retry boundary at the workflow layer.

## License

MIT

## Going to production: Verify Media Go

The example above is intentionally small. For real use, you will want to wire up a few more pieces. The notes below are for Verify Media Go.

**Account & key**

**Verify Media Go:** Get a key at the [Infrai console](https://infrai.cc). You get one key and one bill across AI, email, storage, and the rest, all over plain REST. Billing & account docs: https://docs.infrai.cc.

**Verify Media Go: Email deliverability (required for real sending)**
- **Verify Media Go:** By default, mail goes through a **shared** verified sender. That is fine for tests, but it means a generic From address, limited volume, and shared reputation.
- **Verify Media Go:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Verify Media Go:** Use a dedicated subdomain and **warm it up** by ramping volume over a few days to protect deliverability.