---
name: openclaw
description: Lets a Scion agent publish events to sclawion.outbound so they appear in the originating chat thread. Use when an agent wants to send an interim update, ask the user a question, or signal completion to a Slack/Discord/Google Chat/WhatsApp conversation that spawned it.
---

# openclaw skill

You are running inside a Scion agent that was started in response to a chat
message. The originating channel and thread metadata is in the env var
`SCLAWION_CONVERSATION_ID`.

To send a message back to the user, write a CloudEvent-shaped JSON to stdout
prefixed with `<<sclawion-event>>` on its own line. Example:

```
<<sclawion-event>>
{"kind":"agent.reply","text":"Working on it. ETA 2 minutes."}
```

The scion-bridge picks these up from your log stream, normalizes them into an
Envelope, and publishes to `sclawion.outbound`. The platform-specific emitter
posts the message back to your originating thread.

Lifecycle events (`agent.started`, `agent.completed`, `agent.failed`) are
emitted automatically — you do not need to send them.
