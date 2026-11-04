# What Muse Spark identified: pharmacy / pharmaceuticals

Brand first then strength then galenic form then pack, e.g. AMOCLAN 8.1 ENF 500/62.5MG SACHET B/14. Strengths are written with comma decimals, mixed units and slashes: 1G/125MG, 500/62.5MG, 0,2G/0,5G/100ML, 0,1%, 100UI/ML. Forms and containers are heavily abbreviated in fixed order: CP PELLI/SEC/EFFERV/DISP, GLES/GELS, SP/SUSP BUV, SOL, GTTES, COLLYRE, INJ/AMP, SUPP, PDE/CREME/GEL, FL/T. Pack is B/xx, BT, BOITE DE STYLOS and count decides size; same brand is told apart by strength value, form/coating, population ADULTES/AD/ENF/BEBE and volume.

## First-pass blocks

- **strength** ON — Two and three-value dosages are core identifiers: 500MG/65MG, 1G/125MG, 250MG/5ML, 0,2G/0,5G/100ML, 100UI/ML and partially bare 500/62.5MG must become one dose token or 500 would match 500MG alone.
  - evidence: reads 88 catalogue names differently; products that read alike 13 → 10
- **pack_count** ON — Pack count B/30, B/14, B/10, B/12, B/20, BT, B/5, 30*0.5ML is the box size and must become x30 etc. so B/30 never matches B/20.
  - evidence: reads 303 catalogue names differently; products that read alike 10 → 5

## The desk's own vocabulary (lexicon)

- `comprime` → `cp` [rejected] — suppliers write comprime for catalogue CP
  - evidence: seen lines: +16 newly resolved (13 distinct wordings), 2 wrong, 0 lost; unseen lines: +5 newly resolved (5 distinct wordings), 0 wrong, 0 lost; e.g. « DICYNONE 250 MG B/20 COMPRIME » → DICYNONE 250MG CP B/20; « IRYS 4MG 4MG MG BTE/30 COMPRIME PELLICULAIRE SECABLE » → IRYS 4MG CP PELLI SEC B/30; « ENERGY SEVE COMPRIME A SUCER B/60 MULTI VIT » → ENERGY SEVE CP A SUCER B/60 MULTI- VIT
- `comprimes` → `cp` [live] — suppliers write comprimes for catalogue CP
  - evidence: seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « ZINXIA 25MG COMPRIMES B/30 » → ZINXIA 25MG CP B/30; « CRESTATINE 10MG COMPRIMES » → CRESTATINE 10MG CP
- `gelule` → `gles` [rejected] — suppliers write gelule for catalogue GLES/GELS
  - evidence: seen lines: +4 newly resolved (3 distinct wordings), 0 wrong, 2 lost; unseen lines: +2 newly resolved (1 distinct wordings), 0 wrong, 2 lost; e.g. « HUMEX LIB 4MG/500MG GELULE 4MG/500MG GELULE » → HUMEX LIB 4MG/500MG GLES B/16; lost « FORZAFLEX GELULE T/50GR »; lost « VITAMINE E 400MG GELULE »
- `gelules` → `gles` [live] — suppliers write gelules for catalogue GLES
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « GENESIA GELULES B/20 » → GENESIA GLES B/20
- `sirop` → `sp` [live] — suppliers write sirop for catalogue SP
  - evidence: seen lines: +11 newly resolved (9 distinct wordings), 0 wrong, 0 lost; unseen lines: +6 newly resolved (6 distinct wordings), 0 wrong, 0 lost; e.g. « CLAMOXYL 250MG/5ML SIROP FL/60 ML » → CLAMOXYL 250MG/5ML SP FL 60ML; « SALBUTAMOL SAIDAL 2 MG/5ML MG/ML FL/150 ML SIROP » → SALBUTAMOL SAIDAL 2MG/5ML SP FL 150ML; « INPHADIUM 0,2 MG/ML SIROP FL/125ML » → INPHADIUM 0,2MG/ML SP FL 125ML
- `flacon` → `fl` [rejected] — suppliers write flacon for catalogue FL
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `boite` → `b` [rejected] — suppliers write boite for catalogue B
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `sachet` → `sacht` [rejected] — suppliers write sachet for catalogue SACHT
  - evidence: seen lines: +5 newly resolved (4 distinct wordings), 1 wrong, 1 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. lost « URICARE 3G/SACHET B/01 SACHET »; « LEXIN 1G SACHET B/12 » → LEXIN 1G SACHET B/12; « DOCATRIL 10MG NOURISSANT SAC » → DOCATRIL 10MG NOURISSANT SACHET
