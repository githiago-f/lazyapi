package cli

import (
	"fmt"
	"os"

	"github.com/githiago-f/lazyapi/internal/permit"
)

const allowUsage = `usage: lazyapi allow <origin> [--writes]   remember an origin for the MCP server
       lazyapi allow --list                 list remembered origins
       lazyapi allow --revoke <origin>      forget one

An origin is scheme://host[:port] (https://api.example.com) or "localhost"
for every local server. Without --writes only GET, HEAD and OPTIONS are
allowed. Answers are stored in your config directory
($LAZYAPI_PERMISSIONS overrides the file), never in the project.

This command is for people: agents must ask the user instead of running it.`

// Allow implements `lazyapi allow`.
func Allow(args []string) error {
	fs := newFlagSet("allow")
	writes := fs.Bool("writes", false, "also allow POST, PUT, PATCH, DELETE…")
	list := fs.Bool("list", false, "list remembered origins")
	revoke := fs.Bool("revoke", false, "forget the origin")
	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return usageErr("%s", allowUsage)
	}
	store, err := permit.DefaultStore()
	if err != nil {
		return runtimeErr(err)
	}
	if *list {
		if len(pos) != 0 {
			return usageErr("%s", allowUsage)
		}
		grants, err := store.List()
		if err != nil {
			return runtimeErr(err)
		}
		if len(grants) == 0 {
			fmt.Println("No remembered origins (" + store.Path + ").")
			return nil
		}
		for _, g := range grants {
			access := "reads"
			if g.Writes {
				access = "reads and writes"
			}
			fmt.Printf("%-40s %-17s since %s\n", g.Origin, access, g.GrantedAt.Format("2006-01-02"))
		}
		return nil
	}
	if len(pos) != 1 {
		return usageErr("%s", allowUsage)
	}
	origin, err := permit.Normalize(pos[0])
	if err != nil {
		return usageErr("%v", err)
	}
	if *revoke {
		found, err := store.Revoke(origin)
		if err != nil {
			return runtimeErr(err)
		}
		if !found {
			fmt.Fprintf(os.Stderr, "%s was not remembered\n", origin)
			return nil
		}
		fmt.Printf("Forgot %s\n", origin)
		return nil
	}
	if err := store.Grant(origin, *writes); err != nil {
		return runtimeErr(err)
	}
	access := "reads"
	if *writes || origin == permit.Loopback {
		access = "reads and writes"
	}
	fmt.Printf("Allowed %s for %s (%s)\n", origin, access, store.Path)
	return nil
}
