# WhatsApp

Order Desk speaks to shops and to the owner on WhatsApp through Meta's **WhatsApp Cloud API**, with
the same flows as Telegram: pairing code, text / voice note / photo orders, questions as buttons,
confirmation into the ERP, and supplier invoices (photo or PDF) from the owner's number.

- Up to 3 choices are reply buttons, up to 10 a list, more a numbered message answered by its number.
- WhatsApp cannot edit a sent message: where Telegram edits the summary, WhatsApp gets a new message.
- A shop can only be messaged freely within 24 h of its last message (WhatsApp's rule); Order Desk
  only ever answers, so this does not bite.

## Settings (environment or a keys file)

```
WHATSAPP_TOKEN=...          # access token of the WhatsApp app (temporary from the console, or a system user token)
WHATSAPP_PHONE_ID=...       # "Phone number ID" shown in WhatsApp → API Setup (not the number itself)
WHATSAPP_VERIFY_TOKEN=...   # any secret string you choose; typed again in the webhook settings
WHATSAPP_APP_SECRET=...     # App settings → Basic → App secret (checks every incoming call is from Meta)
```

Several keys files can be given: `orderdesk -keys keys.env,whatsapp.env`.

## First test with Meta's free test number

1. developers.facebook.com → **Create app** → type *Business* → add the **WhatsApp** product.
2. WhatsApp → **API Setup**: Meta gives a test number. Copy the *temporary access token* and the
   *Phone number ID*. Under "To", add your own phone number (Meta sends it a code).
3. Start Order Desk with the four settings above.
4. Give it a public HTTPS address: `cloudflared tunnel --url http://127.0.0.1:8610` prints a
   `https://….trycloudflare.com` address (install: `winget install Cloudflare.cloudflared`).
5. WhatsApp → **Configuration** → Webhook: callback URL `https://….trycloudflare.com/whatsapp/webhook`,
   verify token = `WHATSAPP_VERIFY_TOKEN` → *Verify and save*; then subscribe to the **messages** field.
6. Panel → Customers → pairing code; send it from your phone to the test number; then order.
   Panel → Invoices → owner code, to send supplier invoices from another number.

## Without a WhatsApp account

`orderdesk-erps/fakegraph.py` stands in for the Graph API (records what the desk sends, serves media)
and `orderdesk-erps/wa_sim.py` plays a shop or the owner through the signed webhook:

```
python fakegraph.py 8699
WHATSAPP_GRAPH_URL=http://127.0.0.1:8699 WHATSAPP_TOKEN=TOKEN WHATSAPP_PHONE_ID=PHONE WHATSAPP_VERIFY_TOKEN=VERIFY WHATSAPP_APP_SECRET=SECRET orderdesk …
python wa_sim.py shop  http://127.0.0.1:8613 213555000111 "2 cartons ifri 1.5, 3 fanta 1l"
python wa_sim.py owner http://127.0.0.1:8614 213661000222 IMGINV1
```