- `poudre` → `pdre` [rejected] — suppliers write poudre for catalogue PDRE
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `solution` → `sol` [rejected] — suppliers write solution for catalogue SOL
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost
- `gouttes` → `gttes` [rejected] — suppliers write gouttes for catalogue GTTES/GTTES
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost
- `pommade` → `pde` [rejected] — suppliers write pommade for catalogue PDE
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `injectable` → `inj` [rejected] — suppliers write injectable for catalogue INJ
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `suppositoire` → `supp` [rejected] — suppliers write suppositoire for catalogue SUPP
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `sir` → `sp` [live] — HISTAGAN SIR vs SP
  - evidence: seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « HISTAGAN 0,5MG/5ML SIR FL/125 ML » → HISTAGAN 0,5MG/5ML SP FL 125ML; « KITCOLIC SIR FL 120ML » → KITCOLIC SP FL 120ML
- `susp` → `sp` [rejected] — ZOMAX/NOBAC SUSP BUV vs SP
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 5 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. lost « CLARADINE 5MG/5ML SUSPENSION BUV FL 60ML »; lost « VITATINE SUSPENSION BUV »; lost « FLUCASONE MINI 27.5 UG AVAMYS SUSPENSION BUV FL 120ML »
- `comp` → `cp` [rejected] — COMP./COMP vs CP
  - evidence: seen lines: +56 newly resolved (51 distinct wordings), 2 wrong, 0 lost; unseen lines: +12 newly resolved (11 distinct wordings), 0 wrong, 0 lost; e.g. « CLIMASTON 2MG/10MG B/28 COMP.PELLI » → CLIMASTON 2MG/10MG CP PELLI B/28; « PINATEL 100 MG COMP.PELLI BTE/28 » → PINATEL 100MG CP PELLI B/28; « FEXODINE 120MG BTE/15 COMP.PELLI » → FEXODINE 120MG CP PELLI B/15
- `suppo` → `supp` [rejected] — SUPPO. vs SUPP
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `sach` → `sacht` [rejected] — SACH vs SACHT
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `goutte` → `gttes` [rejected] — singular variant
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `pelliculaire` → `pelli` [rejected] — PELLICULAIRE vs PELLI
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `pellicule` → `pelli` [rejected] — PELLICULE vs PELLI
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `secable` → `sec` [rejected] — SECABLE vs SEC
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `enro` → `enrob` [rejected] — COMP.ENRO. vs ENROB
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `enfants` → `enf` [live] — ENFANTS vs ENF
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « BRONCHOCALM ENFANTS 0,1 % FL/125ML » → BRONCHOCALM ENF 0,1% SP FL 125ML
- `enfant` → `enf` [rejected] — variant
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `adulte` → `adultes` [rejected] — ADULTE vs ADULTES
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `flc` → `fl` [live] — FLC vs FL
  - evidence: seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « PHYSIOMAR BEBE SPRAY NASAL 0 3 ANS FLC 30ML » → PHYSIOMAR BÉBÉ SPRAY NASAL 0-3 ANS FL 30ML; « SELOFER 50 10MG/ML SP FLC 150ML » → SELOFER-50 10MG/ML SP FL 150ML
- `spr` → `spray` [rejected] — SPR NASAL vs SPRAY NASAL
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `creme` → `pde` [rejected] — LOCOID CREME vs PDE
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 3 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 4 lost; e.g. lost « CORTEN 0,01 CREME T/15 G »; lost « CORTEN 0.01 CREME T/15G »; lost « CORTEN 0,01 CREME T/15 G »
- `caps` → `gles` [rejected] — CAPS is catalogue GLES
  - evidence: seen lines: +4 newly resolved (1 distinct wordings), 0 wrong, 1 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. lost « VITAMINE E 400MG GELULE »; « GABATREX 300MG B/50 CAPS » → GABATREX 300MG GLES B/50; « GABATREX 300MG B/50 CAPS » → GABATREX 300MG GLES B/50
- `ad` → `adultes` [rejected] — AD is catalogue ADULTES population
  - evidence: seen lines: +2 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « ESTHEMIX ADULTES CREME TUBE 50GR » → ESTHEMIX AD CREME TUBE 50GR; « ESTHEMIX ADULTES CREME TUBE 50GR » → ESTHEMIX AD CREME TUBE 50GR
- `sol` → `` [rejected] — SOL INJ vs INJ and SOL BUV vs SP/GTTES, SOL never distinguishes, drop it
  - evidence: refused: catalogue names use this word; dropping it would erase a difference between products
- `buv` → `` [rejected] — SUSP/SOL BUV vs SP/GTTES, BUV extra blocks exact match, drop it
  - evidence: refused: catalogue names use this word; dropping it would erase a difference between products
- `bte` → `b` [rejected] — BTE/28 must become B/28 so pack_count makes x28
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `physiopharm` → `physio` [live] — PHYSIOPHARM is catalogue PHYSIO
  - evidence: seen lines: +3 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « NIFUROXAZIDE PHYSIOPHARM 200MG GLES B/30 » → NIFUROXAZIDE PHYSIO 200MG GLES B/30; « NIFUROXAZIDE PHYSIOPHARM 200MG GLES B/30 » → NIFUROXAZIDE PHYSIO 200MG GLES B/30; « NIFUROXAZIDE PHYSIOPHARM 200MG GLES B/30 » → NIFUROXAZIDE PHYSIO 200MG GLES B/30
