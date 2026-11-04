# What Muse Spark identified: pharmacy and parapharmacy

Brand first then function/form, in French with heavy abbreviations: BAD for toothbrushes, BIB/BIBERON, DENT for toothpaste, SHAMP, CREM/CREME, GEL, LOTION. Sizes are volumes and masses 15 ML to 600 ML, 25 G to 400 GR, dosages 0.05%, 400MG, 1000MG, bandage dimensions 4M X 10 CM with order varying. Packs decide variants: B/10, B/20, B/30, B/100, P/30, C/120, CP/COMP/CAPS/SACHET/AMP. What tells two products of one brand apart is dosage, volume/mass, dimension, pack count, plus colour, flavour and size for toothbrushes, condoms, baby bottles.

## First-pass blocks

- **split_glued** ON — lines glue count to form like 30COMP, 30TAB, B100; must split to 30 comp to match spaced catalogue form
  - evidence: reads 0 catalogue names differently; products that read alike 13 → 13
- **dimensions** ON — bandages and plasters written 4M X 10 CM, 10 CM X 4 M, 10 CM X 2.5 M; pair must become one token so 4x10 never matches 4x7
  - evidence: reads 1 catalogue names differently; products that read alike 13 → 13
- **units_glue** ON — needle and bandage lengths 5 MM, 3.5 MM, 10 CM, 4M written spaced or glued; glue to 5mm so spaced and glued forms match
  - evidence: reads 19 catalogue names differently; products that read alike 13 → 12
- **pack_count** ON — B/10, B/20, B/100, P/30, C/120, C/18, B/6 are the pack size and differ between otherwise identical products; must become x10, x30 etc.
  - evidence: reads 146 catalogue names differently; products that read alike 12 → 12

## The desk's own vocabulary (lexicon)

- `biberon` → `bib` [rejected] — suppliers write biberon for the catalogue's BIB
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 1 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. lost « DENIZ BIB PLAS 240 ML A/B »
- `brosse` → `bad` [rejected] — suppliers write brosse a dents for the catalogue's BAD
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « BROSSE KIT 6 PCS » → BAD KIT 6 PCS
- `dentifrice` → `dent` [rejected] — suppliers write dentifrice for the catalogue's DENT
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `shampooing` → `shamp` [rejected] — suppliers write shampooing for the catalogue's SHAMP
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « JOHENSONS SHAMPOOING JAUNE 300 ML » → JOHENSONS SHAMP JAUNE 300 ML
- `shampoing` → `shamp` [rejected] — alternate spelling suppliers use for the catalogue's SHAMP
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `crem` → `creme` [rejected] — catalogue mostly writes creme, variant CREM must normalise
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `comprime` → `cp` [rejected] — suppliers write comprime for the catalogue's CP
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `comp` → `cp` [rejected] — catalogue mixes COMP and CP for same comprimés
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 1 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. lost « CIC COMPRESSE OCULAIRE BT /10 »
- `flacon` → `fl` [rejected] — suppliers write flacon for the catalogue's FL
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `gouttes` → `gtt` [rejected] — suppliers write gouttes for the catalogue's GTT
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `vit` → `vitamine` [rejected] — catalogue writes VITAMINE, lines write VIT: SOLYNE VIT D3 vs VITAMINE D3 and VIT C vs VITAMINE C
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `bb` → `bebe` [live] — lines abbreviate BEBE as BB: HIPOSOL BB, SALISSE HUILE BB ROSE
  - evidence: seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « HIPOSOL SHAMPOING BB 250ML » → HIPOSOL SHAMP BEBE 250ML; « SALISSE HUILE BB ROSE 300 ML » → SALISSE HUILE BEBE ROSE 300 ML; « SHAMP BB HIPOSOL 250 ML » → HIPOSOL SHAMP BEBE 250ML
- `compresse` → `comp` [rejected] — catalogue COMP STERILLE vs line COMPRESSE STERILE: TECHNI PHARM and ALDISMED
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `solu` → `solution` [rejected] — DERMOCRYL SOLU vs SOLUTION 300 ML
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `plas` → `plastique` [rejected] — BIB PLAS vs BIB PLASTIQUE: KIDS HEAVEN
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 1 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 1 lost; e.g. lost « DENIZ BIB PLAS 240 ML A/B »
- `anti` → `a` [rejected] — catalogue A PAILLE / A GAZ vs line ANTI PAILLE / ANTI GAZ: GOURDE and BANDE
  - evidence: seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 1 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. lost « TALC ANTIPOUX EXPERTONE 100G »; « ABN BANDE ANTI GAZ 4M X 10 CM » → ABN BANDE A GAZ 4M X 10 CM; « KIDS HEAVEN GOURDE ANTI PAILLE » → KIDS HEAVEN GOURDE A PAILLE
