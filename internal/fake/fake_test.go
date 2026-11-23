package fake

import (
	"regexp"
	"testing"
)

func TestValues(t *testing.T) {
	for _, v := range Vars() {
		got, ok := Value(v.Name)
		if !ok || got == "" {
			t.Errorf("%s = %q", v.Name, got)
		}
	}
	if _, ok := Value("nope"); ok {
		t.Error("unknown name must fail")
	}
	uuid := regexp.MustCompile(`^[0-9a-f-]{36}$`)
	if g, _ := Value("guid"); !uuid.MatchString(g) {
		t.Errorf("guid = %q", g)
	}
}

func TestSeed(t *testing.T) {
	Seed(42)
	a, _ := Value("randomEmail")
	Seed(42)
	b, _ := Value("randomEmail")
	Seed(0)
	if a != b {
		t.Errorf("seeded values differ: %q %q", a, b)
	}
}

func TestJSName(t *testing.T) {
	for in, want := range map[string]string{
		"randomFirstName": "firstName", "randomUUID": "uuid", "randomIPV6": "ipv6",
		"randomMACAddress": "macAddress", "guid": "guid", "isoTimestamp": "isoTimestamp", "randomBs": "bs",
	} {
		if got := JSName(in); got != want {
			t.Errorf("JSName(%s) = %s, want %s", in, got, want)
		}
	}
}
