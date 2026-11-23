// Package fake generates the values of dynamic template variables such as
// {{$randomEmail}} and {{$guid}} (the names Postman uses), backed by
// gofakeit. Scripts reach the same generators through the `faker` global.
package fake

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

// Var is one built-in dynamic variable.
type Var struct {
	Name        string // without "$", e.g. "randomEmail"
	Description string
	gen         func(f *gofakeit.Faker) string
}

var (
	mu    sync.Mutex
	faker = gofakeit.New(0)
)

// Seed makes the generated values reproducible (0: random again).
func Seed(seed uint64) {
	mu.Lock()
	defer mu.Unlock()
	faker = gofakeit.New(seed)
}

// Value generates a new value for the built-in variable name (without "$").
func Value(name string) (string, bool) {
	v, ok := byName[name]
	if !ok {
		return "", false
	}
	mu.Lock()
	defer mu.Unlock()
	return v.gen(faker), true
}

// Int returns a random integer in [min, max].
func Int(min, max int) int {
	if max < min {
		min, max = max, min
	}
	mu.Lock()
	defer mu.Unlock()
	return faker.IntRange(min, max)
}

// Float returns a random number in [min, max).
func Float(min, max float64) float64 {
	if max < min {
		min, max = max, min
	}
	mu.Lock()
	defer mu.Unlock()
	return faker.Float64Range(min, max)
}

