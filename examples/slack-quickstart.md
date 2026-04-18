# Slack quickstart

1. Create a Slack app at https://api.slack.com/apps → "From scratch".
2. **OAuth & Permissions** → add scopes: `chat:write`, `app_mentions:read`, `channels:history`, `groups:history`, `im:history`.
3. **Event Subscriptions** → enable; request URL = `https://<your-ingress-domain>/v1/slack`.
   - Subscribe to bot events: `app_mention`, `message.channels`, `message.im`.
4. Install the app to your workspace; copy the **Bot User OAuth Token** and **Signing Secret**.
5. Push them into Secret Manager:
   ```bash
   echo -n "$SLACK_SIGNING_SECRET" | gcloud secrets create slack-signing-secret --data-file=-
   echo -n "$SLACK_BOT_TOKEN"     | gcloud secrets create slack-bot-token --data-file=-
   ```
6. Grant the `sclawion-ingress` and `sclawion-emitter-slack` service accounts `secretmanager.secretAccessor` on each.
7. Mention the bot in any channel: `@sclawion build me a status dashboard for...`.
   The ingress verifies the signature, publishes to `sclawion.inbound`, the
   router spawns a Scion agent, and replies stream back into the same thread.