- `02` → `2` [rejected] — B/02 must equal B/2 before pack_count
  - evidence: refused by the digit guard: it would change the digits of a word
- `08` → `8` [rejected] — BTE/08 must equal B/8 before pack_count
  - evidence: refused by the digit guard: it would change the digits of a word
- `sac` → `sacht` [rejected] — SAC truncation for SACHET.
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `adultes` → `ad` [rejected] — ADULTES for AD.
  - evidence: seen lines: +2 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « ESTHEMIX ADULTES CREME TUBE 50GR » → ESTHEMIX AD CREME TUBE 50GR; « ESTHEMIX ADULTES CREME TUBE 50GR » → ESTHEMIX AD CREME TUBE 50GR
- `fl` → `` [rejected] — FL/ container word extra vs GTTES 50ML without it
  - evidence: refused: catalogue names use this word; dropping it would erase a difference between products
- `06` → `6` [rejected] — BTE/06 vs B/6
  - evidence: refused by the digit guard: it would change the digits of a word

## Search method

**BM25** (BM25)

## Rules (rule engineer and attributes)

- R-0003 [live] "ad", "adultes", "enf", "bebe", "nourrissant" each name a population, in orders, invoices and catalogue names alike; two texts whose population differs are never the same product. — by muse-spark-profile
- R-0002 [live] "pelli", "sec", "efferv", "disp", "enrob", "orod", "lp" each name a coating_release, in orders, invoices and catalogue names alike; two texts whose coating_release differs are never the same product. — by muse-spark-profile
- R-0001 [live] "cp", "gles", "gels", "sp", "susp", "sol", "gttes", "collyre", "inj", "amp", "supp", "pde", "creme", "gel", "ovul", "granul", "gran" each name a form, in orders, invoices and catalogue names alike; two texts whose form differs are never the same product. — by muse-spark-profile

## Phases

| phase | invoices | lines | free (alias + rules) | AI | asked | wrong auto | Spark agreed with code | Spark alone right | AI cost |
|---|---|---|---|---|---|---|---|---|---|
| spark-checks-all | 28 | 523 | 270 (52 %) | 183 (35 %) | 70 (13 %) | 0 | 270/270 | 183/183 | $0.1831 |
| learning | 92 | 1635 | 1240 (76 %) | 250 (15 %) | 142 (9 %) | 3 | 542/542 | 250/250 | $0.4014 |
| test after 0 | 30 | 547 | 285 (52 %) | 205 (37 %) | 57 (10 %) | 0 | 285/285 | 205/205 | $0.1947 |
| test after 30 | 30 | 547 | 355 (65 %) | 125 (23 %) | 67 (12 %) | 0 | 160/160 | 125/125 | $0.1382 |
| test after 60 | 30 | 547 | 390 (71 %) | 99 (18 %) | 58 (11 %) | 0 | 189/189 | 99/99 | $0.1368 |
| test after 90 | 30 | 547 | 421 (77 %) | 77 (14 %) | 49 (9 %) | 0 | 178/178 | 77/77 | $0.1284 |
| test after 120 | 30 | 547 | 439 (80 %) | 65 (12 %) | 43 (8 %) | 0 | 192/192 | 65/65 | $0.1301 |

## Calibrations

- **calibration after 30 invoices** (422 seen + 141 unseen lines): 213/422 right with no AI (50 %), 0 wrong → 230/422 right with no AI (55 %), 0 wrong; accepted 5, rejected 28; search: BM25 (brand ×2) + 0.5·trigrams: right product in the top 8 for 89.1 % of lines (MRR 0.88) → BM25: 87.4 % in the top 8 (MRR 0.86); engineer rules []; $0.7498. Spark: lines fail on spaced vs glued doses/volumes and fixed form abbreviations SP/CP/GTTES/PELLI/SACHT, bare slash doses hurt exact match
- **calibration after 60 invoices** (812 seen + 271 unseen lines): 463/812 right with no AI (57 %), 0 wrong → 466/812 right with no AI (57 %), 0 wrong; accepted 1, rejected 8; search: BM25: right product in the top 8 for 88.2 % of lines (MRR 0.87) → BM25: 88.2 % in the top 8 (MRR 0.87); engineer rules []; $0.2130. Spark: COMP/COMPRIME->CP plus PELLI/SEC/ENROB/SUPP/GLES/SP/GTTES/PDE/SACHT normalisation and dropping SOL/BUV with bte->b and bare strength fixes exact matches
- **calibration after 90 invoices** (1206 seen + 402 unseen lines): 690/1206 right with no AI (57 %), 1 wrong → 692/1206 right with no AI (57 %), 1 wrong; accepted 1, rejected 3; search: BM25: right product in the top 8 for 87.9 % of lines (MRR 0.87) → BM25: 87.9 % in the top 8 (MRR 0.87); engineer rules []; $0.1881. Spark: Unresolved are synonyms COMP->CP PELLI/SEC/ENROB SUPPO->SUPP SOL/BUV/SUSP->SP GOUTTES->GTTES CREME->PDE SACHET variants plus glued FL22.5ML and bare 400/40 doses.
- **calibration after 120 invoices** (1618 seen + 540 unseen lines): 937/1618 right with no AI (58 %), 1 wrong → 937/1618 right with no AI (58 %), 1 wrong; accepted 0, rejected 4; search: BM25: right product in the top 8 for 87.9 % of lines (MRR 0.87) → BM25: 87.9 % in the top 8 (MRR 0.87); engineer rules []; $0.2247. Spark: space vs glued strengths, COMP/COMPRIME/PELLI/SUPPO/CAPS/GOUTTES/SUSP/SOL/BUV/SACHET/ADULTE abbreviations and bare dosages block exact match; units_glue plus lexicon unification fixes them.