// Vars lists the built-in variables sorted by name.
func Vars() []Var {
	out := make([]Var, len(vars))
	copy(out, vars)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// JSName is the faker.* method for a variable: "randomFirstName" →
// "firstName", "randomUUID" → "uuid", "randomIPV6" → "ipv6", "guid" → "guid".
func JSName(name string) string {
	s := strings.TrimPrefix(name, "random")
	if s == "" {
		return name
	}
	r := []rune(s)
	n := 0
	for n < len(r) && r[n] >= 'A' && r[n] <= 'Z' {
		n++
	}
	if n > 1 && n < len(r) && r[n] >= 'a' && r[n] <= 'z' {
		n-- // "MACAddress": keep the A of "Address"
	}
	if n == 0 {
		n = 1
	}
	return strings.ToLower(string(r[:n])) + string(r[n:])
}

func s(fn func(f *gofakeit.Faker) string) func(f *gofakeit.Faker) string { return fn }

func words(f *gofakeit.Faker, n int) string {
	w := make([]string, n)
	for i := range w {
		w[i] = f.LoremIpsumWord()
	}
	return strings.Join(w, " ")
}

func sentences(f *gofakeit.Faker, n int) string {
	out := make([]string, n)
	for i := range out {
		out[i] = f.LoremIpsumSentence(f.IntRange(5, 12))
	}
	return strings.Join(out, " ")
}

func date(t time.Time) string { return t.UTC().Format(time.RFC3339) }

var vars = []Var{
	// Common
	{"guid", "a v4 UUID", s(func(f *gofakeit.Faker) string { return f.UUID() })},
	{"timestamp", "current Unix timestamp (seconds)", s(func(*gofakeit.Faker) string { return strconv.FormatInt(time.Now().Unix(), 10) })},
	{"isoTimestamp", "current time, ISO 8601 (UTC)", s(func(*gofakeit.Faker) string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") })},
	{"randomUUID", "a v4 UUID", s(func(f *gofakeit.Faker) string { return f.UUID() })},
	// Text, numbers and colors
	{"randomAlphaNumeric", "one letter or digit", s(func(f *gofakeit.Faker) string {
		const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
		i := f.IntRange(0, len(chars)-1)
		return chars[i : i+1]
	})},
	{"randomBoolean", "true or false", s(func(f *gofakeit.Faker) string { return strconv.FormatBool(f.Bool()) })},
	{"randomInt", "integer between 0 and 1000", s(func(f *gofakeit.Faker) string { return strconv.Itoa(f.IntRange(0, 1000)) })},
	{"randomColor", "color name", s(func(f *gofakeit.Faker) string { return strings.ToLower(f.SafeColor()) })},
	{"randomHexColor", "#rrggbb", s(func(f *gofakeit.Faker) string { return f.HexColor() })},
	{"randomAbbreviation", "abbreviation (e.g. SQL)", s(func(f *gofakeit.Faker) string { return f.HackerAbbreviation() })},
	// Internet
	{"randomIP", "IPv4 address", s(func(f *gofakeit.Faker) string { return f.IPv4Address() })},
	{"randomIPV6", "IPv6 address", s(func(f *gofakeit.Faker) string { return f.IPv6Address() })},
	{"randomMACAddress", "MAC address", s(func(f *gofakeit.Faker) string { return f.MacAddress() })},
	{"randomPassword", "15-character password", s(func(f *gofakeit.Faker) string { return f.Password(true, true, true, false, false, 15) })},
	{"randomLocale", "two-letter language code", s(func(f *gofakeit.Faker) string { return f.LanguageAbbreviation() })},
	{"randomUserAgent", "browser user agent", s(func(f *gofakeit.Faker) string { return f.UserAgent() })},
	{"randomProtocol", "http or https", s(func(f *gofakeit.Faker) string { return f.RandomString([]string{"http", "https"}) })},
	{"randomSemver", "semantic version", s(func(f *gofakeit.Faker) string { return f.AppVersion() })},
	{"randomEmail", "email address", s(func(f *gofakeit.Faker) string { return f.Email() })},
	{"randomExampleEmail", "email at example.com", s(func(f *gofakeit.Faker) string {
		return strings.ToLower(f.Username()) + "@example." + f.RandomString([]string{"com", "net", "org"})
	})},
	{"randomUserName", "username", s(func(f *gofakeit.Faker) string { return f.Username() })},
	{"randomUrl", "URL", s(func(f *gofakeit.Faker) string { return f.URL() })},
	{"randomDomainName", "domain name", s(func(f *gofakeit.Faker) string { return f.DomainName() })},
	{"randomDomainSuffix", "domain suffix (com, net…)", s(func(f *gofakeit.Faker) string { return f.DomainSuffix() })},
	{"randomDomainWord", "word for a domain", s(func(f *gofakeit.Faker) string { return strings.ToLower(f.Word()) })},
	// Names and jobs
	{"randomFirstName", "first name", s(func(f *gofakeit.Faker) string { return f.FirstName() })},
	{"randomLastName", "last name", s(func(f *gofakeit.Faker) string { return f.LastName() })},
	{"randomFullName", "first and last name", s(func(f *gofakeit.Faker) string { return f.Name() })},
	{"randomNamePrefix", "Mr., Ms., Dr.…", s(func(f *gofakeit.Faker) string { return f.NamePrefix() })},
	{"randomNameSuffix", "Jr., Sr., PhD…", s(func(f *gofakeit.Faker) string { return f.NameSuffix() })},
	{"randomJobArea", "job area", s(func(f *gofakeit.Faker) string { return f.JobLevel() })},
	{"randomJobDescriptor", "job descriptor", s(func(f *gofakeit.Faker) string { return f.JobDescriptor() })},
	{"randomJobTitle", "job title", s(func(f *gofakeit.Faker) string { return f.JobTitle() })},
	{"randomJobType", "job type", s(func(f *gofakeit.Faker) string {
		return f.RandomString([]string{"Engineer", "Manager", "Designer", "Analyst", "Consultant"})
	})},
	{"randomGender", "gender", s(func(f *gofakeit.Faker) string { return f.Gender() })},
	{"randomAge", "age between 18 and 80", s(func(f *gofakeit.Faker) string { return strconv.Itoa(f.IntRange(18, 80)) })},
	// Phone and address
	{"randomPhoneNumber", "phone number", s(func(f *gofakeit.Faker) string { return f.PhoneFormatted() })},
	{"randomPhoneNumberExt", "phone number with extension", s(func(f *gofakeit.Faker) string { return f.Numerify("##-###-###-####") })},
	{"randomCity", "city", s(func(f *gofakeit.Faker) string { return f.City() })},
	{"randomStreetName", "street name", s(func(f *gofakeit.Faker) string { return f.StreetName() })},
	{"randomStreetAddress", "number and street", s(func(f *gofakeit.Faker) string { return f.Street() })},
	{"randomState", "state", s(func(f *gofakeit.Faker) string { return f.State() })},
	{"randomZipCode", "zip code", s(func(f *gofakeit.Faker) string { return f.Zip() })},
	{"randomCountry", "country", s(func(f *gofakeit.Faker) string { return f.Country() })},
	{"randomCountryCode", "two-letter country code", s(func(f *gofakeit.Faker) string { return f.CountryAbr() })},
	{"randomLatitude", "latitude", s(func(f *gofakeit.Faker) string { return strconv.FormatFloat(f.Latitude(), 'f', 4, 64) })},
	{"randomLongitude", "longitude", s(func(f *gofakeit.Faker) string { return strconv.FormatFloat(f.Longitude(), 'f', 4, 64) })},
	{"randomTimeZone", "IANA time zone", s(func(f *gofakeit.Faker) string { return f.TimeZoneRegion() })},
	// Images
	{"randomAvatarImage", "avatar image URL", s(func(f *gofakeit.Faker) string {
		return fmt.Sprintf("https://i.pravatar.cc/150?u=%s", f.UUID())
	})},
	{"randomImageUrl", "image URL (640x480)", s(func(f *gofakeit.Faker) string {
		return fmt.Sprintf("https://picsum.photos/seed/%d/640/480", f.IntRange(1, 100000))
	})},
	// Business and finance
	{"randomCompanyName", "company name", s(func(f *gofakeit.Faker) string { return f.Company() })},
	{"randomCompanySuffix", "Inc, LLC…", s(func(f *gofakeit.Faker) string { return f.CompanySuffix() })},
	{"randomBs", "business phrase", s(func(f *gofakeit.Faker) string { return f.BS() })},
	{"randomCatchPhrase", "slogan", s(func(f *gofakeit.Faker) string { return f.Slogan() })},
	{"randomPrice", "price between 1 and 1000", s(func(f *gofakeit.Faker) string { return strconv.FormatFloat(f.Price(1, 1000), 'f', 2, 64) })},
	{"randomCurrencyCode", "ISO 4217 code", s(func(f *gofakeit.Faker) string { return f.CurrencyShort() })},
	{"randomCurrencyName", "currency name", s(func(f *gofakeit.Faker) string { return f.CurrencyLong() })},
	{"randomBitcoin", "bitcoin address", s(func(f *gofakeit.Faker) string { return f.BitcoinAddress() })},
	{"randomBankAccount", "8-digit account number", s(func(f *gofakeit.Faker) string { return f.Numerify("########") })},
	{"randomBankAccountName", "account type", s(func(f *gofakeit.Faker) string { return f.BankType() })},
	{"randomCreditCardMask", "last 4 digits", s(func(f *gofakeit.Faker) string { return f.Numerify("####") })},
	{"randomCreditCardNumber", "test card number", s(func(f *gofakeit.Faker) string { return f.CreditCardNumber(nil) })},
	{"randomTransactionType", "transaction type", s(func(f *gofakeit.Faker) string {
		return f.RandomString([]string{"deposit", "withdrawal", "payment", "invoice"})
	})},
	// Dates
	{"randomDateFuture", "a future date (RFC 3339)", s(func(f *gofakeit.Faker) string { return date(f.FutureDate()) })},
	{"randomDatePast", "a past date (RFC 3339)", s(func(f *gofakeit.Faker) string { return date(f.PastDate()) })},
	{"randomDateRecent", "a date in the last days (RFC 3339)", s(func(f *gofakeit.Faker) string {
		return date(time.Now().Add(-time.Duration(f.IntRange(1, 72*3600)) * time.Second))
	})},
	{"randomWeekday", "day of the week", s(func(f *gofakeit.Faker) string { return f.WeekDay() })},
	{"randomMonth", "month name", s(func(f *gofakeit.Faker) string { return f.MonthString() })},
	// Files
	{"randomFileName", "file name", s(func(f *gofakeit.Faker) string { return strings.ToLower(f.Word()) + "." + f.FileExtension() })},
	{"randomFileExt", "file extension", s(func(f *gofakeit.Faker) string { return f.FileExtension() })},
	{"randomMimeType", "MIME type", s(func(f *gofakeit.Faker) string { return f.FileMimeType() })},
	// Commerce
	{"randomProductName", "product name", s(func(f *gofakeit.Faker) string { return f.ProductName() })},
	{"randomProduct", "product category", s(func(f *gofakeit.Faker) string { return f.ProductCategory() })},
	{"randomProductMaterial", "material", s(func(f *gofakeit.Faker) string { return f.ProductMaterial() })},
	{"randomProductAdjective", "adjective", s(func(f *gofakeit.Faker) string { return f.AdjectiveDescriptive() })},
	{"randomDepartment", "department", s(func(f *gofakeit.Faker) string { return f.ProductCategory() })},
	// Words and lorem ipsum
	{"randomNoun", "noun", s(func(f *gofakeit.Faker) string { return f.Noun() })},
	{"randomVerb", "verb", s(func(f *gofakeit.Faker) string { return f.Verb() })},
	{"randomIngverb", "verb ending in -ing", s(func(f *gofakeit.Faker) string { return f.HackeringVerb() })},
	{"randomAdjective", "adjective", s(func(f *gofakeit.Faker) string { return f.Adjective() })},
	{"randomWord", "word", s(func(f *gofakeit.Faker) string { return f.Word() })},
	{"randomWords", "a few words", s(func(f *gofakeit.Faker) string {
		w := make([]string, f.IntRange(2, 5))
		for i := range w {
			w[i] = f.Word()
		}
		return strings.Join(w, " ")
	})},
	{"randomPhrase", "phrase", s(func(f *gofakeit.Faker) string { return f.HackerPhrase() })},
	{"randomLoremWord", "lorem ipsum word", s(func(f *gofakeit.Faker) string { return f.LoremIpsumWord() })},
	{"randomLoremWords", "three lorem ipsum words", s(func(f *gofakeit.Faker) string { return words(f, 3) })},
	{"randomLoremSentence", "lorem ipsum sentence", s(func(f *gofakeit.Faker) string { return sentences(f, 1) })},
	{"randomLoremSentences", "a few sentences", s(func(f *gofakeit.Faker) string { return sentences(f, f.IntRange(2, 5)) })},
	{"randomLoremParagraph", "paragraph", s(func(f *gofakeit.Faker) string { return sentences(f, f.IntRange(4, 7)) })},
	{"randomLoremSlug", "lorem-ipsum-slug", s(func(f *gofakeit.Faker) string { return strings.ReplaceAll(words(f, 3), " ", "-") })},
	{"randomLoremText", "a few sentences", s(func(f *gofakeit.Faker) string { return sentences(f, f.IntRange(2, 4)) })},
}

var byName = func() map[string]Var {
	m := make(map[string]Var, len(vars))
	for _, v := range vars {
		m[v.Name] = v
	}
	return m
}()
