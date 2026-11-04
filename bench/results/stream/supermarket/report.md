# What Muse Spark identified: supermarket / FMCG

Names are French commercial phrases with brand first then type then variant, but word order is unstable (BISCUIT SANDWICH vs SANDWICH BISCUIT). Sizes are food units written glued with no space (380G, 1KG, 1.8KG, 75CL, 125ML, 1L, 2L) and normalized by clean-up. Packs are written as number+P (3P, 6P, 8P, 10P, 14P, 16P, 120P, 288P) or X+n (X4) and cahier format as 17X22. Within one brand products are told apart by flavour / filling (chocolat, lait, white, vanille, fraise, abricot, pistache, thon), by weight/volume, and by pack count.

## First-pass blocks

- **split_glued** ON — invoice lines often glue brand/type to size/count with no space (yaourt1l, koolx4); splits 4+ letter words so codes stay whole
  - evidence: reads 2 catalogue names differently; products that read alike 13 → 13
- **dimensions** ON — stationery cahiers write format 17X22 / 17 X 22; must stay one token to avoid confusion with packs
  - evidence: reads 1 catalogue names differently; products that read alike 13 → 13
- **units_glue** ON — few non-food lines (LAMPE LED 12W, AMPOULE 7W) write watts with/without space; glues number+technical unit so 12W equals 12 W
  - evidence: reads 1 catalogue names differently; products that read alike 13 → 13
- **pack_count** ON — pack count is decisive: LA VACHE QUI RIT 8P vs plain, PICON 16P, KOOL ROUND BREAK 6P, KOOL X4, GANT 10P; normalizes P/X forms to xN
  - evidence: reads 0 catalogue names differently; products that read alike 13 → 13

## The desk's own vocabulary (lexicon)

- `choco` → `chocolat` [live] — suppliers write choco for the catalogue's chocolat
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « STORY BIFA CHOCO OFFRE » → BIFA STORY CHOCOLAT OFFRE SPECIALE
- `conf` → `confiture` [rejected] — suppliers write conf for the catalogue's confiture
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `veg` → `vegetal` [rejected] — suppliers write veg for the catalogue's vegetal
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `prep` → `preparation` [rejected] — suppliers write prep for the catalogue's preparation
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `bte` → `boite` [rejected] — suppliers write bte for the catalogue's boite
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `كراس` → `cahier` [rejected] — arabic for cahier: RAYANE/EL HILLAL lines
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « كراس EL HILLAL 32P » → EL HILLAL CAHIER 32 PAGES PIQUE PF 32P
- `عجائن` → `pates` [rejected] — arabic for pates: SAFINA/SOSEMIE lines
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `عجاين` → `pates` [live] — variant spelling of عجائن seen in misses
  - evidence: seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « عجائن SEMOULERIE AMOUR 3.5KG » → SEMOULERIE AMOUR PACK PATES ALIMENTAIRES COUDE 3.5KG; « عجائن EL MAICENNE PATES » → EL MAICENNE PATES PATE ALIMENTAIRE
- `شامبو` → `shampooing` [rejected] — HACHFA TEMPO SHAMPOOING
  - evidence: seen lines: +2 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « شامبو HACHFA » → HACHFA TEMPO SHAMPOOING; « شامبو HACHFA » → HACHFA TEMPO SHAMPOOING
- `حليب` → `lait` [live] — milk: RAMY MILKY AU LAIT
  - evidence: seen lines: +3 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « حليب RAMY MILKY KIDS » → RAMY MILKY KIDS BOISSON FRUITEE AU LAIT; « حليب COCOATY SPORT » → COCOATY SPORT CHOCOLAT AU LAIT MINI TABLETTE; « حليب RAMY MILKY KIDS » → RAMY MILKY KIDS BOISSON FRUITEE AU LAIT
- `بسكويت` → `biscuit` [rejected] — CASA GLACES/PALMARY BISCUIT
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost
- `هريسة` → `harissa` [rejected] — CB HARISSA 2KG
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `هريسه` → `harissa` [live] — ta-marbuta variant of harissa
  - evidence: seen lines: +4 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « هريسة CB 2KG » → CB HARISSA 2KG; « هريسة CB 2KG » → CB HARISSA 2KG; « هريسة CB 2KG » → CB HARISSA 2KG
- `قفازات` → `gant` [rejected] — GANT DE MENAGE
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 1 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « قفازات ASRAR PARAPHAR » → ASRAR PARAPHAR GLOVE NOIR L GANT 100P instead of ASRAR PARAPHAR GANTS NOIR TAILLE L GLOVE
- `جبن` → `cheese` [rejected] — SPEEDO CHEESE PREPARATION FROMAGERE
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `مثلجات` → `glacee` [rejected] — PLAYTA SUCETTE GLACEE
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « مثلجات PLAYTA » → PLAYTA SUCETTE GLACEE LUXE SURPRISE
- `صلصة` → `sauce` [rejected] — FOODY'S SAUCE ALGERIENNE
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `عصير` → `jus` [rejected] — FRUIT MAX JUS 2L
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « عصير FRUIT MAX 2L » → FRUIT MAX JUS 2L
- `ياغورت` → `yaourt` [rejected] — ACTIVIA YAOURT
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « ياغورت COQUILLAGE » → COQUILLAGE YAOURT AROMATISE
- `سكر` → `sucre` [live] — ILYIA SUCRE
  - evidence: seen lines: +4 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « سكر ILYIA » → ILYIA SUCRE SWEETY; « سكر ILYIA » → ILYIA SUCRE SWEETY; « سكر ILYIA » → ILYIA SUCRE SWEETY
