# What Muse Spark identified: mobile phones and accessories

Names are brand plus type plus model code plus key specs, word order varies. Sizes are cable lengths in M, screen sizes in inch with " , power in W/V/A, capacity in MAH/GB/TB, memory as RAM/storage like 12/256GB or 8+16/256GB, tip sizes like 4.5*3.0. Packs are not carton counts, codes like BW29 BG3 KSC-467 SK427 identify the exact reference. Two products of one brand differ by model code, memory mem token, length/power/capacity number+unit, connector TYPE-C/LIGHTING/MICRO, or quality ORIGINAL/COPY.

## First-pass blocks

- **inch** ON — tablet and glass screen sizes with inch sign 8" 10.2 must normalize; bare false so memory ratios like 12/256GB are never read as inches
  - evidence: reads 5 catalogue names differently; products that read alike 34 → 34
- **memory** ON — phones tablets PCs differ only by RAM/storage written 12/256GB 8+256 12+12/256GB 4+3/128GB, must become one mem token
  - evidence: reads 130 catalogue names differently; products that read alike 35 → 34
- **units_glue** ON — lengths 1.5M 20M, power 60W 19V 3.42A, capacity 20000MAH 300MBPS, width 22MM, 650VA all identify the product
  - evidence: reads 4 catalogue names differently; products that read alike 34 → 34

## The desk's own vocabulary (lexicon)

- `org` → `original` [rejected] — catalogue writes ORIGINAL but lines write ORG for the same quality
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `orig` → `original` [rejected] — catalogue writes ORIGINAL and ORIG interchangeably
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `bluetooth` → `bl-tooth` [live] — catalogue writes BL-TOOTH, suppliers write bluetooth
  - evidence: seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « BLUETOOTH AIRPODS JOYROOM BB3 » → BL-TOOTH AIRPODS JOYROOM BB3; « BLUETOOTH AIR-3 BUDS » → BL-TOOTH AIR-3 BUDS