- `adulte` → `ad` [live] — catalogue COUCHE AD vs line COUCHE ADULTE
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « COUCHE ADULTE UNI FORM LARGE CUL » → COUCHE AD UNI-FORM CULOTTES LARGE B/8
- `cul` → `culottes` [rejected] — catalogue CULOTTES vs line CUL: COUCHE UNI-FORM LARGE
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `b10` → `b/10` [rejected] — pack without slash LYN B10 vs B/10 so pack_count makes x10
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « LYN COMP OCULAIRE ENFANT B10 » → LYN COMP OCULAIRE ENFANT B/10
- `b30` → `b/30` [rejected] — pack without slash UPPER B30 vs B/30 and VITONIC /B30 so pack_count makes x30
  - evidence: seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; would make 1 more products read alike; e.g. « VITONIC GROSSESSE /B30 » → VITONIC GROSSESSE B/30; « UPPER FIL DENTAIRE B30 » → UPPER FIL DENTAIRE B/30
- `fl` → `` [rejected] — drop flacon filler FL/ FL that never names product: DERMOCRYL FL/300ML, CAMIDENT FL
  - evidence: refused: catalogue names use this word; dropping it would erase a difference between products
- `en` → `` [rejected] — drop French filler EN blocking exact match: PROPOLIS EN SPRAY vs PROPOLIS SPRAY
  - evidence: refused: catalogue names use this word; dropping it would erase a difference between products
- `eff` → `` [rejected] — drop EFF effervescent filler: SUPRADYN BOOST EFF vs BOOST B/10 where count unstated
  - evidence: refused: catalogue names use this word; dropping it would erase a difference between products
- `steriles` → `sterile` [rejected] — STERILE vs STERILES plural
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `steril` → `sterile` [rejected] — COMPRESSE STERIL typo
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `crm` → `creme` [live] — VG CRM = CREME
  - evidence: seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « VG CRM HYDRATANT JOUR 50 ML PH 5 » → VG CREME HYDRATANT JOUR 50 ML PH 5; « FIXYPLUS CRM ADHESIVE FRESH MINT » → FIXYPLUS CREME ADHESIVE FRESH MINT
- `magnesium` → `mag` [shadow] — DUO MAGNESIUM = DUO MAG
  - evidence: seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « JUVAMINE DUO MAGNESIUM B/30 » → JUVAMINE DUO MAG B/30; « BIONUTREX MAGNESIUM BISGLYCINATE » → BIONUTREX MAG BISGLYCINATE 600 MG + B1.B2.B6 B/20
- `gellule` → `gellules` [rejected] — GELLULE vs GELLULES
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « VITONIC MENO 45 GELLULE » → VITONIC MENO B/45 GELLULES
- `f` → `` [rejected] — F/09 ML vs 9ML flacon marker
  - evidence: refused: catalogue names use this word; dropping it would erase a difference between products
- `bouch` → `bouche` [rejected] — BAIN DE BOUCH = BOUCHE
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `grossess` → `grossesse` [rejected] — TEST GROSS ESS typo
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `ecran` → `ecra` [rejected] — VENUS ECRAN = ECRA
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « VENUS ECRAN TOTALE +40 » → VENUS ECRA TOTAL 40
- `totale` → `total` [rejected] — TOTALE vs TOTAL
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `cotonyde` → `cotonnhyde` [rejected] — COTONYDE typo for COTONNHYDE
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « COTON COTONYDE » → COTON COTONNHYDE 25 G
- `cheviller` → `chevillere` [rejected] — CHEVILLER typo
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « SAVET CHEVILLER REGLABLE » → SAVET CHEVILLERE REGLABLE
- `b18` → `x18` [rejected] — B18 = B/18 pack
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « VIEQUILIBRE MAG COMPLEX B18 » → VIEQUILIBRE MAG COMPLEX B/18
- `b100` → `x100` [rejected] — B100 = B/100 pack
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « SERINGUE NIPRO 2.5 CC B100 G22 INTRA MUSCULAIRE » → SERINGUE NIPRO 2.5 CC B/100 G22 INTRA MUSCULAIRE
- `09ml` → `9ml` [rejected] — F/09 ML vs 9ML leading zero
  - evidence: refused by the digit guard: it would change the digits of a word
- `vitamine` → `vit` [live] — catalogue VIT C, line VITAMINE C
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; e.g. « SOLYNE VITAMINE D3 1000UI » → SOLYNE VIT D3 1000 UI B/24
- `sterile` → `sterille` [rejected] — catalogue misspelling STERILLE, line STERILE
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `net` → `nettoyant` [rejected] — line ACM GEL NET vs GEL NETTOYANT
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « ACM ATOLYS GEL NET » → ACM ATOLYS GEL NETTOYANT SURGRAS 200 ML
- `pom` → `pd` [shadow] — line Articure Pom vs catalogue PD pommade
  - evidence: seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « Articure Pom 50G » → ARTICURE PD T/50 GR; « ARTICURE POM 50G » → ARTICURE PD T/50 GR