- `ماء` → `eau` [rejected] — WIPPRO AVEC L'EAU JAVEL
  - evidence: seen lines: +2 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « ماء WIPPRO » → WIPPRO LINGETTES NETTOYANTES ET DESINFECTANTE AVEC L'EAU JAVEL; « ماء WIPPRO » → WIPPRO LINGETTES NETTOYANTES ET DESINFECTANTE AVEC L'EAU JAVEL
- `ليمونادة` → `limonade` [rejected] — SPRITE LIMONADE
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `منتوج` → `` [rejected] — drop generic sugary-drink description words
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `مسكر` → `` [rejected] — drop generic sugary-drink description words
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `جزئيا` → `` [rejected] — drop generic sugary-drink description words
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `cr` → `creme` [live] — BEN AROUS CR CHANTILLY / SPIDER CREME WAX
  - evidence: seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « BEN AROUS CR CHANTILLY » → BEN AROUS CREME CHANTILLY CRUNCHY PISTACHE; « CR DOUBLE SENS SPIDER WAX » → DOUBLE SENS SPIDER CREME WAX; « CR CHANTILLY CRUNCHY BEN AROUS » → BEN AROUS CREME CHANTILLY CRUNCHY PISTACHE
- `liq` → `liquide` [rejected] — KABTON MIEL LIQ -> LIQUIDE
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `frmg` → `fromage` [live] — TIP TOP FRMG FONDU
  - evidence: seen lines: +4 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « FRMG FONDU SLICE MILKY COW » → MILKY COW FROMAGE FONDU SLICE; « FRMG SANDWICH FONDU 330GX48 » → SANDWICH FROMAGE FONDU 330G; « FRMG FONDU LAGRATANIA 900GX24 » → LAGRATANIA FROMAGE FONDU 900G
