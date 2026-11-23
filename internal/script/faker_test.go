package script

import (
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/fake"
)

// Every built-in has a faker.* method declared in lazyapi.d.ts and working
// in the runtime.
func TestFakerMatchesTypes(t *testing.T) {
	block := Types[strings.Index(Types, "declare const faker"):]
	block = block[:strings.Index(block, "};")]
	var calls []string
	for _, v := range fake.Vars() {
		js := fake.JSName(v.Name)
		if js == "int" { // faker.int(min, max) returns a number
			continue
		}
		if !strings.Contains(block, " "+js+"(): string;") {
			t.Errorf("lazyapi.d.ts does not declare faker.%s()", js)
		}
		calls = append(calls, `expect(typeof faker.`+js+`()).toBe("string");`)
	}
	res := Run(`test("all", () => {`+strings.Join(calls, "\n")+`});`, baseData(), Options{})
	mustPass(t, res)
}
