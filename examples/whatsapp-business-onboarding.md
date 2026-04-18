# WhatsApp Business onboarding (Meta Cloud API)

Enterprise rollout takes several days because of business verification.

1. Create a Meta Business Account if you don't have one.
2. Add a WhatsApp Business Account (WABA) → register a phone number you control.
3. Create a Meta App → add the **WhatsApp** product → request **whatsapp_business_messaging** permission.
4. Submit business verification (typical: 1–3 business days).
5. From the App dashboard, copy the **App Secret** and a **system-user permanent access token**.
6. Push them to Secret Manager:
   ```bash
   echo -n "$WA_APP_SECRET"   | gcloud secrets create whatsapp-app-secret --data-file=-
   echo -n "$WA_ACCESS_TOKEN" | gcloud secrets create whatsapp-access-token --data-file=-
   ```
7. **Webhook configuration** in the App dashboard:
   - Callback URL: `https://<your-ingress-domain>/v1/whatsapp`
   - Verify token: any string (Meta sends a GET with `hub.challenge` on first connect — ingress must echo it).
   - Subscribe to `messages` field.
8. Send a message to your number from any WhatsApp client; ingress verifies
   `X-Hub-Signature-256` against the App Secret.

## Compliance notes

- WhatsApp requires user-initiated conversations within a 24-hour window;
  outside that you must use pre-approved **message templates**. The emitter
  must surface a clear error when the agent tries to reply outside the window.
- Per-conversation pricing applies after Meta's free tier (1k conversations/month).
