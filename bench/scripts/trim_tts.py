"""Cut the English instruction Gemini read aloud at the start of the generated voice notes. Local only.

The instruction is the same text in every file and ends "...product or quantity. Text:". Its end is
predicted from text lengths, then snapped to the pause that follows the short word "Text": a long
pause, a burst of under 1.2 s, then a pause. Ties go to the later cut (losing part of the greeting is
harmless, losing a product is not). Originals stay in orders-voice-raw.

    python trim_tts.py
"""
import wave, json, sys, numpy as np
from pathlib import Path
sys.stdout.reconfigure(encoding="utf-8")
HERE = Path(__file__).parent; RAW = Path(r"E:\invoices\orders-voice-raw"); OUT = Path(r"E:\invoices\orders-voice")
STYLE_LEN = 331  # characters of the instruction that was read
orders = {o["id"]: o for o in json.load(open(HERE / "orders.json", encoding="utf-8"))}
src = json.loads((OUT / "sources.json").read_text(encoding="utf-8"))
log = {}
for oid, s in sorted(src.items()):
    if not s.startswith("generated") or not (RAW / f"{oid}.wav").exists(): continue
    w = wave.open(str(RAW / f"{oid}.wav")); sr = w.getframerate(); x = np.frombuffer(w.readframes(w.getnframes()), np.int16)
    f = int(sr * 0.02); n = len(x) // f; e = np.sqrt((x[:n * f].astype(np.float32).reshape(n, f) ** 2).mean(1))
    thr = max(np.percentile(e, 20) * 2.5, 150); sil = e < thr
    P = []; i = 0
    while i < n:
        if sil[i]:
            j = i
            while j < n and sil[j]: j += 1
            if (j - i) * 0.02 >= 0.25: P.append((i * 0.02, (j - i) * 0.02))
            i = j
        else: i += 1
    T = n * 0.02; pred = T * STYLE_LEN / (STYLE_LEN + len(orders[oid]["voice_script"]))
    cands = []
    for a, b in zip(P, P[1:]):
        burst = b[0] - (a[0] + a[1])
        if a[1] >= 0.55 and 0.15 <= burst <= 1.2 and pred - 7 <= b[0] <= pred + 2:
            cands.append((abs(b[0] + b[1] - (pred - 1)), b[0] + b[1]))
    if not cands:
        print(oid, "NO CUT FOUND, left as is", f"pred {pred:.1f}s"); continue
    cut = max(c for d, c in cands if d <= min(cands)[0] + 1.5)  # the later of the near-best cuts
    keep = x[int((cut - 0.05) * sr):]
    with wave.open(str(OUT / f"{oid}.wav"), "wb") as o:
        o.setnchannels(1); o.setsampwidth(2); o.setframerate(sr); o.writeframes(keep.tobytes())
    log[oid] = {"cut_at": round(cut, 2), "predicted": round(pred, 2), "kept_s": round(len(keep) / sr, 1)}
    print(oid, f"cut at {cut:.2f}s (predicted {pred:.1f}s), kept {len(keep) / sr:.1f}s")
(OUT / "trim-log.json").write_text(json.dumps(log, indent=1), encoding="utf-8")