- `yaou` → `yaourt` [rejected] — YAOU SWEETLE / ACTIVIA YAOU
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `frs` → `fraise` [rejected] — SWEETLE FRAISE
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « YAOU SWEETLE FRS » → SWEETLE YAOURT AROMATISE FRAISE
- `pate` → `pates` [live] — singular/plural: PATE LONGUES -> PATES LONGUES
  - evidence: seen lines: +7 newly resolved (4 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « PATE LONGUES SOSEMIE » → SOSEMIE PATES LONGUES; « SEMOULERIE AMOUR PACK PATE 3.5KG » → SEMOULERIE AMOUR PACK PATES ALIMENTAIRES COUDE 3.5KG; « SEMOULERIE AMOUR PATE ALIMENTAIRES PENNE » → SEMOULERIE AMOUR PATES ALIMENTAIRES PENNE
- `صلصه` → `sauce` [rejected] — ta-marbuta variant
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `خردل` → `moutarde` [rejected] — خردل MOUTARDE -> MOUTARDE GM
  - evidence: seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « خردل DAILY SAUCE » → DAILY SAUCE MOUTARDE
- `ليموناده` → `limonade` [rejected] — variant spelling
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `محلى` → `` [rejected] — drop filler
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `قهوة` → `cafe` [rejected] — HOUSE/FAMICO cafe
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost
- `monlait` → `lait` [rejected] — CHEDDAR MONLAIT is lait, lets حليب match
  - evidence: seen lines: +3 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « حليب CHEDDAR » → CHEDDAR MONLAIT; « حليب CHEDDAR » → CHEDDAR MONLAIT; « حليب CHEDDAR » → CHEDDAR MONLAIT
- `ومحلى` → `` [rejected] — drop generic description word
  - evidence: seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost

## Search method

**BM25 (brand ×1.5) + trigrams** (BM25 + trigrams)

## Rules (rule engineer and attributes)

- R-0012 [rejected] "beurre" is the variant "beurre", in orders, invoices and catalogue names alike; a variant stated on one side only, or a different one, is never the same product. — by rule-engineer (a marker needs a plain product beside the variant: 2 of 3 products it marks have none (e.g. HEY BISCUIT SEC PETIT BEURRE 5P))
- R-0011 [shadow] "جبن" or "فرينه" or "خل" or "منظف" or "خردل" or "كراس" or "بيري" describes a product but never tells two products apart. — by rule-engineer
- R-0010 [rejected] "white" is the variant "white", in orders, invoices and catalogue names alike; a variant stated on one side only, or a different one, is never the same product. — by rule-engineer (a marker needs a plain product beside the variant: 2 of 3 products it marks have none (e.g. 4 WINNERS WHITE BISCUIT ENROBE KOOL))
- R-0009 [rejected] "frs" means "fraise". — by rule-engineer (the breaker wrote "ARWA GAZEUSE CITRON 2L FRS" (meaning ARWA BOISSON GAZIFIE CITRON 2L) and the rule booked it as BARIGO BOISSON GAZIFIE ARWA GAZEUSE 2L)
- R-0008 [live] "vngr" means "vinaigre". — by rule-engineer
- R-0007 [rejected] "جبن" means "fromage". — by rule-engineer (resolves only 0 new line(s) and brings 2 closer; the gate needs 2 resolved or 6 closer)
- R-0006 [shadow] "حليب" means "lait". — by rule-engineer
- R-0005 [rejected] "بسكويت" means "biscuit". — by rule-engineer (resolves only 1 new line(s) and brings 3 closer; the gate needs 2 resolved or 6 closer)
- R-0004 [rejected] "ياغورت" means "yaourt". — by rule-engineer (resolves only 1 new line(s) and brings 3 closer; the gate needs 2 resolved or 6 closer)
- R-0003 [rejected] "choc" means "chocolat". — by rule-engineer (resolves only 1 new line(s) and brings 0 closer; the gate needs 2 resolved or 6 closer)
- R-0002 [rejected] "sans", "light", "zero", "diet" each name a sans_light, in orders, invoices and catalogue names alike; a sans_light stated on one side only, or a different one, is never the same product. — by muse-spark-profile (a marker needs a plain product beside the variant: 5 of 6 products it marks have none (e.g. MR RUSTICA LA PUREE SANS GLUTEN))
- R-0001 [live] "chocolat", "vanille", "fraise", "abricot", "pistache", "caramel", "thon", "ananas", "menthe", "citron" each name a saveur_parfum, in orders, invoices and catalogue names alike; two texts whose saveur_parfum differs are never the same product. — by muse-spark-profile

## Phases

| phase | invoices | lines | free (alias + rules) | AI | asked | wrong auto | Spark agreed with code | Spark alone right | AI cost |
|---|---|---|---|---|---|---|---|---|---|
| spark-checks-all | 21 | 389 | 283 (73 %) | 29 (7 %) | 77 (20 %) | 0 | 283/283 | 29/29 | $0.1578 |
| learning | 99 | 1804 | 1458 (81 %) | 119 (7 %) | 225 (12 %) | 2 | 385/385 | 119/120 | $0.4759 |
| test after 0 | 30 | 554 | 402 (73 %) | 42 (8 %) | 110 (20 %) | 0 | 402/402 | 42/42 | $0.2392 |
| test after 30 | 30 | 554 | 420 (76 %) | 50 (9 %) | 82 (15 %) | 2 | 63/63 | 50/52 | $0.1364 |
| test after 60 | 30 | 554 | 452 (82 %) | 28 (5 %) | 72 (13 %) | 2 | 115/115 | 28/28 | $0.1566 |
| test after 90 | 30 | 554 | 475 (86 %) | 27 (5 %) | 50 (9 %) | 2 | 137/137 | 27/27 | $0.1561 |
| test after 120 | 30 | 554 | 482 (87 %) | 25 (5 %) | 44 (8 %) | 3 | 120/120 | 25/26 | $0.1355 |

## Calibrations

- **calibration after 30 invoices** (415 seen + 139 unseen lines): 301/415 right with no AI (73 %), 0 wrong → 316/415 right with no AI (76 %), 0 wrong; accepted 4, rejected 32; search: BM25 (brand ×2) + 0.5·trigrams: right product in the top 8 for 98.8 % of lines (MRR 0.95) → word similarity: 99.3 % in the top 8 (MRR 0.94); engineer rules [R-0006]; $0.2039. Spark: unresolved is mostly Arabic generic words lacking French mapping plus French abbreviations singular/plural, fixing lexicon lets auto-match and search rank the right product
- **calibration after 60 invoices** (824 seen + 275 unseen lines): 618/824 right with no AI (75 %), 0 wrong → 627/824 right with no AI (76 %), 0 wrong; accepted 3, rejected 10; search: word similarity: right product in the top 8 for 99.6 % of lines (MRR 0.95) → BM25 + 0.5·trigrams + word similarity: 99.9 % in the top 8 (MRR 0.96); engineer rules []; $0.1555. Spark: unresolved is Arabic with no shared token and French abbreviations; mapping them to catalogue words fixes search misses and enables auto-match where unique
- **calibration after 90 invoices** (1238 seen + 413 unseen lines): 932/1238 right with no AI (75 %), 0 wrong → 936/1238 right with no AI (76 %), 0 wrong; accepted 1, rejected 8; search: BM25 + 0.5·trigrams + word similarity: right product in the top 8 for 99.8 % of lines (MRR 0.96) → BM25 (brand ×1.5) + trigrams: 99.8 % in the top 8 (MRR 0.96); engineer rules [R-0008]; $0.2262. Spark: unresolved is Arabic/French mismatch and shorthand yaou/frs plus monlait, rest is truly underspecified flavour/size/pack
- **calibration after 120 invoices** (1644 seen + 549 unseen lines): 1250/1644 right with no AI (76 %), 0 wrong → 1250/1644 right with no AI (76 %), 0 wrong; accepted 0, rejected 5; search: BM25 (brand ×1.5) + trigrams: right product in the top 8 for 99.8 % of lines (MRR 0.96) → BM25 (brand ×1.5) + trigrams: 99.8 % in the top 8 (MRR 0.96); engineer rules [R-0011]; $0.8672. Spark: failures are Arabic translations of catalogue French words plus supplier abbrevs yaou/frs and generic drink description words to drop

## Experiment log

- [bootstrap] block · accepted: split_glued (reads 2 catalogue names differently; products that read alike 13 → 13, by muse-spark) — invoice lines often glue brand/type to size/count with no space (yaourt1l, koolx4); splits 4+ letter words so codes stay whole
- [bootstrap] block · accepted: dimensions (reads 1 catalogue names differently; products that read alike 13 → 13, by muse-spark) — stationery cahiers write format 17X22 / 17 X 22; must stay one token to avoid confusion with packs
- [bootstrap] block · accepted: units_glue (reads 1 catalogue names differently; products that read alike 13 → 13, by muse-spark) — few non-food lines (LAMPE LED 12W, AMPOULE 7W) write watts with/without space; glues number+technical unit so 12W equals 12 W
- [bootstrap] block · accepted: pack_count (reads 0 catalogue names differently; products that read alike 13 → 13, by muse-spark) — pack count is decisive: LA VACHE QUI RIT 8P vs plain, PICON 16P, KOOL ROUND BREAK 6P, KOOL X4, GANT 10P; normalizes P/X forms to xN
- [bootstrap] lexicon · accepted: "choco" → "chocolat" (proposed before any invoice (reads 5 catalogue names differently; products that read alike 13 → 13): waits in shadow until known lines prove it, by muse-spark) — suppliers write choco for the catalogue's chocolat
- [bootstrap] lexicon · accepted: "conf" → "confiture" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 13 → 13): waits in shadow until known lines prove it, by muse-spark) — suppliers write conf for the catalogue's confiture
- [bootstrap] lexicon · accepted: "veg" → "vegetal" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 13 → 13): waits in shadow until known lines prove it, by muse-spark) — suppliers write veg for the catalogue's vegetal
- [bootstrap] lexicon · accepted: "prep" → "preparation" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 13 → 13): waits in shadow until known lines prove it, by muse-spark) — suppliers write prep for the catalogue's preparation
- [bootstrap] lexicon · accepted: "bte" → "boite" (proposed before any invoice (reads 0 catalogue names differently; products that read alike 13 → 13): waits in shadow until known lines prove it, by muse-spark) — suppliers write bte for the catalogue's boite
- [bootstrap] search · accepted: BM25 (brand ×2) + 0.5·trigrams (not measured yet: no line has a known answer, by muse-spark) — many brands share generic words (yaourt aromatise, biscuit sandwich, moutarde); strong brand_boost keeps shortlist in-brand, bm25 rewards rare variant words, trigram catches harrisa/hrissa and chaplure/chapelure typos without fuzzy prefix drift
- [bootstrap] attribute · accepted: "chocolat", "vanille", "fraise", "abricot", "pistache", "caramel", "thon", "ananas", "menthe", "citron" each name a saveur_parfum, in orders, invoices and catalogue names alike; two texts whose saveur_parfum differs are never the same product. (reads 51 catalogue names, 0 pairs made alike → shadow, by muse-spark) — one brand declines same base by flavour/filling; different stated flavours means different product
- [bootstrap] attribute · rejected: "sans", "light", "zero", "diet" each name a sans_light, in orders, invoices and catalogue names alike; a sans_light stated on one side only, or a different one, is never the same product. (reads 6 catalogue names, 0 pairs made alike → rejected, by muse-spark) — only the free-from variant states sans/light; absence does not contradict
- [after 21 invoices] autonomy · accepted: the desk's steps earned autonomy: Spark stops checking every answer (284 code answers confirmed, 284 right (95 % lower bound 99.06 % ≥ bar 99 %). While supervised: 389 lines, Spark agreed with the code on 283 (283 right), disagreed on 0; alone on 29, right on 29, by code) — 
- [calibration after 30 invoices] block · rejected: dimensions (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — keep cahier 17X22 as one token on both sides
- [calibration after 30 invoices] lexicon · rejected: "كراس" → "cahier" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « كراس EL HILLAL 32P » → EL HILLAL CAHIER 32 PAGES PIQUE PF 32P, by muse-spark) — arabic for cahier: RAYANE/EL HILLAL lines
- [calibration after 30 invoices] lexicon · rejected: "عجائن" → "pates" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — arabic for pates: SAFINA/SOSEMIE lines
- [calibration after 30 invoices] lexicon · rejected: "عجاين" → "pates" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « عجائن SEMOULERIE AMOUR 3.5KG » → SEMOULERIE AMOUR PACK PATES ALIMENTAIRES COUDE 3.5KG; « عجائن EL MAICENNE PATES » → EL MAICENNE PATES PATE ALIMENTAIRE, by muse-spark) — variant spelling of عجائن seen in misses
- [calibration after 30 invoices] lexicon · rejected: "شامبو" → "shampooing" (seen lines: +2 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « شامبو HACHFA » → HACHFA TEMPO SHAMPOOING; « شامبو HACHFA » → HACHFA TEMPO SHAMPOOING, by muse-spark) — HACHFA TEMPO SHAMPOOING
- [calibration after 30 invoices] lexicon · rejected: "حليب" → "lait" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « حليب RAMY MILKY KIDS » → RAMY MILKY KIDS BOISSON FRUITEE AU LAIT; « حليب COCOATY SPORT » → COCOATY SPORT CHOCOLAT AU LAIT MINI TABLETTE, by muse-spark) — milk: RAMY MILKY AU LAIT
- [calibration after 30 invoices] lexicon · rejected: "بسكويت" → "biscuit" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost, by muse-spark) — CASA GLACES/PALMARY BISCUIT
- [calibration after 30 invoices] lexicon · rejected: "هريسة" → "harissa" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — CB HARISSA 2KG
- [calibration after 30 invoices] lexicon · rejected: "هريسه" → "harissa" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « هريسة CB 2KG » → CB HARISSA 2KG; « هريسة CB 2KG » → CB HARISSA 2KG, by muse-spark) — ta-marbuta variant of harissa
- [calibration after 30 invoices] lexicon · rejected: "قفازات" → "gant" (seen lines: +0 newly resolved (0 distinct wordings), 1 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « قفازات ASRAR PARAPHAR » → ASRAR PARAPHAR GLOVE NOIR L GANT 100P instead of ASRAR PARAPHAR GANTS NOIR TAILLE L GLOVE, by muse-spark) — GANT DE MENAGE
- [calibration after 30 invoices] lexicon · rejected: "جبن" → "cheese" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — SPEEDO CHEESE PREPARATION FROMAGERE
- [calibration after 30 invoices] lexicon · rejected: "مثلجات" → "glacee" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « مثلجات PLAYTA » → PLAYTA SUCETTE GLACEE LUXE SURPRISE, by muse-spark) — PLAYTA SUCETTE GLACEE
- [calibration after 30 invoices] lexicon · rejected: "صلصة" → "sauce" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — FOODY'S SAUCE ALGERIENNE
- [calibration after 30 invoices] lexicon · rejected: "عصير" → "jus" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « عصير FRUIT MAX 2L » → FRUIT MAX JUS 2L, by muse-spark) — FRUIT MAX JUS 2L
- [calibration after 30 invoices] lexicon · rejected: "ياغورت" → "yaourt" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « ياغورت COQUILLAGE » → COQUILLAGE YAOURT AROMATISE, by muse-spark) — ACTIVIA YAOURT
- [calibration after 30 invoices] lexicon · rejected: "سكر" → "sucre" (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « سكر ILYIA » → ILYIA SUCRE SWEETY; « سكر ILYIA » → ILYIA SUCRE SWEETY, by muse-spark) — ILYIA SUCRE
- [calibration after 30 invoices] lexicon · rejected: "ماء" → "eau" (seen lines: +2 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « ماء WIPPRO » → WIPPRO LINGETTES NETTOYANTES ET DESINFECTANTE AVEC L'EAU JAVEL; « ماء WIPPRO » → WIPPRO LINGETTES NETTOYANTES ET DESINFECTANTE AVEC L'EAU JAVEL, by muse-spark) — WIPPRO AVEC L'EAU JAVEL
- [calibration after 30 invoices] lexicon · rejected: "ليمونادة" → "limonade" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — SPRITE LIMONADE
- [calibration after 30 invoices] lexicon · rejected: "منتوج" → "" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — drop generic sugary-drink description words
- [calibration after 30 invoices] lexicon · rejected: "مسكر" → "" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — drop generic sugary-drink description words
- [calibration after 30 invoices] lexicon · rejected: "جزئيا" → "" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — drop generic sugary-drink description words
- [calibration after 30 invoices] lexicon · accepted: "cr" → "creme" (seen lines: +3 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « BEN AROUS CR CHANTILLY » → BEN AROUS CREME CHANTILLY CRUNCHY PISTACHE; « CR DOUBLE SENS SPIDER WAX » → DOUBLE SENS SPIDER CREME WAX; « CR CHANTILLY CRUNCHY BEN AROUS » → BEN AROUS CREME CHANTILLY CRUNCHY PISTACHE, by muse-spark) — BEN AROUS CR CHANTILLY / SPIDER CREME WAX
- [calibration after 30 invoices] lexicon · rejected: "liq" → "liquide" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — KABTON MIEL LIQ -> LIQUIDE
- [calibration after 30 invoices] lexicon · accepted: "frmg" → "fromage" (seen lines: +4 newly resolved (3 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « FRMG FONDU SLICE MILKY COW » → MILKY COW FROMAGE FONDU SLICE; « FRMG SANDWICH FONDU 330GX48 » → SANDWICH FROMAGE FONDU 330G; « FRMG FONDU LAGRATANIA 900GX24 » → LAGRATANIA FROMAGE FONDU 900G, by muse-spark) — TIP TOP FRMG FONDU
- [calibration after 30 invoices] lexicon · rejected: "yaou" → "yaourt" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — YAOU SWEETLE / ACTIVIA YAOU
- [calibration after 30 invoices] lexicon · rejected: "frs" → "fraise" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « YAOU SWEETLE FRS » → SWEETLE YAOURT AROMATISE FRAISE, by muse-spark) — SWEETLE FRAISE
- [calibration after 30 invoices] lexicon · accepted: "pate" → "pates" (seen lines: +7 newly resolved (4 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « PATE LONGUES SOSEMIE » → SOSEMIE PATES LONGUES; « SEMOULERIE AMOUR PACK PATE 3.5KG » → SEMOULERIE AMOUR PACK PATES ALIMENTAIRES COUDE 3.5KG; « SEMOULERIE AMOUR PATE ALIMENTAIRES PENNE » → SEMOULERIE AMOUR PATES ALIMENTAIRES PENNE, by muse-spark) — singular/plural: PATE LONGUES -> PATES LONGUES
- [calibration after 30 invoices] lexicon · accepted: "choco" → "chocolat" (from shadow) (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; e.g. « STORY BIFA CHOCO OFFRE » → BIFA STORY CHOCOLAT OFFRE SPECIALE, by muse-spark) — suppliers write choco for the catalogue's chocolat
- [calibration after 30 invoices] lexicon · rejected: "conf" → "confiture" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write conf for the catalogue's confiture
- [calibration after 30 invoices] lexicon · rejected: "veg" → "vegetal" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write veg for the catalogue's vegetal
- [calibration after 30 invoices] lexicon · rejected: "prep" → "preparation" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write prep for the catalogue's preparation
- [calibration after 30 invoices] lexicon · rejected: "bte" → "boite" (from shadow) (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — suppliers write bte for the catalogue's boite
- [calibration after 30 invoices] lexicon · rejected: "عجاين" → "pates" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « عجائن SEMOULERIE AMOUR 3.5KG » → SEMOULERIE AMOUR PACK PATES ALIMENTAIRES COUDE 3.5KG; « عجائن EL MAICENNE PATES » → EL MAICENNE PATES PATE ALIMENTAIRE, by muse-spark) — variant spelling of عجائن seen in misses
- [calibration after 30 invoices] lexicon · rejected: "حليب" → "lait" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « حليب RAMY MILKY KIDS » → RAMY MILKY KIDS BOISSON FRUITEE AU LAIT; « حليب COCOATY SPORT » → COCOATY SPORT CHOCOLAT AU LAIT MINI TABLETTE, by muse-spark) — milk: RAMY MILKY AU LAIT
- [calibration after 30 invoices] lexicon · rejected: "هريسه" → "harissa" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « هريسة CB 2KG » → CB HARISSA 2KG; « هريسة CB 2KG » → CB HARISSA 2KG, by muse-spark) — ta-marbuta variant of harissa
- [calibration after 30 invoices] lexicon · rejected: "سكر" → "sucre" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « سكر ILYIA » → ILYIA SUCRE SWEETY; « سكر ILYIA » → ILYIA SUCRE SWEETY, by muse-spark) — ILYIA SUCRE
- [calibration after 30 invoices] search · accepted: classic word similarity [word similarity] (classic word similarity [word similarity]: 99.3 % in top 8, MRR 0.94; BM25 [BM25]: 99.0 % in top 8, MRR 0.95; BM25, brand ×2 [BM25 (brand ×2)]: 98.8 % in top 8, MRR 0.95; trigrams [trigrams]: 98.8 % in top 8, MRR 0.94; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 99.0 % in top 8, MRR 0.95; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 99.0 % in top 8, MRR 0.95; Spark: bm25-brand-trigram [BM25 (brand ×2) + 0.5·trigrams]: 98.8 % in top 8, MRR 0.95; Spark: pack-exact-wordsim [BM25 (brand ×2) + 0.7·trigrams + 0.6·word similarity]: 98.8 % in top 8, MRR 0.95; Spark: high-trigram [BM25 (brand ×3) + trigrams + 0.3·word similarity]: 98.8 % in top 8, MRR 0.95; current: Spark's first guess [BM25 (brand ×2) + 0.5·trigrams]: 98.8 % in top 8, MRR 0.95. Chosen: 99.3 % in top 8 on seen lines; on unseen lines 100.0 % vs 100.0 % for the current method, by library) — 
- [calibration after 30 invoices] rekey · accepted: re-read every known wording (31 alias(es) moved to the new reading, no collision, by code) — 
- [calibration after 60 invoices] block · rejected: dimensions (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — keeps cahier 17X22 as one token
- [calibration after 60 invoices] block · rejected: inch off (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — no inch plumbing in FMCG
- [calibration after 60 invoices] block · rejected: strength off (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — no pharma dosage
- [calibration after 60 invoices] block · rejected: memory off (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — no phones
- [calibration after 60 invoices] block · rejected: shade off (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — no cosmetics shade
- [calibration after 60 invoices] lexicon · rejected: "صلصه" → "sauce" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — ta-marbuta variant
- [calibration after 60 invoices] lexicon · rejected: "خردل" → "moutarde" (seen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « خردل DAILY SAUCE » → DAILY SAUCE MOUTARDE, by muse-spark) — خردل MOUTARDE -> MOUTARDE GM
- [calibration after 60 invoices] lexicon · rejected: "ليموناده" → "limonade" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — variant spelling
- [calibration after 60 invoices] lexicon · rejected: "محلى" → "" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — drop filler
- [calibration after 60 invoices] lexicon · accepted: "عجاين" → "pates" (from shadow) (seen lines: +2 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « عجائن SEMOULERIE AMOUR 3.5KG » → SEMOULERIE AMOUR PACK PATES ALIMENTAIRES COUDE 3.5KG; « عجائن EL MAICENNE PATES » → EL MAICENNE PATES PATE ALIMENTAIRE, by muse-spark) — variant spelling of عجائن seen in misses
- [calibration after 60 invoices] lexicon · accepted: "حليب" → "lait" (from shadow) (seen lines: +3 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « حليب RAMY MILKY KIDS » → RAMY MILKY KIDS BOISSON FRUITEE AU LAIT; « حليب COCOATY SPORT » → COCOATY SPORT CHOCOLAT AU LAIT MINI TABLETTE; « حليب RAMY MILKY KIDS » → RAMY MILKY KIDS BOISSON FRUITEE AU LAIT, by muse-spark) — milk: RAMY MILKY AU LAIT
- [calibration after 60 invoices] lexicon · accepted: "هريسه" → "harissa" (from shadow) (seen lines: +4 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « هريسة CB 2KG » → CB HARISSA 2KG; « هريسة CB 2KG » → CB HARISSA 2KG; « هريسة CB 2KG » → CB HARISSA 2KG, by muse-spark) — ta-marbuta variant of harissa
- [calibration after 60 invoices] lexicon · rejected: "سكر" → "sucre" (from shadow) (seen lines: +4 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; no gain yet on unseen lines: waits in shadow; e.g. « سكر ILYIA » → ILYIA SUCRE SWEETY; « سكر ILYIA » → ILYIA SUCRE SWEETY; « سكر ILYIA » → ILYIA SUCRE SWEETY, by muse-spark) — ILYIA SUCRE
- [calibration after 60 invoices] search · accepted: Spark: bag wordsim trigram [BM25 + 0.5·trigrams + word similarity] (classic word similarity [word similarity]: 99.6 % in top 8, MRR 0.95; BM25 [BM25]: 99.9 % in top 8, MRR 0.96; BM25, brand ×2 [BM25 (brand ×2)]: 99.8 % in top 8, MRR 0.96; trigrams [trigrams]: 99.6 % in top 8, MRR 0.95; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 99.9 % in top 8, MRR 0.96; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 99.9 % in top 8, MRR 0.96; Spark: bag wordsim [BM25 + word similarity]: 99.9 % in top 8, MRR 0.96; Spark: bag wordsim trigram [BM25 + 0.5·trigrams + word similarity]: 99.9 % in top 8, MRR 0.96; Spark: bm25 brand [BM25 (brand ×2) + 0.3·trigrams + 0.7·word similarity]: 99.8 % in top 8, MRR 0.96; current: classic word similarity [word similarity]: 99.6 % in top 8, MRR 0.95. Chosen: 99.9 % in top 8 on seen lines; on unseen lines 99.6 % vs 98.9 % for the current method, by muse-spark) — adds fuzzy for monlait/lait glace/glacee and typos
- [calibration after 60 invoices] rekey · accepted: re-read every known wording (14 alias(es) moved to the new reading, no collision, by code) — 
- [calibration after 90 invoices] block · rejected: dimensions (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — cahier format 17X22 must stay one token
- [calibration after 90 invoices] block · rejected: inch off (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — no plumbing inches in FMCG
- [calibration after 90 invoices] block · rejected: strength off (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — no mg dosages in FMCG
- [calibration after 90 invoices] block · rejected: memory off (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — no phone memory in FMCG
- [calibration after 90 invoices] block · rejected: shade off (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — no cosmetic shades in FMCG
- [calibration after 90 invoices] lexicon · rejected: "قهوة" → "cafe" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — HOUSE/FAMICO cafe
- [calibration after 90 invoices] lexicon · rejected: "monlait" → "lait" (seen lines: +3 newly resolved (1 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; e.g. « حليب CHEDDAR » → CHEDDAR MONLAIT; « حليب CHEDDAR » → CHEDDAR MONLAIT; « حليب CHEDDAR » → CHEDDAR MONLAIT, by muse-spark) — CHEDDAR MONLAIT is lait, lets حليب match
- [calibration after 90 invoices] lexicon · rejected: "ومحلى" → "" (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — drop generic description word
- [calibration after 90 invoices] lexicon · accepted: "سكر" → "sucre" (from shadow) (seen lines: +4 newly resolved (2 distinct wordings), 0 wrong, 0 lost; unseen lines: +1 newly resolved (1 distinct wordings), 0 wrong, 0 lost; e.g. « سكر ILYIA » → ILYIA SUCRE SWEETY; « سكر ILYIA » → ILYIA SUCRE SWEETY; « سكر ILYIA » → ILYIA SUCRE SWEETY, by muse-spark) — ILYIA SUCRE
- [calibration after 90 invoices] search · accepted: BM25 + trigrams [BM25 (brand ×1.5) + trigrams] (classic word similarity [word similarity]: 99.5 % in top 8, MRR 0.95; BM25 [BM25]: 99.8 % in top 8, MRR 0.96; BM25, brand ×2 [BM25 (brand ×2)]: 99.6 % in top 8, MRR 0.95; trigrams [trigrams]: 99.5 % in top 8, MRR 0.95; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 99.8 % in top 8, MRR 0.96; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 99.8 % in top 8, MRR 0.96; Spark: keep BM25+trigram+wordsim [BM25 (brand ×2) + 0.5·trigrams + word similarity]: 99.7 % in top 8, MRR 0.96; current: Spark: bag wordsim trigram [BM25 + 0.5·trigrams + word similarity]: 99.8 % in top 8, MRR 0.96. Chosen: 99.8 % in top 8 on seen lines; on unseen lines 99.8 % vs 99.8 % for the current method, by library) — 
- [calibration after 90 invoices] rekey · accepted: re-read every known wording (2 alias(es) moved to the new reading, no collision, by code) — 
- [calibration after 120 invoices] block · rejected: dimensions (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — cahier format 17X22 must stay one token
- [calibration after 120 invoices] block · rejected: inch off (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — no plumbing inch sizes in FMCG
- [calibration after 120 invoices] block · rejected: strength off (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — no pharma dosages
- [calibration after 120 invoices] block · rejected: memory off (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — no phones
- [calibration after 120 invoices] block · rejected: shade off (seen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost; unseen lines: +0 newly resolved (0 distinct wordings), 0 wrong, 0 lost, by muse-spark) — no cosmetics shades
- [calibration after 120 invoices] search · accepted: BM25 + trigrams [BM25 (brand ×1.5) + trigrams] (classic word similarity [word similarity]: 99.5 % in top 8, MRR 0.95; BM25 [BM25]: 99.8 % in top 8, MRR 0.96; BM25, brand ×2 [BM25 (brand ×2)]: 99.6 % in top 8, MRR 0.96; trigrams [trigrams]: 99.5 % in top 8, MRR 0.96; BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 99.8 % in top 8, MRR 0.96; BM25 + trigrams + word similarity [BM25 (brand ×1.5) + 0.7·trigrams + 0.5·word similarity]: 99.8 % in top 8, MRR 0.96; Spark: bm25-brand-trigram [BM25 (brand ×1.5) + 0.5·trigrams]: 99.8 % in top 8, MRR 0.96; Spark: bm25-brand-wordsim [BM25 (brand ×2) + 0.3·trigrams + 0.5·word similarity]: 99.6 % in top 8, MRR 0.96; Spark: bm25-brand-strong [BM25 (brand ×3) + 0.5·trigrams]: 99.6 % in top 8, MRR 0.96; current: BM25 + trigrams [BM25 (brand ×1.5) + trigrams]: 99.8 % in top 8, MRR 0.96. Chosen: 99.8 % in top 8 on seen lines; on unseen lines 99.8 % vs 99.8 % for the current method, by library) — 

Meta model spend for the whole run: $2.9343

## Who made the free answers possible (test invoices)

2237 free answers: Muse Spark found 172 (8 %), the matching Muse Spark designed gave 1932 (86 %), a person taught 133 (6 %).

- catalogue match through the matching Muse Spark designed: 1320
- alias first matched through the matching Muse Spark designed: 595
- alias first answered by a person: 110
- rule Muse Spark wrote: 106
- alias first found by Muse Spark: 63
- code first answered by a person: 23
- code first matched through the matching Muse Spark designed: 17
- code first found by Muse Spark: 3