## Experiment log

- [bootstrap] block · accepted: strength (bare) (reads 88 catalogue names differently; products that read alike 13 → 10, by muse-spark) — Two and three-value dosages are core identifiers: 500MG/65MG, 1G/125MG, 250MG/5ML, 0,2G/0,5G/100ML, 100UI/ML and partially bare 500/62.5MG must become one dose token or 500 would match 500MG alone.
- [bootstrap] block · accepted: pack_count (reads 303 catalogue names differently; products that read alike 10 → 5, by muse-spark) — Pack count B/30, B/14, B/10, B/12, B/20, BT, B/5, 30*0.5ML is the box size and must become x30 etc. so B/30 never matches B/20.
- [bootstrap] lexicon · accepted: "comprime" → "cp" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write comprime for catalogue CP
- [bootstrap] lexicon · accepted: "comprimes" → "cp" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write comprimes for catalogue CP
- [bootstrap] lexicon · accepted: "gelule" → "gles" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write gelule for catalogue GLES/GELS
- [bootstrap] lexicon · accepted: "gelules" → "gles" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write gelules for catalogue GLES
- [bootstrap] lexicon · accepted: "sirop" → "sp" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write sirop for catalogue SP
- [bootstrap] lexicon · accepted: "flacon" → "fl" (proposed before any invoice (reads 3 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write flacon for catalogue FL
- [bootstrap] lexicon · accepted: "boite" → "b" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write boite for catalogue B
- [bootstrap] lexicon · accepted: "sachet" → "sacht" (proposed before any invoice (reads 9 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write sachet for catalogue SACHT
- [bootstrap] lexicon · accepted: "poudre" → "pdre" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write poudre for catalogue PDRE
- [bootstrap] lexicon · accepted: "solution" → "sol" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write solution for catalogue SOL
- [bootstrap] lexicon · accepted: "gouttes" → "gttes" (proposed before any invoice (reads 1 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write gouttes for catalogue GTTES/GTTES
- [bootstrap] lexicon · accepted: "pommade" → "pde" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write pommade for catalogue PDE
- [bootstrap] lexicon · accepted: "injectable" → "inj" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write injectable for catalogue INJ
- [bootstrap] lexicon · accepted: "suppositoire" → "supp" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 5 → 5): waits in shadow until known lines prove it, by muse-spark) — suppliers write suppositoire for catalogue SUPP
- [bootstrap] search · accepted: BM25 (brand ×2) + 0.5·trigrams (not measured yet: no line has a known answer, by muse-spark) — 384 brands in 524 products so brand must dominate; bm25 on rare strength/form tokens for precision, low trigram for small typos only, no wordsim because digit words must match exactly
- [bootstrap] attribute · accepted: "cp", "gles", "gels", "sp", "susp", "sol", "gttes", "collyre", "inj", "amp", "supp", "pde", "creme", "gel", "ovul", "granul", "gran" each name a form, in orders, invoices and catalogue names alike; two texts whose form differs are never the same product. (reads 472 catalogue names, 0 pairs made alike → shadow, by muse-spark) — same brand in CP vs SP vs INJ vs PDE, e.g. CLAMOXYL SP vs CP, is a different product
- [bootstrap] attribute · accepted: "pelli", "sec", "efferv", "disp", "enrob", "orod", "lp" each name a coating_release, in orders, invoices and catalogue names alike; two texts whose coating_release differs are never the same product. (reads 122 catalogue names, 0 pairs made alike → shadow, by muse-spark) — coating and release PELLI vs SEC vs EFFERV vs LP tells ATACAND 16MG CP SEC from plain CP apart
- [bootstrap] attribute · accepted: "ad", "adultes", "enf", "bebe", "nourrissant" each name a population, in orders, invoices and catalogue names alike; two texts whose population differs are never the same product. (reads 20 catalogue names, 0 pairs made alike → shadow, by muse-spark) — AUGMENTIN ADULTES vs ENF and GLYCERINE BEBE are different products of same range
- [after 28 invoices] autonomy · accepted: the desk's steps earned autonomy: Spark stops checking every answer (270 code answers confirmed, 270 right (95 % lower bound 99.01 % ≥ bar 99 %). While supervised: 523 lines, Spark agreed with the code on 270 (270 right), disagreed on 0; alone on 183, right on 183, by code) — 
- [calibration after 30 invoices] block · rejected: strength (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 1 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « PROCTOLON NEO 400/40 » → PROCTOLON 400/40 MG NEO; lost « BETACYL 0,05/3 % T/15G », by muse-spark) — bare dose creation turns 150MG/150MG into dose150x150 while catalogue has single 150MG and turns bare 400/40 into dose400x40 while catalogue has 400/40 MG; bare off keeps slash codes as plain words so subset match works
- [calibration after 30 invoices] block · rejected: units_glue (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — catalogue glues 250MG FL60ML T30G while lines write 250 MG FL/60 ML T/30G; without glue digit-words never match
- [calibration after 30 invoices] block · rejected: split_glued (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — split glued number words so venus5l/fl22 form splits before units_glue re-glues the dose/volume part
- [calibration after 30 invoices] lexicon · accepted: "sir" → "sp" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « HISTAGAN 0,5MG/5ML SIR FL/125 ML » → HISTAGAN 0,5MG/5ML SP FL 125ML; « KITCOLIC SIR FL 120ML » → KITCOLIC SP FL 120ML, by muse-spark) — HISTAGAN SIR vs SP
- [calibration after 30 invoices] lexicon · rejected: "susp" → "sp" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 5 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. lost « CLARADINE 5MG/5ML SUSPENSION BUV FL 60ML »; lost « VITATINE SUSPENSION BUV »; lost « FLUCASONE MINI 27.5 UG AVAMYS SUSPENSION BUV FL 120ML », by muse-spark) — ZOMAX/NOBAC SUSP BUV vs SP
- [calibration after 30 invoices] lexicon · rejected: "comp" → "cp" (seen lines: +56 newly resolved (51 distinct wordings), 2 wrong, 0 lost; unseen lines: +12 newly resolved (11 distinct wordings), 0 wrong, 0 lost; e.g. « CLIMASTON 2MG/10MG B/28 COMP.PELLI » → CLIMASTON 2MG/10MG CP PELLI B/28; « PINATEL 100 MG COMP.PELLI BTE/28 » → PINATEL 100MG CP PELLI B/28; « FEXODINE 120MG BTE/15 COMP.PELLI » → FEXODINE 120MG CP PELLI B/15, by muse-spark) — COMP./COMP vs CP
- [calibration after 30 invoices] lexicon · rejected: "suppo" → "supp" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — SUPPO. vs SUPP
- [calibration after 30 invoices] lexicon · rejected: "sach" → "sacht" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — SACH vs SACHT
- [calibration after 30 invoices] lexicon · rejected: "goutte" → "gttes" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — singular variant
- [calibration after 30 invoices] lexicon · rejected: "pelliculaire" → "pelli" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — PELLICULAIRE vs PELLI
- [calibration after 30 invoices] lexicon · rejected: "pellicule" → "pelli" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — PELLICULE vs PELLI
- [calibration after 30 invoices] lexicon · rejected: "secable" → "sec" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — SECABLE vs SEC
- [calibration after 30 invoices] lexicon · rejected: "enro" → "enrob" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — COMP.ENRO. vs ENROB
- [calibration after 30 invoices] lexicon · accepted: "enfants" → "enf" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « BRONCHOCALM ENFANTS 0,1 % FL/125ML » → BRONCHOCALM ENF 0,1% SP FL 125ML, by muse-spark) — ENFANTS vs ENF
- [calibration after 30 invoices] lexicon · rejected: "enfant" → "enf" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — variant
- [calibration after 30 invoices] lexicon · rejected: "adulte" → "adultes" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — ADULTE vs ADULTES
- [calibration after 30 invoices] lexicon · accepted: "flc" → "fl" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « PHYSIOMAR BEBE SPRAY NASAL 0 3 ANS FLC 30ML » → PHYSIOMAR BÉBÉ SPRAY NASAL 0-3 ANS FL 30ML; « SELOFER 50 10MG/ML SP FLC 150ML » → SELOFER-50 10MG/ML SP FL 150ML, by muse-spark) — FLC vs FL
- [calibration after 30 invoices] lexicon · rejected: "spr" → "spray" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — SPR NASAL vs SPRAY NASAL
- [calibration after 30 invoices] lexicon · rejected: "creme" → "pde" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 3 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 4 lost; e.g. lost « CORTEN 0,01 CREME T/15 G »; lost « CORTEN 0.01 CREME T/15G »; lost « CORTEN 0,01 CREME T/15 G », by muse-spark) — LOCOID CREME vs PDE
- [calibration after 30 invoices] lexicon · rejected: "comprime" → "cp" (from shadow) (seen lines: +16 newly resolved (13 distinct wordings), 2 wrong, 0 lost; unseen lines: +5 newly resolved (5 distinct wordings), 0 wrong, 0 lost; e.g. « DICYNONE 250 MG B/20 COMPRIME » → DICYNONE 250MG CP B/20; « IRYS 4MG 4MG MG BTE/30 COMPRIME PELLICULAIRE SECABLE » → IRYS 4MG CP PELLI SEC B/30; « ENERGY SEVE COMPRIME A SUCER B/60 MULTI VIT » → ENERGY SEVE CP A SUCER B/60 MULTI- VIT, by muse-spark) — suppliers write comprime for catalogue CP
- [calibration after 30 invoices] lexicon · rejected: "comprimes" → "cp" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « ZINXIA 25MG COMPRIMES B/30 » → ZINXIA 25MG CP B/30; « CRESTATINE 10MG COMPRIMES » → CRESTATINE 10MG CP, by muse-spark) — suppliers write comprimes for catalogue CP
- [calibration after 30 invoices] lexicon · rejected: "gelule" → "gles" (from shadow) (seen lines: +4 newly resolved (3 distinct wordings), 0 wrong, 2 lost; unseen lines: +2 newly resolved (1 distinct wordings), 0 wrong, 2 lost; e.g. « HUMEX LIB 4MG/500MG GELULE 4MG/500MG GELULE » → HUMEX LIB 4MG/500MG GLES B/16; lost « FORZAFLEX GELULE T/50GR »; lost « VITAMINE E 400MG GELULE », by muse-spark) — suppliers write gelule for catalogue GLES/GELS
- [calibration after 30 invoices] lexicon · accepted: "gelules" → "gles" (from shadow) (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « GENESIA GELULES B/20 » → GENESIA GLES B/20, by muse-spark) — suppliers write gelules for catalogue GLES
- [calibration after 30 invoices] lexicon · accepted: "sirop" → "sp" (from shadow) (seen lines: +11 newly resolved (9 distinct wordings), 0 wrong, 0 lost; unseen lines: +6 newly resolved (6 distinct wordings), 0 wrong, 0 lost; e.g. « CLAMOXYL 250MG/5ML SIROP FL/60 ML » → CLAMOXYL 250MG/5ML SP FL 60ML; « SALBUTAMOL SAIDAL 2 MG/5ML MG/ML FL/150 ML SIROP » → SALBUTAMOL SAIDAL 2MG/5ML SP FL 150ML; « INPHADIUM 0,2 MG/ML SIROP FL/125ML » → INPHADIUM 0,2MG/ML SP FL 125ML, by muse-spark) — suppliers write sirop for catalogue SP
- [calibration after 30 invoices] lexicon · rejected: "flacon" → "fl" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write flacon for catalogue FL
- [calibration after 30 invoices] lexicon · rejected: "boite" → "b" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write boite for catalogue B
- [calibration after 30 invoices] lexicon · rejected: "sachet" → "sacht" (from shadow) (seen lines: +5 newly resolved (4 distinct wordings), 1 wrong, 1 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. lost « URICARE 3G/SACHET B/01 SACHET »; « LEXIN 1G SACHET B/12 » → LEXIN 1G SACHET B/12; « DOCATRIL 10MG NOURISSANT SAC » → DOCATRIL 10MG NOURISSANT SACHET, by muse-spark) — suppliers write sachet for catalogue SACHT
- [calibration after 30 invoices] lexicon · rejected: "poudre" → "pdre" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write poudre for catalogue PDRE
- [calibration after 30 invoices] lexicon · rejected: "solution" → "sol" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write solution for catalogue SOL
- [calibration after 30 invoices] lexicon · rejected: "gouttes" → "gttes" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write gouttes for catalogue GTTES/GTTES
- [calibration after 30 invoices] lexicon · rejected: "pommade" → "pde" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write pommade for catalogue PDE
- [calibration after 30 invoices] lexicon · rejected: "injectable" → "inj" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write injectable for catalogue INJ
- [calibration after 30 invoices] lexicon · rejected: "suppositoire" → "supp" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write suppositoire for catalogue SUPP
- [calibration after 30 invoices] search · accepted: BM25 [BM25] (classic word similarity [word similarity]: 87.4 % in top 8, MRR 0.86; BM25 [BM25]: 87.4 % in top 8, MRR 0.86; BM25, brand ×2 [BM25 (brand ×2)]: 87.4 % in top 8, MRR 0.86; trigrams [trigrams]: 87.4 % in top 8, MRR 0.86; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 87.4 % in top 8, MRR 0.86; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 87.4 % in top 8, MRR 0.86; Spark: bm25-brand-trigram [BM25 (brand ×2) + 0.5·trigrams]: 87.4 % in top 8, MRR 0.86; Spark: trigram-heavy [BM25 (brand ×2) + trigrams]: 87.4 % in top 8, MRR 0.86; Spark: wordsim-help [BM25 (brand ×3) + 0.7·trigrams + 0.5·word similarity]: 87.4 % in top 8, MRR 0.86; current: Spark's first guess [BM25 (brand ×2) + 0.5·trigrams]: 87.4 % in top 8, MRR 0.86. Chosen: 87.4 % in top 8 on seen lines; on unseen lines 86.5 % vs 86.5 % for the current method, by library) — 
- [calibration after 30 invoices] rekey · accepted: re-read every known wording (37 alias(es) moved to the new reading, no collision, by code) — 
- [calibration after 60 invoices] lexicon · rejected: "caps" → "gles" (seen lines: +4 newly resolved (1 distinct wordings), 0 wrong, 1 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. lost « VITAMINE E 400MG GELULE »; « GABATREX 300MG B/50 CAPS » → GABATREX 300MG GLES B/50; « GABATREX 300MG B/50 CAPS » → GABATREX 300MG GLES B/50, by muse-spark) — CAPS is catalogue GLES
- [calibration after 60 invoices] lexicon · rejected: "ad" → "adultes" (seen lines: +2 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « ESTHEMIX ADULTES CREME TUBE 50GR » → ESTHEMIX AD CREME TUBE 50GR; « ESTHEMIX ADULTES CREME TUBE 50GR » → ESTHEMIX AD CREME TUBE 50GR, by muse-spark) — AD is catalogue ADULTES population
- [calibration after 60 invoices] lexicon · rejected: "sol" → "" (refused: catalogue names use this word; dropping it would erase a difference between products, by muse-spark) — SOL INJ vs INJ and SOL BUV vs SP/GTTES, SOL never distinguishes, drop it
- [calibration after 60 invoices] lexicon · rejected: "buv" → "" (refused: catalogue names use this word; dropping it would erase a difference between products, by muse-spark) — SUSP/SOL BUV vs SP/GTTES, BUV extra blocks exact match, drop it
- [calibration after 60 invoices] lexicon · rejected: "bte" → "b" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — BTE/28 must become B/28 so pack_count makes x28
- [calibration after 60 invoices] lexicon · accepted: "physiopharm" → "physio" (seen lines: +3 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « NIFUROXAZIDE PHYSIOPHARM 200MG GLES B/30 » → NIFUROXAZIDE PHYSIO 200MG GLES B/30; « NIFUROXAZIDE PHYSIOPHARM 200MG GLES B/30 » → NIFUROXAZIDE PHYSIO 200MG GLES B/30; « NIFUROXAZIDE PHYSIOPHARM 200MG GLES B/30 » → NIFUROXAZIDE PHYSIO 200MG GLES B/30, by muse-spark) — PHYSIOPHARM is catalogue PHYSIO
- [calibration after 60 invoices] lexicon · rejected: "02" → "2" (refused by the digit guard: it would change the digits of a word, by muse-spark) — B/02 must equal B/2 before pack_count
- [calibration after 60 invoices] lexicon · rejected: "08" → "8" (refused by the digit guard: it would change the digits of a word, by muse-spark) — BTE/08 must equal B/8 before pack_count
- [calibration after 60 invoices] lexicon · rejected: "comprimes" → "cp" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « ZINXIA 25MG COMPRIMES B/30 » → ZINXIA 25MG CP B/30; « CRESTATINE 10MG COMPRIMES » → CRESTATINE 10MG CP, by muse-spark) — suppliers write comprimes for catalogue CP
- [calibration after 60 invoices] search · accepted: BM25 [BM25] (classic word similarity [word similarity]: 88.2 % in top 8, MRR 0.87; BM25 [BM25]: 88.2 % in top 8, MRR 0.87; BM25, brand ×2 [BM25 (brand ×2)]: 88.2 % in top 8, MRR 0.87; trigrams [trigrams]: 88.2 % in top 8, MRR 0.87; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 88.2 % in top 8, MRR 0.87; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 88.2 % in top 8, MRR 0.87; Spark: brand-trigram [BM25 (brand ×3) + 0.7·trigrams + 0.3·word similarity]: 88.2 % in top 8, MRR 0.87; Spark: balanced [BM25 (brand ×2) + trigrams]: 88.2 % in top 8, MRR 0.87; Spark: precise [BM25 (brand ×4) + 0.3·trigrams + 0.5·word similarity]: 88.2 % in top 8, MRR 0.87; current: BM25 [BM25]: 88.2 % in top 8, MRR 0.87. Chosen: 88.2 % in top 8 on seen lines; on unseen lines 86.3 % vs 86.3 % for the current method, by library) — 
- [calibration after 60 invoices] rekey · accepted: re-read every known wording (7 alias(es) moved to the new reading, no collision, by code) — 
- [calibration after 90 invoices] block · rejected: split_glued (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — Split FL22.5ML -> FL 22.5ML and similar glued FL/volume forms so volume clean-up can match.
- [calibration after 90 invoices] lexicon · rejected: "sac" → "sacht" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — SAC truncation for SACHET.
- [calibration after 90 invoices] lexicon · rejected: "adultes" → "ad" (seen lines: +2 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « ESTHEMIX ADULTES CREME TUBE 50GR » → ESTHEMIX AD CREME TUBE 50GR; « ESTHEMIX ADULTES CREME TUBE 50GR » → ESTHEMIX AD CREME TUBE 50GR, by muse-spark) — ADULTES for AD.
- [calibration after 90 invoices] lexicon · accepted: "comprimes" → "cp" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « ZINXIA 25MG COMPRIMES B/30 » → ZINXIA 25MG CP B/30; « CRESTATINE 10MG COMPRIMES » → CRESTATINE 10MG CP, by muse-spark) — suppliers write comprimes for catalogue CP
- [calibration after 90 invoices] search · accepted: BM25 [BM25] (classic word similarity [word similarity]: 87.9 % in top 8, MRR 0.86; BM25 [BM25]: 87.9 % in top 8, MRR 0.87; BM25, brand ×2 [BM25 (brand ×2)]: 87.9 % in top 8, MRR 0.87; trigrams [trigrams]: 87.9 % in top 8, MRR 0.86; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 87.9 % in top 8, MRR 0.87; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 87.9 % in top 8, MRR 0.87; Spark: brand-precision [BM25 (brand ×2) + 0.5·trigrams]: 87.9 % in top 8, MRR 0.87; Spark: typo-tolerant [BM25 + trigrams + 0.5·word similarity]: 87.9 % in top 8, MRR 0.87; Spark: balanced [BM25 (brand ×1.5) + 0.8·trigrams]: 87.9 % in top 8, MRR 0.87; current: BM25 [BM25]: 87.9 % in top 8, MRR 0.87. Chosen: 87.9 % in top 8 on seen lines; on unseen lines 88.3 % vs 88.3 % for the current method, by library) — 
- [calibration after 90 invoices] rekey · accepted: re-read every known wording (4 alias(es) moved to the new reading, no collision, by code) — 
- [calibration after 120 invoices] block · rejected: units_glue (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — lines write 250 MG, 0,5 MG/ML with space vs catalogue 250MG; glue number+unit or digit-words never match
- [calibration after 120 invoices] block · rejected: split_glued (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — split glued number words like sachet counts and FL22.5ML style gluings where possible
- [calibration after 120 invoices] lexicon · rejected: "fl" → "" (refused: catalogue names use this word; dropping it would erase a difference between products, by muse-spark) — FL/ container word extra vs GTTES 50ML without it
- [calibration after 120 invoices] lexicon · rejected: "06" → "6" (refused by the digit guard: it would change the digits of a word, by muse-spark) — BTE/06 vs B/6
- [calibration after 120 invoices] search · accepted: BM25 [BM25] (classic word similarity [word similarity]: 87.9 % in top 8, MRR 0.86; BM25 [BM25]: 87.9 % in top 8, MRR 0.87; BM25, brand ×2 [BM25 (brand ×2)]: 87.9 % in top 8, MRR 0.87; trigrams [trigrams]: 87.9 % in top 8, MRR 0.86; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 87.9 % in top 8, MRR 0.87; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 87.9 % in top 8, MRR 0.87; Spark: brand-strength [BM25 (brand ×3) + 0.3·trigrams + 0.5·word similarity]: 87.9 % in top 8, MRR 0.87; Spark: typo-tolerant [BM25 (brand ×2) + 0.8·trigrams + 0.3·word similarity]: 87.9 % in top 8, MRR 0.87; Spark: balanced [BM25 (brand ×1.5) + 0.5·trigrams + 0.8·word similarity]: 87.9 % in top 8, MRR 0.87; current: BM25 [BM25]: 87.9 % in top 8, MRR 0.87. Chosen: 87.9 % in top 8 on seen lines; on unseen lines 90.2 % vs 90.2 % for the current method, by library) — 

Meta model spend for the whole run: $2.7082

## Who made the free answers possible (test invoices)

1890 free answers: Muse Spark found 904 (48 %), the matching Muse Spark designed gave 885 (47 %), a person taught 101 (5 %).

- rule Muse Spark wrote: 612
- catalogue match through the matching Muse Spark designed: 481
- alias first matched through the matching Muse Spark designed: 387
- alias first found by Muse Spark: 292
- alias first answered by a person: 101
- code first matched through the matching Muse Spark designed: 17
