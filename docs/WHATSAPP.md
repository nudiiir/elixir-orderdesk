# WhatsApp

Order Desk speaks to shops and to the owner on WhatsApp with the same flows as Telegram:

- the pairing code;
- orders as text, voice notes or photos;
- questions as buttons;
- confirmation into the ERP;
- supplier invoices (photo or PDF) sent from the owner's number.

Messages arrive either through Meta's **WhatsApp Cloud API** or through **Twilio**.

- Up to 3 choices are reply buttons, up to 10 a list, and more a numbered message answered by its number.
- WhatsApp cannot edit a sent message: where Telegram edits the summary, WhatsApp sends a new message.
- A business may only message a person freely within 24 h of that person's last message. Order Desk only
  ever answers, so this never blocks it.

There are three ways to try it, from the quickest to the real thing.

## 1. Without any WhatsApp account (local simulator)

`tools/whatsapp-sim` plays both sides on your computer:

- `fakegraph.py` stands in for Meta's Graph API: it records every message the desk sends (`outbox.jsonl`) and
  serves media from `media/`.
- `wa_sim.py` is a phone. It sends signed webhook calls as a shop or as the owner, reads the desk's replies
  and taps the buttons.

Use three terminals, all started from the repository root.

```bash
# terminal 1: the fake Graph API
python tools/whatsapp-sim/fakegraph.py 8699
```

```bash
# terminal 2: Order Desk wired to it (bash; in PowerShell set each variable with $env:NAME="value")
WHATSAPP_GRAPH_URL=http://127.0.0.1:8699 WHATSAPP_TOKEN=TOKEN WHATSAPP_PHONE_ID=PHONE \
WHATSAPP_VERIFY_TOKEN=VERIFY WHATSAPP_APP_SECRET=SECRET ./orderdesk -addr 127.0.0.1:8610 -data ./data
```

```bash
# terminal 3: a shop pairs, orders and confirms; then the owner sends the specimen invoice (media id IMGINV1)
python tools/whatsapp-sim/wa_sim.py shop  http://127.0.0.1:8610 213555000111 "2 cartons ifri 1.5, 3 selecto 2l"
python tools/whatsapp-sim/wa_sim.py owner http://127.0.0.1:8610 213661000222 IMGINV1
```

The shop run prints the conversation and ends with `RESULT: order booked` when the order is in the ERP. An
ERP must be connected first, and reading the invoice photo needs `META_MODEL_API_KEY`.

## 2. Twilio's WhatsApp Sandbox (no Meta business setup)

```
TWILIO_ACCOUNT_SID=AC...            # Twilio console → Account Info
TWILIO_AUTH_TOKEN=...               # Twilio console → Account Info (it also signs every webhook call)
TWILIO_WHATSAPP_FROM=+14155238886   # the sandbox number, or your approved WhatsApp sender
TWILIO_WEBHOOK_URL=https://<your tunnel>/whatsapp/webhook   # the exact public URL Twilio calls
```

1. Give Order Desk a public HTTPS address (see "Public address" below).
2. Twilio console → Messaging → Try it out → Send a WhatsApp message → **Sandbox settings**. Set "When a message
   comes in" to `https://<your tunnel>/whatsapp/webhook`, method POST.
3. Each phone first sends `join <your-code>` to the sandbox number.
4. In the panel, go to Customers → Pairing code. Send the code from the phone, then order.

When the Twilio settings are present, they are used instead of the Cloud API settings below.

## 3. Meta's WhatsApp Cloud API, with Meta's free test number

```
WHATSAPP_TOKEN=...          # access token of the WhatsApp app (temporary from the console, or a system user token)
WHATSAPP_PHONE_ID=...       # "Phone number ID" shown in WhatsApp → API Setup (not the number itself)
WHATSAPP_VERIFY_TOKEN=...   # any secret string you choose; typed again in the webhook settings
WHATSAPP_APP_SECRET=...     # App settings → Basic → App secret: required, it proves each call comes from Meta
```

1. developers.facebook.com → **Create app** → type *Business* → add the **WhatsApp** product.
2. WhatsApp → **API Setup**: Meta gives you a test number. Copy the *temporary access token* and the
   *Phone number ID*. Under "To", add your own phone number (Meta sends it a code).
3. Start Order Desk with the four settings above, in `.env` or a keys file (`-keys .env,whatsapp.env`).
4. Give it a public HTTPS address (see below).
5. WhatsApp → **Configuration** → Webhook. Set the callback URL to `https://<your tunnel>/whatsapp/webhook` and
   the verify token to your `WHATSAPP_VERIFY_TOKEN`, then *Verify and save*. Subscribe to the **messages** field.
6. In the panel, go to Customers → Pairing code. Send the code from your phone to the test number, then order.
   For supplier invoices, get the owner code from Invoices → owner code and send it from another number.

## Public address

Meta and Twilio need an HTTPS address that reaches your computer. A quick tunnel works:

```bash
cloudflared tunnel --url http://127.0.0.1:8610      # prints https://….trycloudflare.com
```

Install it with `winget install Cloudflare.cloudflared` (Windows) or `brew install cloudflared` (macOS).

**What a tunnel exposes.** The tunnel forwards every path, but Order Desk only serves `/whatsapp/webhook` to
requests that arrive through it. Each of those calls must carry Meta's or Twilio's signature. The panel and its
API refuse any request that does not come from this computer: they check the forwarding headers, the host
name, and the page's origin. Even so, stop the tunnel when you are done, and never share the panel's address.
