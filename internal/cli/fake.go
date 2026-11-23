package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/githiago-f/lazyapi/internal/fake"
)

// ListFake prints the built-in dynamic variables ({{$name}}) with an
// example value, or generates the value of one: lazyapi fake randomEmail.
func ListFake(args []string) error {
	fs := newFlagSet("fake")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return usageErr("%v\nusage: lazyapi fake [name] [--json]", err)
	}
	if len(pos) > 0 {
		name := pos[0]
		if len(name) > 0 && name[0] == '$' {
			name = name[1:]
		}
		v, ok := fake.Value(name)
		if !ok {
			return usageErr("unknown dynamic variable %q (lazyapi fake lists them)", pos[0])
		}
		fmt.Println(v)
		return nil
	}
	type row struct {
		Name        string `json:"name"`
		Template    string `json:"template"`
		Script      string `json:"script"`
		Description string `json:"description"`
		Example     string `json:"example"`
	}
	var rows []row
	for _, v := range fake.Vars() {
		ex, _ := fake.Value(v.Name)
		rows = append(rows, row{v.Name, "{{$" + v.Name + "}}", "faker." + fake.JSName(v.Name) + "()", v.Description, ex})
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TEMPLATE\tSCRIPT\tDESCRIPTION\tEXAMPLE")
	for _, r := range rows {
		ex := r.Example
		if len([]rune(ex)) > 40 {
			ex = string([]rune(ex)[:39]) + "…"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Template, r.Script, r.Description, ex)
	}
	fmt.Fprintln(w, "\nA test script can add its own or replace these: export function randomSku() { return \"SKU-\" + faker.int(1, 999); } → {{$randomSku}}")
	return w.Flush()
}
