// readas prints how a desk reads lines with the Matching Profile Muse Spark designed for it.
//
//	readas -data ./v3-hardware/desk "VIS TH M8X20 ZN BT100" "ROBINET 15/21"
package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/nudiiir/elixir-orderdesk/internal/catalog"
	"github.com/nudiiir/elixir-orderdesk/internal/profile"
	"github.com/nudiiir/elixir-orderdesk/internal/rules"
	"github.com/nudiiir/elixir-orderdesk/internal/store"
)

func main() {
	data := flag.String("data", "data", "desk data directory")
	flag.Parse()
	db, err := store.Open(filepath.Join(*data, "orderdesk.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	cat := catalog.New(db)
	if p := profile.Load(db); p != nil {
		p.Install(cat)
	}
	live := rules.NewSet(rules.WithStatus(db, "live"))
	for _, line := range flag.Args() {
		out := live.Apply(cat.StripCodes(catalog.Normalize(line)), "", "")
		fmt.Printf("%s\t%s\n", line, strings.TrimSpace(out.Text))
	}
}