- `pde` → `creme` [shadow] — line NEOCAPSICUM PDE vs catalogue CREME pommade
  - evidence: seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « NEOCAPSICUM PDE » → NEOCAPSICUM CREME 100 G; « MOOV 50 G PDE » → MOOV CREME 50G

## Search method

**BM25 (brand ×2) + 0.5·trigrams + 0.5·word similarity** (Spark: bm25-brand-tri-wordsim)

## Rules (rule engineer and attributes)

- R-0008 [shadow] "citron", "orange", "pomme", "aloe vera" each name a flavour, in orders, invoices and catalogue names alike. — by rule-engineer
- R-0007 [shadow] "en" is never part of a product name. — by rule-engineer
- R-0006 [rejected] "{n}cap" is written as "{n} caps". — by rule-engineer (resolves only 1 new line(s) and brings 0 closer; the gate needs 2 resolved or 6 closer)
- R-0005 [shadow] "crem" means "creme". — by rule-engineer
- R-0004 [shadow] "bad" describes a product but never tells two products apart. — by rule-engineer
- R-0003 [shadow] "fl" describes a product but never tells two products apart. — by rule-engineer
- R-0002 [live] "fraise", "menthe", "mint", "vanille" each name a flavour, in orders, invoices and catalogue names alike; two texts whose flavour differs are never the same product. — by muse-spark-profile
- R-0001 [live] "bleu", "rouge", "noir", "vert", "rose", "gris", "jaune" each name a colour, in orders, invoices and catalogue names alike; two texts whose colour differs are never the same product. — by muse-spark-profile

## Phases

| phase | invoices | lines | free (alias + rules) | AI | asked | wrong auto | Spark agreed with code | Spark alone right | AI cost |
|---|---|---|---|---|---|---|---|---|---|
| spark-checks-all | 19 | 372 | 263 (71 %) | 41 (11 %) | 68 (18 %) | 0 | 263/263 | 41/41 | $0.1534 |
| learning | 11 | 207 | 158 (76 %) | 21 (10 %) | 28 (14 %) | 0 | 45/45 | 21/21 | $0.0571 |
| test after 0 | 10 | 191 | 140 (73 %) | 23 (12 %) | 27 (14 %) | 1 | 140/141 | 23/23 | $0.0787 |
| test after 10 | 10 | 191 | 139 (73 %) | 21 (11 %) | 30 (16 %) | 1 | 139/140 | 21/21 | $0.0833 |
| test after 20 | 10 | 191 | 146 (76 %) | 19 (10 %) | 25 (13 %) | 1 | 41/42 | 19/19 | $0.0457 |
| test after 30 | 10 | 191 | 149 (78 %) | 17 (9 %) | 24 (13 %) | 1 | 45/46 | 17/17 | $0.0462 |

## Calibrations

- **calibration after 10 invoices** (143 seen + 48 unseen lines): 110/143 right with no AI (77 %), 0 wrong → 111/143 right with no AI (78 %), 0 wrong; accepted 1, rejected 24; search: BM25 (brand ×2) + 0.5·trigrams: right product in the top 8 for 96.5 % of lines (MRR 0.94) → BM25 (brand ×1.5) + trigrams: 96.5 % in the top 8 (MRR 0.94); engineer rules []; $0.2286. Spark: catalogue abbreviates opposite to lines VIT/SHAMP/COMP/AD/A vs VITAMINE/SHAMPOOING/COMPRESSE/ADULTE/ANTI and packs B/10 vs B10 blocking exact match
- **calibration after 20 invoices** (293 seen + 98 unseen lines): 219/293 right with no AI (75 %), 0 wrong → 222/293 right with no AI (76 %), 0 wrong; accepted 1, rejected 18; search: BM25 (brand ×1.5) + trigrams: right product in the top 8 for 95.6 % of lines (MRR 0.93) → BM25 (brand ×2) + 0.5·trigrams + 0.5·word similarity: 95.6 % in the top 8 (MRR 0.93); engineer rules []; $0.1912. Spark: abbreviations SHAMP COMP BB CREM FL and bare packs B10/B30 block exact match, plus typos and leading-zero sizes block ranking
- **calibration after 30 invoices** (434 seen + 145 unseen lines): 329/434 right with no AI (76 %), 0 wrong → 332/434 right with no AI (76 %), 0 wrong; accepted 2, rejected 10; search: BM25 (brand ×2) + 0.5·trigrams + 0.5·word similarity: right product in the top 8 for 95.4 % of lines (MRR 0.94) → BM25 (brand ×2) + 0.5·trigrams + 0.5·word similarity: 95.4 % in the top 8 (MRR 0.93); engineer rules [R-0003 R-0004 R-0005 R-0007 R-0008]; $0.5616. Spark: packs decide variants but bare B10/B30/72P miss x10/x30, plus supplier abbreviations vs catalogue BAD SHAMP VIT MAG COMP PD CREME need lexicon merges, GR/ML contradictions left to human

