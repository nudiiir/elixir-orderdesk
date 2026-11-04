"""Generate the voice notes nobody recorded, with Gemini TTS, in Algerian Darja (seed 2030).

The script of each note is written in Algerian Darja in Arabic script (zouj, not jouj; ab3athli, kartonat,
ya3tik essaha), product names left in French as shops say them. Each file is saved as <order id>.wav next
to the recorded ones, and sources.json marks it "generated", so human and synthetic voices stay apart.
Orders that already have a voice note are never touched.

    python -I tts.py [--only ID,ID] [--dry]
"""
import os, sys, json, random, base64, wave, io, re, time, urllib.request
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).parent
OUT = Path(r"E:\invoices\orders-voice")
MODEL = "gemini-3.8-flash-tts"
VOICES = ["Charon", "Puck", "Fenrir", "Orus", "Kore", "Aoede", "Leda", "Algenib"]
R = random.Random(2030)
args = sys.argv[1:]
ONLY = set(args[args.index("--only") + 1].split(",")) if "--only" in args else None
DRY = "--dry" in args

NUM = {1: "واحد", 2: "زوج", 3: "ثلاثة", 4: "ربعة", 5: "خمسة", 6: "ستة", 7: "سبعة", 8: "ثمانية", 9: "تسعة", 10: "عشرة",
       12: "douze", 20: "vingt", 24: "vingt-quatre", 30: "trente", 50: "cinquante"}
UNIT = {"carton": ("كرطونة", "كرطونات"), "pack": ("فاردو", "فاردوات"),
        "unit": {"pharmacy": ("boîte", "boîtes"), "electronics": ("حبة", "حبات"), "cosmetics": ("حبة", "حبات"), "supermarket": ("حبة", "حبات")}}
SPOKEN = {"amp": "ampoules", "buv": "buvables", "eff": "effervescent", "sp": "sirop", "srp": "sirop", "supp": "suppositoires", "inj": "injectable",
          "pde": "pommade", "crm": "crème", "sol": "solution", "gttes": "gouttes", "bb": "bébé", "gm": "grand modèle", "pm": "petit modèle",
          "bl-tooth": "bluetooth", "t-c": "type C", "s-fil": "sans fil", "tonner": "toner"}
SILENT = {"cp", "cpr", "comp", "gles", "pdre", "sacht", "sach", "bt", "bte", "pel", "pelli", "enrob", "sec", "gast-res", "disp", "fl", "flc", "t"}

def name(s):
    s = " ".join(SPOKEN.get(w.lower(), w) for w in s.split() if w.lower() not in SILENT) or s
    s = re.sub(r"(?i)\bt/(?=\d)", "tube ", s)
    return re.sub(r"(\d+(?:\.\d+)?)/(\d+(?:\.\d+)?)", r"\1 \2", s).lower()

def unit(o, l):
    u = UNIT[l["unit"]]
    if isinstance(u, dict): u = u[o["industry"]]
    return u[0] if l["qty"] == 1 else u[1]

def script(o):
    items = [f"{NUM.get(l['qty'], str(l['qty']))} {unit(o, l)} {name(l['say'])}" for l in o["lines"]]
    hello = R.choice(["السلام عليكم خويا", "سلام عليكم", "صباح الخير خويا", "السلام عليكم، كيراك خويا"])
    ask = R.choice(["ابعثلي", "حضرلي", "نحب", "زيدني في الكوموند"])
    bye = R.choice(["يعطيك الصحة", "بارك الله فيك", "صحة خويا، ابعثهم اليوم إن شاء الله", "ربي يحفظك"])
    return f"{hello}، {ask} " + "، و".join(items) + f". {bye}."

# A short directive in the form Gemini TTS treats as direction ("Say ...:"). A longer paragraph was read aloud
# in 21 of 26 notes on the first run; trim_tts.py cut it off.
STYLE = "Say this in Algerian Darja with an Algiers accent, like a casual WhatsApp voice note: "

def tts(text, voice, key):
    body = {"contents": [{"parts": [{"text": STYLE + text}]}],
            "generationConfig": {"responseModalities": ["AUDIO"], "speechConfig": {"voiceConfig": {"prebuiltVoiceConfig": {"voiceName": voice}}}}}
    req = urllib.request.Request(f"https://generativelanguage.googleapis.com/v1beta/models/{MODEL}:generateContent",
                                 data=json.dumps(body).encode(), headers={"x-goog-api-key": key, "Content-Type": "application/json"})
    r = json.load(urllib.request.urlopen(req, timeout=180))
    part = r["candidates"][0]["content"]["parts"][0]["inlineData"]
    pcm = base64.b64decode(part["data"]); rate = int(re.search(r"rate=(\d+)", part.get("mimeType", "")).group(1)) if "rate=" in part.get("mimeType", "") else 24000
    buf = io.BytesIO()
    with wave.open(buf, "wb") as w: w.setnchannels(1); w.setsampwidth(2); w.setframerate(rate); w.writeframes(pcm)
    return buf.getvalue(), len(pcm) / 2 / rate

orders = json.load(open(HERE / "orders.json", encoding="utf-8"))
have = {p.stem for p in OUT.iterdir() if p.suffix != ".json"}
todo = [o for o in orders if o["channel"] == "voice" and o["id"] not in have and (ONLY is None or o["id"] in ONLY)]
key = None if DRY else os.environ["GEMINI_API_KEY"]
srcp = OUT / "sources.json"; src = json.loads(srcp.read_text(encoding="utf-8")) if srcp.exists() else {}
for o in todo:
    text, voice = script(o), R.choice(VOICES)
    o["voice_script"], o["voice_lang"] = text, "darja_algerian"
    if DRY: print(o["id"], voice, "|", text); continue
    for attempt in range(3):
        try:
            wav, secs = tts(text, voice, key); break
        except Exception as e:
            print(o["id"], "retry", attempt + 1, type(e).__name__, str(e)[:120]); time.sleep(5)
    else:
        continue
    (OUT / f"{o['id']}.wav").write_bytes(wav)
    src[o["id"]] = f"generated: {MODEL}, voice {voice}"
    srcp.write_text(json.dumps(src, ensure_ascii=False, indent=1), encoding="utf-8")
    print(o["id"], voice, f"{secs:.0f}s", flush=True)
(HERE / "orders.json").write_text(json.dumps(orders, ensure_ascii=False, indent=1), encoding="utf-8")
for ind in ["supermarket", "pharmacy", "cosmetics", "electronics"]:
    (HERE / f"orders-{ind}.json").write_text(json.dumps([o for o in orders if o["industry"] == ind], ensure_ascii=False, indent=1), encoding="utf-8")
