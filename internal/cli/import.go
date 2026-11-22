package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/importer"
	"github.com/githiago-f/lazyapi/internal/report"
	"github.com/githiago-f/lazyapi/internal/store"
)

const importUsage = `usage: lazyapi import <curl|postman|har> <source> --into <spec.yml> [--dry-run] [--json]

  curl     <source> is a curl command line, or - to read it from stdin
  postman  <source> is a Postman collection (v2.0/v2.1 JSON)
  har      <source> is a HAR file (browser DevTools → Save all as HAR)

Each request becomes an operation (path parameters, query/header
parameters, request body and response examples with inferred schemas,
security schemes). Credentials are never copied into the spec. Existing
operations are kept. --into creates the spec when it does not exist.`

// Import implements `lazyapi import`.
func Import(args []string) error {
	fs := newFlagSet("import")
	into := fs.String("into", "", "spec to add the operations to (created if missing)")
	dryRun := fs.Bool("dry-run", false, "show what would be imported without writing")
	asJSON := fs.Bool("json", false, "print the result as JSON")
	pos, err := parseInterleaved(fs, args)
	if err != nil || len(pos) < 2 || *into == "" {
		return usageErr("%s", importUsage)
	}
	kind, source := pos[0], strings.Join(pos[1:], " ")

	spec, title, err := openOrNewSpec(*into)
	if err != nil {
		return err
	}

	var reqs []importer.Request
	var extra importer.Extras
	switch kind {
	case "curl":
		cmd := source
		if source == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return runtimeErr(err)
			}
			cmd = string(data)
		}
		r, err := importer.ParseCurl(cmd)
		if err != nil {
			return usageErr("curl: %v", err)
		}
		reqs = []importer.Request{r}
	case "postman":
		data, err := os.ReadFile(source)
		if err != nil {
			return runtimeErr(err)
		}
		if reqs, extra, err = importer.ParsePostman(data); err != nil {
			return runtimeErr(fmt.Errorf("postman: %w", err))
		}
		if title == "" {
			title = extra.Title
		}
	case "har":
		data, err := os.ReadFile(source)
		if err != nil {
			return runtimeErr(err)
		}
		if reqs, err = importer.ParseHAR(data); err != nil {
			return runtimeErr(fmt.Errorf("har: %w", err))
		}
	default:
		return usageErr("unknown import source %q\n%s", kind, importUsage)
	}
	if spec.Info != nil && title != "" && spec.Info.Title == "Imported API" {
		spec.Info.Title = title
	}

	specPath := *into
	if *dryRun {
		// Apply to a throwaway copy so scripts are not written either.
		specPath = ""
	}
	res, err := importer.Apply(spec, specPath, reqs)
	if err != nil {
		return runtimeErr(err)
	}
	if !*dryRun {
		if err := store.SaveSpec(*into, spec); err != nil {
			return runtimeErr(err)
		}
		if err := importer.WriteExtras(*into, extra, &res); err != nil {
			return runtimeErr(err)
		}
	}

	if *asJSON {
		return report.WriteJSON(os.Stdout, res)
	}
	verb := "Added"
	if *dryRun {
		verb = "Would add"
	}
	fmt.Printf("%s %d operation(s) to %s\n", verb, len(res.Added), *into)
	for _, a := range res.Added {
		fmt.Println("  + " + a)
	}
	for _, s := range res.Skipped {
		fmt.Println("  - skipped " + s)
	}
	for _, t := range res.Files {
		fmt.Println("  ✎ " + t)
	}
	for _, n := range res.Notes {
		fmt.Println("  ! " + n)
	}
	if len(res.Added) > 0 && !*dryRun {
		fmt.Printf("\nNext: lazyapi validate spec %s && lazyapi run %s\n", *into, *into)
	}
	return nil
}

// openOrNewSpec loads path, or returns a new empty spec when it is missing.
func openOrNewSpec(path string) (*openapi3.T, string, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return importer.NewSpec(""), "", nil
	}
	spec, err := loadSpec(path)
	if err != nil {
		return nil, "", err
	}
	return spec, "", nil
}
