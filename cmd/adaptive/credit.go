package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

// Credit says who made each free answer on the test invoices possible. A free answer never calls a
// model, but something taught the desk to give it:
//   - SparkFound: a wording or supplier code the desk remembers because Muse Spark picked that product the
//     first time the wording was seen (and a person confirmed it), or a rule Muse Spark wrote;
//   - SparkDesigned: a direct catalogue match through the normaliser, vocabulary and search that Muse Spark
//     designed for this industry (and the Lab proved), or a wording first matched that way;
//   - Person: a wording a person answered the first time.
type Credit struct {
	Free          int            `json:"free"`
	SparkFound    int            `json:"spark_found"`
	SparkDesigned int            `json:"spark_designed"`
	Person        int            `json:"person"`
	Detail        map[string]int `json:"detail"`
}

func Attribute(db *store.DB) Credit {
	c := Credit{Detail: map[string]int{}}
	authors := map[string]string{}
	if rs, err := db.Query(`SELECT id, author FROM rules`); err == nil {
		for rs.Next() {
			var id, a string
			rs.Scan(&id, &a)
			authors[id] = a
		}
		rs.Close()
	}
	// how the desk found each wording (and each product for each supplier) the first time
	firstNorm, firstProd := map[[2]string]string{}, map[[2]string]string{}
	if rs, err := db.Query(`SELECT norm, product_id, supplier_id, level FROM ground_truth ORDER BY created_at`); err == nil {
		for rs.Next() {
			var norm, pid, sup, lvl string
			rs.Scan(&norm, &pid, &sup, &lvl)
			if _, ok := firstNorm[[2]string{norm, pid}]; !ok {
				firstNorm[[2]string{norm, pid}] = lvl
			}
			if _, ok := firstProd[[2]string{pid, sup}]; !ok {
				firstProd[[2]string{pid, sup}] = lvl
			}
		}
		rs.Close()
	}
	rs, err := db.Query(`SELECT l.level, l.used, l.norm, l.product_id, d.supplier_id FROM lines l JOIN drafts d ON d.id = l.draft_id
		WHERE d.status = 'cancelled' AND l.status = 'ok' AND l.level IN ('alias', 'code', 'rules')`)
	if err != nil {
		return c
	}
	defer rs.Close()
	for rs.Next() {
		var lvl, used, norm, pid, sup string
		rs.Scan(&lvl, &used, &norm, &pid, &sup)
		c.Free++
		var ids []string
		json.Unmarshal([]byte(used), &ids)
		spark, ruled := false, false
		for _, id := range ids {
			if strings.HasPrefix(id, "R-") {
				ruled = true
				a := authors[id]
				spark = spark || strings.HasPrefix(a, "rule-engineer") || strings.HasPrefix(a, "muse-spark")
			}
		}
		switch {
		case lvl == "rules" && ruled && spark:
			c.SparkFound++
			c.Detail["rule Muse Spark wrote"]++
		case lvl == "rules" && ruled:
			c.Person++
			c.Detail["rule a person wrote"]++
		case lvl == "rules":
			c.SparkDesigned++
			c.Detail["catalogue match through the matching Muse Spark designed"]++
		default:
			first, ok := firstNorm[[2]string{norm, pid}]
			if !ok {
				first = firstProd[[2]string{pid, sup}]
			}
			switch first {
			case "ai":
				c.SparkFound++
				c.Detail[lvl+" first found by Muse Spark"]++
			case "person":
				c.Person++
				c.Detail[lvl+" first answered by a person"]++
			default:
				c.SparkDesigned++
				c.Detail[lvl+" first matched through the matching Muse Spark designed"]++
			}
		}
	}
	return c
}

func (c Credit) Markdown() string {
	if c.Free == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n## Who made the free answers possible (test invoices)\n\n%d free answers: Muse Spark found %d (%s), the matching Muse Spark designed gave %d (%s), a person taught %d (%s).\n\n",
		c.Free, c.SparkFound, pct(c.SparkFound, c.Free), c.SparkDesigned, pct(c.SparkDesigned, c.Free), c.Person, pct(c.Person, c.Free))
	keys := make([]string, 0, len(c.Detail))
	for k := range c.Detail {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return c.Detail[keys[i]] > c.Detail[keys[j]] })
	for _, k := range keys {
		fmt.Fprintf(&b, "- %s: %d\n", k, c.Detail[k])
	}
	return b.String()
}