## Experiment log

- [bootstrap] block · accepted: split_glued (reads 0 catalogue names differently; products that read alike 13 → 13, by muse-spark) — lines glue count to form like 30COMP, 30TAB, B100; must split to 30 comp to match spaced catalogue form
- [bootstrap] block · accepted: dimensions (reads 1 catalogue names differently; products that read alike 13 → 13, by muse-spark) — bandages and plasters written 4M X 10 CM, 10 CM X 4 M, 10 CM X 2.5 M; pair must become one token so 4x10 never matches 4x7
- [bootstrap] block · accepted: units_glue (reads 19 catalogue names differently; products that read alike 13 → 12, by muse-spark) — needle and bandage lengths 5 MM, 3.5 MM, 10 CM, 4M written spaced or glued; glue to 5mm so spaced and glued forms match
- [bootstrap] block · accepted: pack_count (reads 146 catalogue names differently; products that read alike 12 → 12, by muse-spark) — B/10, B/20, B/100, P/30, C/120, C/18, B/6 are the pack size and differ between otherwise identical products; must become x10, x30 etc.
- [bootstrap] lexicon · accepted: "biberon" → "bib" (proposed before any invoice (reads 3 catalogue names differently; products that read alike 12 → 12): waits in shadow until known lines prove it, by muse-spark) — suppliers write biberon for the catalogue's BIB
- [bootstrap] lexicon · accepted: "brosse" → "bad" (proposed before any invoice (reads 1 catalogue names differently; products that read alike 12 → 12): waits in shadow until known lines prove it, by muse-spark) — suppliers write brosse a dents for the catalogue's BAD
- [bootstrap] lexicon · accepted: "dentifrice" → "dent" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 12 → 12): waits in shadow until known lines prove it, by muse-spark) — suppliers write dentifrice for the catalogue's DENT
- [bootstrap] lexicon · accepted: "shampooing" → "shamp" (proposed before any invoice (reads 2 catalogue names differently; products that read alike 12 → 12): waits in shadow until known lines prove it, by muse-spark) — suppliers write shampooing for the catalogue's SHAMP
- [bootstrap] lexicon · accepted: "shampoing" → "shamp" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 12 → 12): waits in shadow until known lines prove it, by muse-spark) — alternate spelling suppliers use for the catalogue's SHAMP
- [bootstrap] lexicon · accepted: "crem" → "creme" (proposed before any invoice (reads 1 catalogue names differently; products that read alike 12 → 12): waits in shadow until known lines prove it, by muse-spark) — catalogue mostly writes creme, variant CREM must normalise
- [bootstrap] lexicon · accepted: "comprime" → "cp" (proposed before any invoice (reads 1 catalogue names differently; products that read alike 12 → 12): waits in shadow until known lines prove it, by muse-spark) — suppliers write comprime for the catalogue's CP
- [bootstrap] lexicon · accepted: "comp" → "cp" (proposed before any invoice (reads 11 catalogue names differently; products that read alike 12 → 12): waits in shadow until known lines prove it, by muse-spark) — catalogue mixes COMP and CP for same comprimés
- [bootstrap] lexicon · accepted: "flacon" → "fl" (proposed before any invoice (reads 1 catalogue names differently; products that read alike 12 → 12): waits in shadow until known lines prove it, by muse-spark) — suppliers write flacon for the catalogue's FL
- [bootstrap] lexicon · accepted: "gouttes" → "gtt" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 12 → 12): waits in shadow until known lines prove it, by muse-spark) — suppliers write gouttes for the catalogue's GTT
- [bootstrap] search · accepted: BM25 (brand ×2) + 0.5·trigrams (not measured yet: no line has a known answer, by muse-spark) — 269 brands make brand decisive, bm25 on rare dosage and function words; trigram tolerates typos like JOHENSON/JOHNSON and glued 250ML/30COMP; wordsim 0 because digit words must match exactly and prefix similarity would confuse dosages and dimensions
- [bootstrap] attribute · accepted: "bleu", "rouge", "noir", "vert", "rose", "gris", "jaune" each name a colour, in orders, invoices and catalogue names alike; two texts whose colour differs are never the same product. (reads 34 catalogue names, 0 pairs made alike → shadow, by muse-spark) — masks, toothbrushes, bottles differ only by colour e.g. BLEU vs VERT vs ROUGE, both sides state it
- [bootstrap] attribute · accepted: "fraise", "menthe", "mint", "vanille" each name a flavour, in orders, invoices and catalogue names alike; two texts whose flavour differs are never the same product. (reads 9 catalogue names, 0 pairs made alike → shadow, by muse-spark) — mouthwash, toothpaste, condoms differ by parfum e.g. FRAISE vs MENTHE vs VANILLE
- [bootstrap] attribute · rejected: size (a size value is {size} or a size such as 1500ml, by muse-spark) — devices, hot-cold poche, semelles differ by SMALL/MEDIUM/LARGE/XL/GM/PM
- [calibration after 10 invoices] block · rejected: dimensions (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — bandage sizes 4M X 10 CM, 5X5, 10X10 decide variants and order varies
- [calibration after 10 invoices] lexicon · rejected: "vit" → "vitamine" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — catalogue writes VITAMINE, lines write VIT: SOLYNE VIT D3 vs VITAMINE D3 and VIT C vs VITAMINE C
- [calibration after 10 invoices] lexicon · rejected: "bb" → "bebe" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « HIPOSOL SHAMPOING BB 250ML » → HIPOSOL SHAMP BEBE 250ML; « SALISSE HUILE BB ROSE 300 ML » → SALISSE HUILE BEBE ROSE 300 ML, by muse-spark) — lines abbreviate BEBE as BB: HIPOSOL BB, SALISSE HUILE BB ROSE
- [calibration after 10 invoices] lexicon · rejected: "compresse" → "comp" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — catalogue COMP STERILLE vs line COMPRESSE STERILE: TECHNI PHARM and ALDISMED
- [calibration after 10 invoices] lexicon · rejected: "solu" → "solution" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — DERMOCRYL SOLU vs SOLUTION 300 ML
- [calibration after 10 invoices] lexicon · rejected: "plas" → "plastique" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 1 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 1 lost; e.g. lost « DENIZ BIB PLAS 240 ML A/B », by muse-spark) — BIB PLAS vs BIB PLASTIQUE: KIDS HEAVEN
- [calibration after 10 invoices] lexicon · rejected: "anti" → "a" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 1 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. lost « TALC ANTIPOUX EXPERTONE 100G »; « ABN BANDE ANTI GAZ 4M X 10 CM » → ABN BANDE A GAZ 4M X 10 CM; « KIDS HEAVEN GOURDE ANTI PAILLE » → KIDS HEAVEN GOURDE A PAILLE, by muse-spark) — catalogue A PAILLE / A GAZ vs line ANTI PAILLE / ANTI GAZ: GOURDE and BANDE
- [calibration after 10 invoices] lexicon · accepted: "adulte" → "ad" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « COUCHE ADULTE UNI FORM LARGE CUL » → COUCHE AD UNI-FORM CULOTTES LARGE B/8, by muse-spark) — catalogue COUCHE AD vs line COUCHE ADULTE
- [calibration after 10 invoices] lexicon · rejected: "cul" → "culottes" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — catalogue CULOTTES vs line CUL: COUCHE UNI-FORM LARGE
- [calibration after 10 invoices] lexicon · rejected: "b10" → "b/10" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « LYN COMP OCULAIRE ENFANT B10 » → LYN COMP OCULAIRE ENFANT B/10, by muse-spark) — pack without slash LYN B10 vs B/10 so pack_count makes x10
- [calibration after 10 invoices] lexicon · rejected: "b30" → "b/30" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; would make 1 more products read alike; e.g. « VITONIC GROSSESSE /B30 » → VITONIC GROSSESSE B/30; « UPPER FIL DENTAIRE B30 » → UPPER FIL DENTAIRE B/30, by muse-spark) — pack without slash UPPER B30 vs B/30 and VITONIC /B30 so pack_count makes x30
- [calibration after 10 invoices] lexicon · rejected: "fl" → "" (refused: catalogue names use this word; dropping it would erase a difference between products, by muse-spark) — drop flacon filler FL/ FL that never names product: DERMOCRYL FL/300ML, CAMIDENT FL
- [calibration after 10 invoices] lexicon · rejected: "en" → "" (refused: catalogue names use this word; dropping it would erase a difference between products, by muse-spark) — drop French filler EN blocking exact match: PROPOLIS EN SPRAY vs PROPOLIS SPRAY
- [calibration after 10 invoices] lexicon · rejected: "eff" → "" (refused: catalogue names use this word; dropping it would erase a difference between products, by muse-spark) — drop EFF effervescent filler: SUPRADYN BOOST EFF vs BOOST B/10 where count unstated
- [calibration after 10 invoices] lexicon · rejected: "biberon" → "bib" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 1 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. lost « DENIZ BIB PLAS 240 ML A/B », by muse-spark) — suppliers write biberon for the catalogue's BIB
- [calibration after 10 invoices] lexicon · rejected: "brosse" → "bad" (from shadow) (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « BROSSE KIT 6 PCS » → BAD KIT 6 PCS, by muse-spark) — suppliers write brosse a dents for the catalogue's BAD
- [calibration after 10 invoices] lexicon · rejected: "dentifrice" → "dent" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write dentifrice for the catalogue's DENT
- [calibration after 10 invoices] lexicon · rejected: "shampooing" → "shamp" (from shadow) (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « JOHENSONS SHAMPOOING JAUNE 300 ML » → JOHENSONS SHAMP JAUNE 300 ML, by muse-spark) — suppliers write shampooing for the catalogue's SHAMP
- [calibration after 10 invoices] lexicon · rejected: "shampoing" → "shamp" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — alternate spelling suppliers use for the catalogue's SHAMP
- [calibration after 10 invoices] lexicon · rejected: "crem" → "creme" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — catalogue mostly writes creme, variant CREM must normalise
- [calibration after 10 invoices] lexicon · rejected: "comprime" → "cp" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write comprime for the catalogue's CP
- [calibration after 10 invoices] lexicon · rejected: "comp" → "cp" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 1 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. lost « CIC COMPRESSE OCULAIRE BT /10 », by muse-spark) — catalogue mixes COMP and CP for same comprimés
- [calibration after 10 invoices] lexicon · rejected: "flacon" → "fl" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write flacon for the catalogue's FL
- [calibration after 10 invoices] lexicon · rejected: "gouttes" → "gtt" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write gouttes for the catalogue's GTT
- [calibration after 10 invoices] lexicon · rejected: "bb" → "bebe" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « HIPOSOL SHAMPOING BB 250ML » → HIPOSOL SHAMP BEBE 250ML; « SALISSE HUILE BB ROSE 300 ML » → SALISSE HUILE BEBE ROSE 300 ML, by muse-spark) — lines abbreviate BEBE as BB: HIPOSOL BB, SALISSE HUILE BB ROSE
- [calibration after 10 invoices] search · accepted: BM25 + trigrams [BM25 (brand ×1.5) + trigrams] (classic word similarity [word similarity]: 96.5 % in top 8, MRR 0.93; BM25 [BM25]: 95.8 % in top 8, MRR 0.92; BM25, brand ×2 [BM25 (brand ×2)]: 95.8 % in top 8, MRR 0.92; trigrams [trigrams]: 96.5 % in top 8, MRR 0.93; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 96.5 % in top 8, MRR 0.94; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 96.5 % in top 8, MRR 0.94; Spark: bm25-brand-trigram [BM25 (brand ×2) + 0.5·trigrams]: 96.5 % in top 8, MRR 0.94; Spark: typo-tolerant [BM25 (brand ×2) + trigrams + 0.5·word similarity]: 96.5 % in top 8, MRR 0.94; Spark: pack-recall [0.7·BM25 (brand ×3) + 0.8·trigrams + 0.8·word similarity]: 96.5 % in top 8, MRR 0.94; current: Spark's first guess [BM25 (brand ×2) + 0.5·trigrams]: 96.5 % in top 8, MRR 0.94. Chosen: 96.5 % in top 8 on seen lines; on unseen lines 97.9 % vs 97.9 % for the current method, by library) — 
- [calibration after 10 invoices] rekey · accepted: re-read every known wording (3 alias(es) moved to the new reading, no collision, by code) — 
- [after 19 invoices] autonomy · accepted: the desk's steps earned autonomy: Spark stops checking every answer (272 code answers confirmed, 272 right (95 % lower bound 99.01 % ≥ bar 99 %). While supervised: 372 lines, Spark agreed with the code on 263 (263 right), disagreed on 1; alone on 41, right on 41, by code) — 
- [calibration after 20 invoices] block · rejected: dimensions (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — bandage 4m x 10cm order varies, need 10x10 token
- [calibration after 20 invoices] lexicon · rejected: "steriles" → "sterile" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — STERILE vs STERILES plural
- [calibration after 20 invoices] lexicon · rejected: "steril" → "sterile" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — COMPRESSE STERIL typo
- [calibration after 20 invoices] lexicon · rejected: "crm" → "creme" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « VG CRM HYDRATANT JOUR 50 ML PH 5 » → VG CREME HYDRATANT JOUR 50 ML PH 5; « FIXYPLUS CRM ADHESIVE FRESH MINT » → FIXYPLUS CREME ADHESIVE FRESH MINT, by muse-spark) — VG CRM = CREME
- [calibration after 20 invoices] lexicon · rejected: "magnesium" → "mag" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « JUVAMINE DUO MAGNESIUM B/30 » → JUVAMINE DUO MAG B/30; « BIONUTREX MAGNESIUM BISGLYCINATE » → BIONUTREX MAG BISGLYCINATE 600 MG + B1.B2.B6 B/20, by muse-spark) — DUO MAGNESIUM = DUO MAG
- [calibration after 20 invoices] lexicon · rejected: "gellule" → "gellules" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « VITONIC MENO 45 GELLULE » → VITONIC MENO B/45 GELLULES, by muse-spark) — GELLULE vs GELLULES
- [calibration after 20 invoices] lexicon · rejected: "f" → "" (refused: catalogue names use this word; dropping it would erase a difference between products, by muse-spark) — F/09 ML vs 9ML flacon marker
- [calibration after 20 invoices] lexicon · rejected: "bouch" → "bouche" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — BAIN DE BOUCH = BOUCHE
- [calibration after 20 invoices] lexicon · rejected: "grossess" → "grossesse" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — TEST GROSS ESS typo
- [calibration after 20 invoices] lexicon · rejected: "ecran" → "ecra" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « VENUS ECRAN TOTALE +40 » → VENUS ECRA TOTAL 40, by muse-spark) — VENUS ECRAN = ECRA
- [calibration after 20 invoices] lexicon · rejected: "totale" → "total" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — TOTALE vs TOTAL
- [calibration after 20 invoices] lexicon · rejected: "cotonyde" → "cotonnhyde" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « COTON COTONYDE » → COTON COTONNHYDE 25 G, by muse-spark) — COTONYDE typo for COTONNHYDE
- [calibration after 20 invoices] lexicon · rejected: "cheviller" → "chevillere" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « SAVET CHEVILLER REGLABLE » → SAVET CHEVILLERE REGLABLE, by muse-spark) — CHEVILLER typo
- [calibration after 20 invoices] lexicon · rejected: "b18" → "x18" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « VIEQUILIBRE MAG COMPLEX B18 » → VIEQUILIBRE MAG COMPLEX B/18, by muse-spark) — B18 = B/18 pack
- [calibration after 20 invoices] lexicon · rejected: "b100" → "x100" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « SERINGUE NIPRO 2.5 CC B100 G22 INTRA MUSCULAIRE » → SERINGUE NIPRO 2.5 CC B/100 G22 INTRA MUSCULAIRE, by muse-spark) — B100 = B/100 pack
- [calibration after 20 invoices] lexicon · rejected: "09ml" → "9ml" (refused by the digit guard: it would change the digits of a word, by muse-spark) — F/09 ML vs 9ML leading zero
- [calibration after 20 invoices] lexicon · accepted: "bb" → "bebe" (from shadow) (seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « HIPOSOL SHAMPOING BB 250ML » → HIPOSOL SHAMP BEBE 250ML; « SALISSE HUILE BB ROSE 300 ML » → SALISSE HUILE BEBE ROSE 300 ML; « SHAMP BB HIPOSOL 250 ML » → HIPOSOL SHAMP BEBE 250ML, by muse-spark) — lines abbreviate BEBE as BB: HIPOSOL BB, SALISSE HUILE BB ROSE
- [calibration after 20 invoices] lexicon · rejected: "crm" → "creme" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « VG CRM HYDRATANT JOUR 50 ML PH 5 » → VG CREME HYDRATANT JOUR 50 ML PH 5; « FIXYPLUS CRM ADHESIVE FRESH MINT » → FIXYPLUS CREME ADHESIVE FRESH MINT, by muse-spark) — VG CRM = CREME
- [calibration after 20 invoices] lexicon · rejected: "magnesium" → "mag" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « JUVAMINE DUO MAGNESIUM B/30 » → JUVAMINE DUO MAG B/30; « BIONUTREX MAGNESIUM BISGLYCINATE » → BIONUTREX MAG BISGLYCINATE 600 MG + B1.B2.B6 B/20, by muse-spark) — DUO MAGNESIUM = DUO MAG
- [calibration after 20 invoices] search · accepted: Spark: bm25-brand-tri-wordsim [BM25 (brand ×2) + 0.5·trigrams + 0.5·word similarity] (classic word similarity [word similarity]: 95.2 % in top 8, MRR 0.91; BM25 [BM25]: 94.2 % in top 8, MRR 0.90; BM25, brand ×2 [BM25 (brand ×2)]: 94.2 % in top 8, MRR 0.90; trigrams [trigrams]: 95.6 % in top 8, MRR 0.92; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 95.6 % in top 8, MRR 0.93; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 95.6 % in top 8, MRR 0.93; Spark: bm25-brand-tri [BM25 (brand ×2) + 0.7·trigrams]: 95.6 % in top 8, MRR 0.93; Spark: bm25-brand-tri-wordsim [BM25 (brand ×2) + 0.5·trigrams + 0.5·word similarity]: 95.6 % in top 8, MRR 0.93; Spark: bm25-brand-high [BM25 (brand ×3) + trigrams]: 95.6 % in top 8, MRR 0.93; current: BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 95.6 % in top 8, MRR 0.93. Chosen: 95.6 % in top 8 on seen lines; on unseen lines 93.9 % vs 93.9 % for the current method, by muse-spark) — wordsim for cotonyde cheviller sterile variants
- [calibration after 20 invoices] rekey · accepted: re-read every known wording (4 alias(es) moved to the new reading, no collision, by code) — 
- [calibration after 30 invoices] block · rejected: dimensions (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — bandage dimensions 4M X 10 CM with order varying must stay one token 4x10
- [calibration after 30 invoices] block · rejected: strength (bare) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 1 lost, by muse-spark) — pharmacy two-value dosages written bare 5/160, normalizes to dose token without affecting single mg
- [calibration after 30 invoices] block · rejected: shade (seen lines: +0 newly resolved (0 distinct wordings), 1 wrong, 1 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « VENUS ECRAN TOTALE +40 » → VENUS ECRAN TOTAL TEINTE 60 instead of VENUS ECRA TOTAL 40; lost « Venus 60 Ecran Total Teinte », by muse-spark) — colour flavour size variants for toothbrushes condoms cosmetics N02/teinte/#2 -> n2, safe normalization
- [calibration after 30 invoices] lexicon · accepted: "vitamine" → "vit" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; e.g. « SOLYNE VITAMINE D3 1000UI » → SOLYNE VIT D3 1000 UI B/24, by muse-spark) — catalogue VIT C, line VITAMINE C
- [calibration after 30 invoices] lexicon · rejected: "sterile" → "sterille" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — catalogue misspelling STERILLE, line STERILE
- [calibration after 30 invoices] lexicon · rejected: "net" → "nettoyant" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « ACM ATOLYS GEL NET » → ACM ATOLYS GEL NETTOYANT SURGRAS 200 ML, by muse-spark) — line ACM GEL NET vs GEL NETTOYANT
- [calibration after 30 invoices] lexicon · rejected: "pom" → "pd" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « Articure Pom 50G » → ARTICURE PD T/50 GR; « ARTICURE POM 50G » → ARTICURE PD T/50 GR, by muse-spark) — line Articure Pom vs catalogue PD pommade
- [calibration after 30 invoices] lexicon · rejected: "pde" → "creme" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « NEOCAPSICUM PDE » → NEOCAPSICUM CREME 100 G; « MOOV 50 G PDE » → MOOV CREME 50G, by muse-spark) — line NEOCAPSICUM PDE vs catalogue CREME pommade
- [calibration after 30 invoices] lexicon · accepted: "crm" → "creme" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « VG CRM HYDRATANT JOUR 50 ML PH 5 » → VG CREME HYDRATANT JOUR 50 ML PH 5; « FIXYPLUS CRM ADHESIVE FRESH MINT » → FIXYPLUS CREME ADHESIVE FRESH MINT, by muse-spark) — VG CRM = CREME
- [calibration after 30 invoices] lexicon · rejected: "magnesium" → "mag" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « JUVAMINE DUO MAGNESIUM B/30 » → JUVAMINE DUO MAG B/30; « BIONUTREX MAGNESIUM BISGLYCINATE » → BIONUTREX MAG BISGLYCINATE 600 MG + B1.B2.B6 B/20, by muse-spark) — DUO MAGNESIUM = DUO MAG
- [calibration after 30 invoices] lexicon · rejected: "pom" → "pd" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « Articure Pom 50G » → ARTICURE PD T/50 GR; « ARTICURE POM 50G » → ARTICURE PD T/50 GR, by muse-spark) — line Articure Pom vs catalogue PD pommade
- [calibration after 30 invoices] lexicon · rejected: "pde" → "creme" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « NEOCAPSICUM PDE » → NEOCAPSICUM CREME 100 G; « MOOV 50 G PDE » → MOOV CREME 50G, by muse-spark) — line NEOCAPSICUM PDE vs catalogue CREME pommade
- [calibration after 30 invoices] search · accepted: current: Spark: bm25-brand-tri-wordsim [BM25 (brand ×2) + 0.5·trigrams + 0.5·word similarity] (classic word similarity [word similarity]: 95.2 % in top 8, MRR 0.92; BM25 [BM25]: 94.5 % in top 8, MRR 0.91; BM25, brand ×2 [BM25 (brand ×2)]: 94.5 % in top 8, MRR 0.91; trigrams [trigrams]: 95.2 % in top 8, MRR 0.92; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 95.2 % in top 8, MRR 0.93; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 95.4 % in top 8, MRR 0.93; Spark: pack-fuzzy trigram [BM25 (brand ×2) + trigrams + 0.5·word similarity]: 95.4 % in top 8, MRR 0.93; Spark: balanced recall [BM25 + 0.8·trigrams + 0.8·word similarity]: 95.4 % in top 8, MRR 0.93; Spark: typo tolerant [BM25 (brand ×2) + 0.7·trigrams + word similarity]: 95.4 % in top 8, MRR 0.93; current: Spark: bm25-brand-tri-wordsim [BM25 (brand ×2) + 0.5·trigrams + 0.5·word similarity]: 95.4 % in top 8, MRR 0.93. Chosen: 95.4 % in top 8 on seen lines; on unseen lines 96.6 % vs 96.6 % for the current method, by library) — wordsim for cotonyde cheviller sterile variants
- [calibration after 30 invoices] rekey · accepted: re-read every known wording (11 alias(es) moved to the new reading, no collision, by code) — 

Meta model spend for the whole run: $1.4705

## Who made the free answers possible (test invoices)

578 free answers: Muse Spark found 18 (3 %), the matching Muse Spark designed gave 560 (97 %), a person taught 0 (0 %).

- catalogue match through the matching Muse Spark designed: 560
- rule Muse Spark wrote: 18