- `bt` → `bl-tooth` [rejected] — suppliers write BT for the catalogue's BL-TOOTH
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `typec` → `type-c` [rejected] — suppliers glue TYPE-C as TYPEC, catalogue writes TYPE-C/T-C
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `smartphone` → `mobile` [live] — catalogue says MOBILE not SMARTPHONE
  - evidence: seen lines: +5 newly resolved (5 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « SMARTPHONE Y7 PRIME 4+64 HUAWEI » → MOBILE HUAWEI Y7 PRIME 4/64GB; « SMARTPHONE REALME 8 PRO 6-128 » → MOBILE REALME 8 PRO 6/128GB; « SUPPORT SMARTPHONE KSC-761B KAKUSIGA » → SUPPORT MOBILE KAKUSIGA KSC-761B
- `tel` → `mobile` [live] — TEL SAMSUNG -> MOBILE SAMSUNG
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost
- `tab` → `tablette` [live] — TAB ATOUCH -> TABLETTE ATOUCH
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « TAB ATOUCH 5G 4-128 S13 » → TABLETTE ATOUCH 5G 4/128GB S13
- `notebook` → `laptop` [live] — NOTEBOOK HP -> LAPTOP HP
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « NOTEBOOK HP ELITBOOK 840 G2 I5 5EME 8/256SSD » → LAPTOP HP ELITBOOK 840 G2 I5 5EME 8/256SSD
- `portable` → `laptop` [rejected] — PC PORTABLE -> LAPTOP, keep laptop token
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `pc` → `` [rejected] — drop PC when paired with PORTABLE, avoids extra word blocking LAPTOP match
  - evidence: refused: catalogue names use this word; dropping it would erase a difference between products
- `charger` → `chargeur` [live] — normalize both sides to CHARGEUR, catalogue uses both spellings
  - evidence: seen lines: +7 newly resolved (7 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « CHARGER LAPTOP UNIVERSEL 120W » → CHARGEUR LAPTOP UNIVERSEL 120W; « CHARGEUR IPHONE 14PRO RECUPIRATION ORG » → CHARGER IPHONE 14PRO RECUPIRATION ORG; « CHARGER ACEFAST 4IN1 60W AUTO » → CHARGEUR AUTO ACEFAST 4IN1 60W
- `cbl` → `cable` [live] — supplier abbrev CBL for CABLE
  - evidence: seen lines: +4 newly resolved (4 distinct wordings), 0 wrong, 0 lost; unseen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; e.g. « CBL X66 HOCO » → CABLE HOCO X66; « CBL REALME LUNA ORIG » → CABLE REALME LUNA ORIG; « CBL HOCO U14 » → CABLE HOCO U14
- `usb-c` → `type-c` [rejected] — supplier USB-C = catalogue TYPE-C / T-C
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `t-c` → `type-c` [rejected] — catalogue T-C = TYPE-C
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `coque` → `pochette` [rejected] — COQUE XUNDO -> POCHETTE XUNDO
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « COQUE XUNDO SIMPLE IPHONE » → POCHETTE IPHONE XUNDO SIMPLE
- `verre` → `glass` [rejected] — VERRE TREMPE -> GLASS
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `trempe` → `` [rejected] — never in catalogue, drop
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `protection` → `glass` [rejected] — PROTECTION ECRAN -> GLASS
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `ecran` → `` [rejected] — never in catalogue, drop
  - evidence: refused: catalogue names use this word; dropping it would erase a difference between products
- `mouse` → `souris` [live] — MOUSE -> SOURIS
  - evidence: seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « MOUSE USB CAPSYS-CS345 » → SOURIS USB CAPSYS-CS345; « MOUSE HAVIT S-FIL MS76GT PLUS RECHAREGABLE » → SOURIS HAVIT SANS-FIL MS76GT PLUS RECHAREGABLE; « MOUSE GAMER PROGRAMMABLE MS900S RGB HAVIT » → SOURIS HAVIT GAMER PROGRAMMABLE MS900S RGB
- `speaker` → `baffle` [live] — SPEAKER -> BAFFLE
  - evidence: seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « SPEAKER HAVIT 2.0 HV-SK427 » → BAFFLE HAVIT 2.0 HV-SK427; « SPEAKER BL-TOOTH HAINO TEKO S18 RGB » → BAFFLE BL-TOOTH HAINO TEKO S18 RGB
- `holder` → `support` [live] — HOLDER -> SUPPORT
  - evidence: seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « HOLDER AUTO HOCO CA63 » → SUPPORT AUTO HOCO CA63; « HOLDER BOROFONE AUTO BH52 » → SUPPORT BOROFONE AUTO BH52; « HOLDER BOROFONE AUTO BH52 » → SUPPORT BOROFONE AUTO BH52
- `headset` → `casque` [rejected] — HEADSET -> CASQUE
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « HEADSET BL-TOOTH CAT-EAR B39M » → CASQUE BL-TOOTH CAT-EAR B39M
- `watch` → `montre` [rejected] — WATCH -> MONTRE
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « WATCH PATEK PHILIPPE AA HOMME » → MONTRE HOMME PATEK PHILIPPE AA
- `core` → `` [live] — CORE I5/I7 never in catalogue
  - evidence: seen lines: +4 newly resolved (4 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « THINKPAD L13 CORE I5 13TH 16/512SSD LENOVO » → LENOVO THINKPAD L13 I5 13TH 16/512SSD; « NOTEBOOK HP ELITBOOK 650 G10 CORE I5 13TH 16/512SSD » → LAPTOP HP ELITBOOK 650 G10 I5 13TH 16/512SSD; « DELL LATITUDE 9430 CORE I7 12G 16/512SSD TACTILE 360° FISSURE » → DELL LATITUDE 9430 I7 12TH 16/512SSD TACTILE 360° FISSURE
- `galaxy` → `` [rejected] — GALAXY extra vs catalogue M32 S24 without it
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `xiaomi` → `` [rejected] — XIAOMI prefix not in REDMI catalogue names
  - evidence: refused: catalogue names use this word; dropping it would erase a difference between products
- `11eme` → `11th` [rejected] — 11EME = 11TH
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « HP 17 I5 11EME 16-512 SSD LAPTOP » → LAPTOP HP 17 I5 11TH 16/512 SSD
- `8eme` → `8th` [rejected] — 8EME = 8TH
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `s-fil` → `sans-fil` [rejected] — S-FIL = SANS-FIL
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `etui` → `pochette` [live] — ETUI OPPO/BELKIN/IPHONE -> POCHETTE
  - evidence: seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « ETUI OPPO XUNDO SIMPLE » → POCHETTE OPPO XUNDO SIMPLE; « ETUI IPHONE 14 PRO MAX XUNDO NEW » → POCHETTE IPHONE 14 PRO MAX XUNDO NEW; « ETUI BELKIN IPHONE » → POCHETTE IPHONE BELKIN
- `keyboard` → `clavier` [live] — KEYBOARD GAMER -> CLAVIER HAVIT/GAMING
  - evidence: seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « KEYBOARD GAMER KB275L HAVIT » → CLAVIER HAVIT GAMER KB275L; « KEYBOARD + SOURIS GAMING LED V100 » → CLAVIER + SOURIS GAMING LED V100; « KEYBOARD S-F HAVIT KB250G » → CLAVIER S-F HAVIT KB250G
- `haut` → `baffle` [rejected] — HAUT PARLEUR HC21 -> BAFFLE HOCO HC21
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `parleur` → `` [rejected] — drop second word of HAUT PARLEUR
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `usb` → `` [rejected] — USB-C vs T-C / MICRO USB vs MICRO, USB never discriminates
  - evidence: refused: catalogue names use this word; dropping it would erase a difference between products
- `v8` → `micro` [rejected] — V8 YESIDO/CHARGEUR V8 HP -> MICRO YESIDO/CHARGEUR MICRO HP
  - evidence: refused by the digit guard: it would change the digits of a word
- `cle` → `flash` [rejected] — CLE USB HOCO DISK -> FLASH DISK HOCO
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `10eme` → `10th` [rejected] — 10EME vs 10TH laptop generations
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « DELL LATITUDE 5310 I5 10EME 16-256 LAPTOP » → LAPTOP DELL LATITUDE 5310 I5 10TH 16/256GB
- `sam` → `samsung` [rejected] — SAM = SAMSUNG
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `t` → `type` [live] — T-C split to t c must match TYPE C
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « PORT CHARGEUR TYPE C SAM 25W ORG » → PORT CHARGER T-C SAMSUNG 25W ORG
- `10gb` → `` [rejected] — grade token blocking S10 5G match
  - evidence: refused by the digit guard: it would drop a word with digits (a code or a size; codes are learned per supplier in the code table)

## Search method

**BM25** (BM25)

## Rules (rule engineer and attributes)

- R-0009 [shadow] "galaxy" describes a product but never tells two products apart. — by rule-engineer
- R-0008 [shadow] "haut parleur" means "baffle". — by rule-engineer
- R-0007 [shadow] "verre trempe" means "glass". — by rule-engineer
- R-0006 [rejected] "coque" means "pochette". — by rule-engineer (the breaker wrote "COQUE XUNDO NEW IPHONE 12PRO MAX" (meaning POCHETTE IPHONE 12 PRO MAX XUNDO NEW) and the rule booked it as POCHETTE IPHONE 12/12PRO XUNDO NEW)
- R-0005 [shadow] "pc portable" means "laptop". — by rule-engineer
- R-0004 [shadow] "headset" means "casque". — by rule-engineer
- R-0003 [rejected] "rgb", "gaming", "privacy" each name a light, in orders, invoices and catalogue names alike; a light stated on one side only, or a different one, is never the same product. — by muse-spark-profile (a marker needs a plain product beside the variant: 25 of 28 products it marks have none (e.g. BAFFLE BL-TOOTH HAINO TEKO S18 RGB))
- R-0002 [live] "pro", "max", "plus", "ultra" each name a version, in orders, invoices and catalogue names alike; two texts whose version differs are never the same product. — by muse-spark-profile
- R-0001 [live] "original", "copy", "recupiration" each name a quality, in orders, invoices and catalogue names alike; two texts whose quality differs are never the same product. — by muse-spark-profile

## Phases

| phase | invoices | lines | free (alias + rules) | AI | asked | wrong auto | Spark agreed with code | Spark alone right | AI cost |
|---|---|---|---|---|---|---|---|---|---|
| spark-checks-all | 20 | 414 | 278 (67 %) | 96 (23 %) | 40 (10 %) | 0 | 278/278 | 96/96 | $0.1373 |
| learning | 10 | 183 | 133 (73 %) | 22 (12 %) | 28 (15 %) | 0 | 14/14 | 22/22 | $0.0360 |
| test after 0 | 10 | 209 | 124 (59 %) | 59 (28 %) | 26 (12 %) | 0 | 124/124 | 59/59 | $0.0634 |
| test after 10 | 10 | 209 | 152 (73 %) | 31 (15 %) | 26 (12 %) | 0 | 152/152 | 31/31 | $0.0669 |
| test after 20 | 10 | 209 | 157 (75 %) | 26 (12 %) | 26 (12 %) | 0 | 11/11 | 26/26 | $0.0247 |
| test after 30 | 10 | 209 | 166 (79 %) | 18 (9 %) | 25 (12 %) | 0 | 9/9 | 18/18 | $0.0200 |

## Calibrations

- **calibration after 10 invoices** (162 seen + 54 unseen lines): 100/162 right with no AI (62 %), 0 wrong → 123/162 right with no AI (76 %), 0 wrong; accepted 9, rejected 26; search: BM25 (brand ×2) + 0.5·trigrams: right product in the top 8 for 93.2 % of lines (MRR 0.93) → BM25: 93.2 % in the top 8 (MRR 0.93); engineer rules []; $0.1935. Spark: unresolved are pure synonyms and extra words, catalogue uses MOBILE GLASS BAFFLE SOURIS SUPPORT MONTRE CHARGEUR/CHARGER TYPE-C, plus droppable CORE GALAXY XIAOMI TREMPE ECRAN PORTABLE PC
- **calibration after 20 invoices** (310 seen + 104 unseen lines): 236/310 right with no AI (76 %), 0 wrong → 244/310 right with no AI (79 %), 0 wrong; accepted 2, rejected 11; search: BM25: right product in the top 8 for 94.2 % of lines (MRR 0.94) → BM25: 94.2 % in the top 8 (MRR 0.94); engineer rules []; $0.1354. Spark: suppliers write French generic words while catalogue uses GLASS/POCHETTE/MOBILE/CASQUE/CLAVIER/BAFFLE/MONTRE/FLASH and omits GALAXY/XIAOMI/USB/ECRAN, plus V8=MICRO and EME=TH generations
- **calibration after 30 invoices** (447 seen + 150 unseen lines): 350/447 right with no AI (78 %), 0 wrong → 358/447 right with no AI (80 %), 0 wrong; accepted 3, rejected 4; search: BM25: right product in the top 8 for 94.0 % of lines (MRR 0.94) → BM25: 94.0 % in the top 8 (MRR 0.94); engineer rules [R-0004 R-0005 R-0007 R-0008 R-0009]; $0.5262. Spark: glass/pochette/casque synonyms plus galaxy/xiaomi/usb drops and ordinals block most auto-match failures, partial memory needs fuzzy search for AI pick

## Experiment log

- [bootstrap] block · accepted: memory (reads 130 catalogue names differently; products that read alike 35 → 34, by muse-spark) — phones tablets PCs differ only by RAM/storage written 12/256GB 8+256 12+12/256GB 4+3/128GB, must become one mem token
- [bootstrap] block · accepted: units_glue (reads 4 catalogue names differently; products that read alike 34 → 34, by muse-spark) — lengths 1.5M 20M, power 60W 19V 3.42A, capacity 20000MAH 300MBPS, width 22MM, 650VA all identify the product
- [bootstrap] block · accepted: inch (reads 5 catalogue names differently; products that read alike 34 → 34, by muse-spark) — tablet and glass screen sizes with inch sign 8" 10.2 must normalize; bare false so memory ratios like 12/256GB are never read as inches
- [bootstrap] lexicon · accepted: "org" → "original" (proposed before any invoice (reads 11 catalogue names differently; products that read alike 34 → 34): waits in shadow until known lines prove it, by muse-spark) — catalogue writes ORIGINAL but lines write ORG for the same quality
- [bootstrap] lexicon · accepted: "orig" → "original" (proposed before any invoice (reads 4 catalogue names differently; products that read alike 34 → 34): waits in shadow until known lines prove it, by muse-spark) — catalogue writes ORIGINAL and ORIG interchangeably
- [bootstrap] lexicon · accepted: "bluetooth" → "bl-tooth" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 34 → 34): waits in shadow until known lines prove it, by muse-spark) — catalogue writes BL-TOOTH, suppliers write bluetooth
- [bootstrap] lexicon · accepted: "bt" → "bl-tooth" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 34 → 34): waits in shadow until known lines prove it, by muse-spark) — suppliers write BT for the catalogue's BL-TOOTH
- [bootstrap] lexicon · accepted: "typec" → "type-c" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 34 → 34): waits in shadow until known lines prove it, by muse-spark) — suppliers glue TYPE-C as TYPEC, catalogue writes TYPE-C/T-C
- [bootstrap] search · accepted: BM25 (brand ×2) + 0.5·trigrams (not measured yet: no line has a known answer, by muse-spark) — rare model codes BW29 BG3 KSC-467 drive identity via bm25, brand disambiguates BOROFONE/HOCO/XUNDO glass, trigram catches glued 11PRO+ vs 11 PRO + and typos, no wordsim to avoid confusing close codes
- [bootstrap] attribute · accepted: "original", "copy", "recupiration" each name a quality, in orders, invoices and catalogue names alike; two texts whose quality differs are never the same product. (reads 27 catalogue names, 0 pairs made alike → shadow, by muse-spark) — same iphone/charger model in ORIGINAL vs COPY vs RECUPIRATION is a different product
- [bootstrap] attribute · accepted: "pro", "max", "plus", "ultra" each name a version, in orders, invoices and catalogue names alike; two texts whose version differs are never the same product. (reads 82 catalogue names, 0 pairs made alike → shadow, by muse-spark) — glass and phones differ by PRO vs MAX vs PLUS, e.g. 13 PRO MAX vs 16 PRO
- [bootstrap] attribute · rejected: "rgb", "gaming", "privacy" each name a light, in orders, invoices and catalogue names alike; a light stated on one side only, or a different one, is never the same product. (reads 28 catalogue names, 0 pairs made alike → rejected, by muse-spark) — only the variant states RGB GAMING PRIVACY, e.g. SK202 RGB vs M29
- [calibration after 10 invoices] block · rejected: inch (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — screen sizes in inch still need normalizing
- [calibration after 10 invoices] block · rejected: dimensions (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — tip sizes like 4.5MM*3.0MM identify exact reference, make one token
- [calibration after 10 invoices] lexicon · rejected: "smartphone" → "mobile" (seen lines: +5 newly resolved (5 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « SMARTPHONE Y7 PRIME 4+64 HUAWEI » → MOBILE HUAWEI Y7 PRIME 4/64GB; « SMARTPHONE REALME 8 PRO 6-128 » → MOBILE REALME 8 PRO 6/128GB; « SUPPORT SMARTPHONE KSC-761B KAKUSIGA » → SUPPORT MOBILE KAKUSIGA KSC-761B, by muse-spark) — catalogue says MOBILE not SMARTPHONE
- [calibration after 10 invoices] lexicon · accepted: "tel" → "mobile" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost, by muse-spark) — TEL SAMSUNG -> MOBILE SAMSUNG
- [calibration after 10 invoices] lexicon · accepted: "tab" → "tablette" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « TAB ATOUCH 5G 4-128 S13 » → TABLETTE ATOUCH 5G 4/128GB S13, by muse-spark) — TAB ATOUCH -> TABLETTE ATOUCH
- [calibration after 10 invoices] lexicon · accepted: "notebook" → "laptop" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « NOTEBOOK HP ELITBOOK 840 G2 I5 5EME 8/256SSD » → LAPTOP HP ELITBOOK 840 G2 I5 5EME 8/256SSD, by muse-spark) — NOTEBOOK HP -> LAPTOP HP
- [calibration after 10 invoices] lexicon · rejected: "portable" → "laptop" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — PC PORTABLE -> LAPTOP, keep laptop token
- [calibration after 10 invoices] lexicon · rejected: "pc" → "" (refused: catalogue names use this word; dropping it would erase a difference between products, by muse-spark) — drop PC when paired with PORTABLE, avoids extra word blocking LAPTOP match
- [calibration after 10 invoices] lexicon · accepted: "charger" → "chargeur" (seen lines: +7 newly resolved (7 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « CHARGER LAPTOP UNIVERSEL 120W » → CHARGEUR LAPTOP UNIVERSEL 120W; « CHARGEUR IPHONE 14PRO RECUPIRATION ORG » → CHARGER IPHONE 14PRO RECUPIRATION ORG; « CHARGER ACEFAST 4IN1 60W AUTO » → CHARGEUR AUTO ACEFAST 4IN1 60W, by muse-spark) — normalize both sides to CHARGEUR, catalogue uses both spellings
- [calibration after 10 invoices] lexicon · rejected: "cbl" → "cable" (seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « CBL X66 HOCO » → CABLE HOCO X66; « CBL REALME LUNA ORIG » → CABLE REALME LUNA ORIG; « CBL HOCO U14 » → CABLE HOCO U14, by muse-spark) — supplier abbrev CBL for CABLE
- [calibration after 10 invoices] lexicon · rejected: "usb-c" → "type-c" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — supplier USB-C = catalogue TYPE-C / T-C
- [calibration after 10 invoices] lexicon · rejected: "t-c" → "type-c" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — catalogue T-C = TYPE-C
- [calibration after 10 invoices] lexicon · rejected: "coque" → "pochette" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « COQUE XUNDO SIMPLE IPHONE » → POCHETTE IPHONE XUNDO SIMPLE, by muse-spark) — COQUE XUNDO -> POCHETTE XUNDO
- [calibration after 10 invoices] lexicon · rejected: "verre" → "glass" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — VERRE TREMPE -> GLASS
- [calibration after 10 invoices] lexicon · rejected: "trempe" → "" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — never in catalogue, drop
- [calibration after 10 invoices] lexicon · rejected: "protection" → "glass" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — PROTECTION ECRAN -> GLASS
- [calibration after 10 invoices] lexicon · rejected: "ecran" → "" (refused: catalogue names use this word; dropping it would erase a difference between products, by muse-spark) — never in catalogue, drop
- [calibration after 10 invoices] lexicon · accepted: "mouse" → "souris" (seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « MOUSE USB CAPSYS-CS345 » → SOURIS USB CAPSYS-CS345; « MOUSE HAVIT S-FIL MS76GT PLUS RECHAREGABLE » → SOURIS HAVIT SANS-FIL MS76GT PLUS RECHAREGABLE; « MOUSE GAMER PROGRAMMABLE MS900S RGB HAVIT » → SOURIS HAVIT GAMER PROGRAMMABLE MS900S RGB, by muse-spark) — MOUSE -> SOURIS
- [calibration after 10 invoices] lexicon · accepted: "speaker" → "baffle" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « SPEAKER HAVIT 2.0 HV-SK427 » → BAFFLE HAVIT 2.0 HV-SK427; « SPEAKER BL-TOOTH HAINO TEKO S18 RGB » → BAFFLE BL-TOOTH HAINO TEKO S18 RGB, by muse-spark) — SPEAKER -> BAFFLE
- [calibration after 10 invoices] lexicon · accepted: "holder" → "support" (seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « HOLDER AUTO HOCO CA63 » → SUPPORT AUTO HOCO CA63; « HOLDER BOROFONE AUTO BH52 » → SUPPORT BOROFONE AUTO BH52; « HOLDER BOROFONE AUTO BH52 » → SUPPORT BOROFONE AUTO BH52, by muse-spark) — HOLDER -> SUPPORT
- [calibration after 10 invoices] lexicon · rejected: "headset" → "casque" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « HEADSET BL-TOOTH CAT-EAR B39M » → CASQUE BL-TOOTH CAT-EAR B39M, by muse-spark) — HEADSET -> CASQUE
- [calibration after 10 invoices] lexicon · rejected: "watch" → "montre" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « WATCH PATEK PHILIPPE AA HOMME » → MONTRE HOMME PATEK PHILIPPE AA, by muse-spark) — WATCH -> MONTRE
- [calibration after 10 invoices] lexicon · accepted: "core" → "" (seen lines: +4 newly resolved (4 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « THINKPAD L13 CORE I5 13TH 16/512SSD LENOVO » → LENOVO THINKPAD L13 I5 13TH 16/512SSD; « NOTEBOOK HP ELITBOOK 650 G10 CORE I5 13TH 16/512SSD » → LAPTOP HP ELITBOOK 650 G10 I5 13TH 16/512SSD; « DELL LATITUDE 9430 CORE I7 12G 16/512SSD TACTILE 360° FISSURE » → DELL LATITUDE 9430 I7 12TH 16/512SSD TACTILE 360° FISSURE, by muse-spark) — CORE I5/I7 never in catalogue
- [calibration after 10 invoices] lexicon · rejected: "galaxy" → "" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — GALAXY extra vs catalogue M32 S24 without it
- [calibration after 10 invoices] lexicon · rejected: "xiaomi" → "" (refused: catalogue names use this word; dropping it would erase a difference between products, by muse-spark) — XIAOMI prefix not in REDMI catalogue names
- [calibration after 10 invoices] lexicon · rejected: "11eme" → "11th" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « HP 17 I5 11EME 16-512 SSD LAPTOP » → LAPTOP HP 17 I5 11TH 16/512 SSD, by muse-spark) — 11EME = 11TH
- [calibration after 10 invoices] lexicon · rejected: "8eme" → "8th" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — 8EME = 8TH
- [calibration after 10 invoices] lexicon · rejected: "s-fil" → "sans-fil" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — S-FIL = SANS-FIL
- [calibration after 10 invoices] lexicon · rejected: "org" → "original" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — catalogue writes ORIGINAL but lines write ORG for the same quality
- [calibration after 10 invoices] lexicon · rejected: "orig" → "original" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — catalogue writes ORIGINAL and ORIG interchangeably
- [calibration after 10 invoices] lexicon · accepted: "bluetooth" → "bl-tooth" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « BLUETOOTH AIRPODS JOYROOM BB3 » → BL-TOOTH AIRPODS JOYROOM BB3; « BLUETOOTH AIR-3 BUDS » → BL-TOOTH AIR-3 BUDS, by muse-spark) — catalogue writes BL-TOOTH, suppliers write bluetooth
- [calibration after 10 invoices] lexicon · rejected: "bt" → "bl-tooth" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write BT for the catalogue's BL-TOOTH
- [calibration after 10 invoices] lexicon · rejected: "typec" → "type-c" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers glue TYPE-C as TYPEC, catalogue writes TYPE-C/T-C
- [calibration after 10 invoices] lexicon · rejected: "smartphone" → "mobile" (from shadow) (seen lines: +5 newly resolved (5 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « SMARTPHONE Y7 PRIME 4+64 HUAWEI » → MOBILE HUAWEI Y7 PRIME 4/64GB; « SMARTPHONE REALME 8 PRO 6-128 » → MOBILE REALME 8 PRO 6/128GB; « SUPPORT SMARTPHONE KSC-761B KAKUSIGA » → SUPPORT MOBILE KAKUSIGA KSC-761B, by muse-spark) — catalogue says MOBILE not SMARTPHONE
- [calibration after 10 invoices] lexicon · rejected: "cbl" → "cable" (from shadow) (seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « CBL X66 HOCO » → CABLE HOCO X66; « CBL REALME LUNA ORIG » → CABLE REALME LUNA ORIG; « CBL HOCO U14 » → CABLE HOCO U14, by muse-spark) — supplier abbrev CBL for CABLE
- [calibration after 10 invoices] search · accepted: BM25 [BM25] (classic word similarity [word similarity]: 93.2 % in top 8, MRR 0.93; BM25 [BM25]: 93.2 % in top 8, MRR 0.93; BM25, brand ×2 [BM25 (brand ×2)]: 93.2 % in top 8, MRR 0.93; trigrams [trigrams]: 93.2 % in top 8, MRR 0.93; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 93.2 % in top 8, MRR 0.93; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 93.2 % in top 8, MRR 0.93; Spark: brand-trigram [BM25 (brand ×3) + 0.7·trigrams]: 93.2 % in top 8, MRR 0.93; Spark: wordsim-help [BM25 (brand ×2) + 0.5·trigrams + 0.5·word similarity]: 93.2 % in top 8, MRR 0.93; Spark: precision [BM25 (brand ×4) + 0.3·trigrams]: 93.2 % in top 8, MRR 0.93; current: Spark's first guess [BM25 (brand ×2) + 0.5·trigrams]: 93.2 % in top 8, MRR 0.93. Chosen: 93.2 % in top 8 on seen lines; on unseen lines 94.4 % vs 94.4 % for the current method, by library) — 
- [calibration after 10 invoices] rekey · accepted: re-read every known wording (38 alias(es) moved to the new reading, no collision, by code) — 
- [after 20 invoices] autonomy · accepted: the desk's steps earned autonomy: Spark stops checking every answer (278 code answers confirmed, 278 right (95 % lower bound 99.04 % ≥ bar 99 %). While supervised: 414 lines, Spark agreed with the code on 278 (278 right), disagreed on 0; alone on 96, right on 96, by code) — 
- [calibration after 20 invoices] block · rejected: inch (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — screen sizes in inch must stay normalized
- [calibration after 20 invoices] block · rejected: dimensions (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — tip sizes like 4.8*1.7MM must become one token to separate chargers
- [calibration after 20 invoices] lexicon · accepted: "etui" → "pochette" (seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « ETUI OPPO XUNDO SIMPLE » → POCHETTE OPPO XUNDO SIMPLE; « ETUI IPHONE 14 PRO MAX XUNDO NEW » → POCHETTE IPHONE 14 PRO MAX XUNDO NEW; « ETUI BELKIN IPHONE » → POCHETTE IPHONE BELKIN, by muse-spark) — ETUI OPPO/BELKIN/IPHONE -> POCHETTE
- [calibration after 20 invoices] lexicon · rejected: "keyboard" → "clavier" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « KEYBOARD GAMER KB275L HAVIT » → CLAVIER HAVIT GAMER KB275L; « KEYBOARD + SOURIS GAMING LED V100 » → CLAVIER + SOURIS GAMING LED V100, by muse-spark) — KEYBOARD GAMER -> CLAVIER HAVIT/GAMING
- [calibration after 20 invoices] lexicon · rejected: "haut" → "baffle" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — HAUT PARLEUR HC21 -> BAFFLE HOCO HC21
- [calibration after 20 invoices] lexicon · rejected: "parleur" → "" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — drop second word of HAUT PARLEUR
- [calibration after 20 invoices] lexicon · rejected: "usb" → "" (refused: catalogue names use this word; dropping it would erase a difference between products, by muse-spark) — USB-C vs T-C / MICRO USB vs MICRO, USB never discriminates
- [calibration after 20 invoices] lexicon · rejected: "v8" → "micro" (refused by the digit guard: it would change the digits of a word, by muse-spark) — V8 YESIDO/CHARGEUR V8 HP -> MICRO YESIDO/CHARGEUR MICRO HP
- [calibration after 20 invoices] lexicon · rejected: "cle" → "flash" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — CLE USB HOCO DISK -> FLASH DISK HOCO
- [calibration after 20 invoices] lexicon · rejected: "10eme" → "10th" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « DELL LATITUDE 5310 I5 10EME 16-256 LAPTOP » → LAPTOP DELL LATITUDE 5310 I5 10TH 16/256GB, by muse-spark) — 10EME vs 10TH laptop generations
- [calibration after 20 invoices] lexicon · accepted: "smartphone" → "mobile" (from shadow) (seen lines: +5 newly resolved (5 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « SMARTPHONE Y7 PRIME 4+64 HUAWEI » → MOBILE HUAWEI Y7 PRIME 4/64GB; « SMARTPHONE REALME 8 PRO 6-128 » → MOBILE REALME 8 PRO 6/128GB; « SUPPORT SMARTPHONE KSC-761B KAKUSIGA » → SUPPORT MOBILE KAKUSIGA KSC-761B, by muse-spark) — catalogue says MOBILE not SMARTPHONE
- [calibration after 20 invoices] lexicon · rejected: "cbl" → "cable" (from shadow) (seen lines: +4 newly resolved (4 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « CBL X66 HOCO » → CABLE HOCO X66; « CBL REALME LUNA ORIG » → CABLE REALME LUNA ORIG; « CBL HOCO U14 » → CABLE HOCO U14, by muse-spark) — supplier abbrev CBL for CABLE
- [calibration after 20 invoices] lexicon · rejected: "keyboard" → "clavier" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « KEYBOARD GAMER KB275L HAVIT » → CLAVIER HAVIT GAMER KB275L; « KEYBOARD + SOURIS GAMING LED V100 » → CLAVIER + SOURIS GAMING LED V100, by muse-spark) — KEYBOARD GAMER -> CLAVIER HAVIT/GAMING
- [calibration after 20 invoices] search · accepted: BM25 [BM25] (classic word similarity [word similarity]: 93.9 % in top 8, MRR 0.93; BM25 [BM25]: 94.2 % in top 8, MRR 0.94; BM25, brand ×2 [BM25 (brand ×2)]: 94.2 % in top 8, MRR 0.94; trigrams [trigrams]: 93.9 % in top 8, MRR 0.93; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 94.2 % in top 8, MRR 0.94; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 94.2 % in top 8, MRR 0.94; Spark: brand-heavy [BM25 (brand ×3) + 0.3·trigrams]: 94.2 % in top 8, MRR 0.94; Spark: balanced-trigram [BM25 (brand ×2) + 0.7·trigrams]: 94.2 % in top 8, MRR 0.94; Spark: bm25-only [BM25]: 94.2 % in top 8, MRR 0.94; current: BM25 [BM25]: 94.2 % in top 8, MRR 0.94. Chosen: 94.2 % in top 8 on seen lines; on unseen lines 93.3 % vs 93.3 % for the current method, by library) — 
- [calibration after 20 invoices] rekey · accepted: re-read every known wording (13 alias(es) moved to the new reading, no collision, by code) — 
- [calibration after 30 invoices] block · rejected: dimensions (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — tip sizes like 4.8*1.7MM must become one token to match catalogue
- [calibration after 30 invoices] block · rejected: inch (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — keep screen sizes unified
- [calibration after 30 invoices] lexicon · rejected: "sam" → "samsung" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — SAM = SAMSUNG
- [calibration after 30 invoices] lexicon · accepted: "t" → "type" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « PORT CHARGEUR TYPE C SAM 25W ORG » → PORT CHARGER T-C SAMSUNG 25W ORG, by muse-spark) — T-C split to t c must match TYPE C
- [calibration after 30 invoices] lexicon · rejected: "10gb" → "" (refused by the digit guard: it would drop a word with digits (a code or a size; codes are learned per supplier in the code table), by muse-spark) — grade token blocking S10 5G match
- [calibration after 30 invoices] lexicon · accepted: "cbl" → "cable" (from shadow) (seen lines: +4 newly resolved (4 distinct wordings), 0 wrong, 0 lost; unseen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; e.g. « CBL X66 HOCO » → CABLE HOCO X66; « CBL REALME LUNA ORIG » → CABLE REALME LUNA ORIG; « CBL HOCO U14 » → CABLE HOCO U14, by muse-spark) — supplier abbrev CBL for CABLE
- [calibration after 30 invoices] lexicon · accepted: "keyboard" → "clavier" (from shadow) (seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « KEYBOARD GAMER KB275L HAVIT » → CLAVIER HAVIT GAMER KB275L; « KEYBOARD + SOURIS GAMING LED V100 » → CLAVIER + SOURIS GAMING LED V100; « KEYBOARD S-F HAVIT KB250G » → CLAVIER S-F HAVIT KB250G, by muse-spark) — KEYBOARD GAMER -> CLAVIER HAVIT/GAMING
- [calibration after 30 invoices] search · accepted: BM25 [BM25] (classic word similarity [word similarity]: 93.7 % in top 8, MRR 0.93; BM25 [BM25]: 94.0 % in top 8, MRR 0.94; BM25, brand ×2 [BM25 (brand ×2)]: 94.0 % in top 8, MRR 0.94; trigrams [trigrams]: 93.7 % in top 8, MRR 0.93; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 94.0 % in top 8, MRR 0.94; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 94.0 % in top 8, MRR 0.94; Spark: bm25-trigram [BM25 (brand ×2) + 0.6·trigrams]: 94.0 % in top 8, MRR 0.94; Spark: balanced-fuzzy [BM25 + 0.8·trigrams + 0.5·word similarity]: 94.0 % in top 8, MRR 0.94; Spark: recall-max [0.5·BM25 + trigrams + word similarity]: 94.0 % in top 8, MRR 0.94; current: BM25 [BM25]: 94.0 % in top 8, MRR 0.94. Chosen: 94.0 % in top 8 on seen lines; on unseen lines 96.0 % vs 96.0 % for the current method, by library) — 
- [calibration after 30 invoices] rekey · accepted: re-read every known wording (19 alias(es) moved to the new reading, no collision, by code) — 

Meta model spend for the whole run: $1.2256

## Who made the free answers possible (test invoices)

599 free answers: Muse Spark found 49 (8 %), the matching Muse Spark designed gave 550 (92 %), a person taught 0 (0 %).

- catalogue match through the matching Muse Spark designed: 550
- rule Muse Spark wrote: 49
