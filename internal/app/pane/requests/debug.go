package requests

import (
	"fmt"
	"strings"

	"github.com/githiago-f/lazyapi/internal/model"
)

func DebugRenderList(width, height int) string {
	items := []RequestItem{
		{Method: model.GET, URI: "/api/pets", About: model.About{Summary: "List all pets"}, Tags: []string{"Pets"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/pets", Method: "GET"}},
		{Method: model.POST, URI: "/api/pets", About: model.About{Summary: "Create a pet"}, Tags: []string{"Pets"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/pets", Method: "POST"}},
		{Method: model.GET, URI: "/api/pets/{id}", About: model.About{Summary: "Get pet by ID"}, Tags: []string{"Pets"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/pets/{id}", Method: "GET"}},
		{Method: model.DELETE, URI: "/api/pets/{id}", About: model.About{Summary: "Delete a pet"}, Tags: []string{"Pets"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/pets/{id}", Method: "DELETE"}},
		{Method: model.GET, URI: "/api/sales", About: model.About{Summary: "List sales"}, Tags: []string{"Sales"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/sales", Method: "GET"}},
		{Method: model.POST, URI: "/api/sales", About: model.About{Summary: "Create a sale"}, Tags: []string{"Sales"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/sales", Method: "POST"}},
		{Method: model.GET, URI: "/api/sales/{id}", About: model.About{Summary: "Get sale by ID"}, Tags: []string{"Sales"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/sales/{id}", Method: "GET"}},
		{Method: model.GET, URI: "/api/reports/daily", About: model.About{Summary: "Daily report"}, Tags: []string{}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/reports/daily", Method: "GET"}},
		{Method: model.GET, URI: "/api/health", About: model.About{Summary: ""}, Tags: []string{}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/health", Method: "GET"}},
	}

	rl := NewRequestList()
	rl = rl.SetItems(items)
	rl.SetSize(width, height)
	return rl.View().Content
}

func DebugRenderListFiltered(width, height int) string {
	items := []RequestItem{
		{Method: model.GET, URI: "/api/pets", About: model.About{Summary: "List all pets"}, Tags: []string{"Pets"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/pets", Method: "GET"}},
		{Method: model.POST, URI: "/api/pets", About: model.About{Summary: "Create a pet"}, Tags: []string{"Pets"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/pets", Method: "POST"}},
		{Method: model.GET, URI: "/api/pets/{id}", About: model.About{Summary: "Get pet by ID"}, Tags: []string{"Pets"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/pets/{id}", Method: "GET"}},
		{Method: model.DELETE, URI: "/api/pets/{id}", About: model.About{Summary: "Delete a pet"}, Tags: []string{"Pets"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/pets/{id}", Method: "DELETE"}},
		{Method: model.GET, URI: "/api/sales", About: model.About{Summary: "List sales"}, Tags: []string{"Sales"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/sales", Method: "GET"}},
		{Method: model.POST, URI: "/api/sales", About: model.About{Summary: "Create a sale"}, Tags: []string{"Sales"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/sales", Method: "POST"}},
		{Method: model.GET, URI: "/api/sales/{id}", About: model.About{Summary: "Get sale by ID"}, Tags: []string{"Sales"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/sales/{id}", Method: "GET"}},
		{Method: model.GET, URI: "/api/reports/daily", About: model.About{Summary: "Daily report"}, Tags: []string{}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/reports/daily", Method: "GET"}},
		{Method: model.GET, URI: "/api/health", About: model.About{Summary: ""}, Tags: []string{}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/health", Method: "GET"}},
	}

	rl := NewRequestList()
	rl.filter.SetValue("pet")
	rl.filtering = true
	rl = rl.SetItems(items)
	rl.SetSize(width, height)
	return rl.View().Content
}

func DebugRenderTagsOverlay(width int) string {
	item := RequestItem{
		Method: model.GET,
		URI:    "/api/pets",
		About:  model.About{Summary: "List all pets"},
		Tags:   []string{"pets", "animals"},
		OpenAPIRef: &model.OpenAPIRef{
			FilePath: "spec.yml",
			Path:     "/api/pets",
			Method:   "GET",
		},
	}
	to := NewTagsOverlay(item)
	to.width = width
	return to.View().Content
}

func DebugRenderAll() string {
	var b strings.Builder

	b.WriteString("=== REQUEST LIST (normal) ===\n")
	b.WriteString(DebugRenderList(50, 14))
	b.WriteString("\n\n")

	b.WriteString("=== REQUEST LIST (filtered 'pet') ===\n")
	b.WriteString(DebugRenderListFiltered(50, 10))
	b.WriteString("\n\n")

	b.WriteString("=== TAGS OVERLAY ===\n")
	b.WriteString(DebugRenderTagsOverlay(40))

	return b.String()
}

func DebugCursorMovement() string {
	items := []RequestItem{
		{Method: model.GET, URI: "/api/pets", About: model.About{Summary: "List all pets"}, Tags: []string{"Pets"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/pets", Method: "GET"}},
		{Method: model.POST, URI: "/api/pets", About: model.About{Summary: "Create a pet"}, Tags: []string{"Pets"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/pets", Method: "POST"}},
		{Method: model.DELETE, URI: "/api/pets/{id}", About: model.About{Summary: "Delete a pet"}, Tags: []string{"Pets"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/pets/{id}", Method: "DELETE"}},
		{Method: model.GET, URI: "/api/sales", About: model.About{Summary: "List sales"}, Tags: []string{"Sales"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/sales", Method: "GET"}},
		{Method: model.POST, URI: "/api/sales", About: model.About{Summary: "Create a sale"}, Tags: []string{"Sales"}, FileName: "spec.yml", OpenAPIRef: &model.OpenAPIRef{FilePath: "spec.yml", Path: "/api/sales", Method: "POST"}},
	}

	rl := NewRequestList()
	rl = rl.SetItems(items)
	rl.SetSize(50, 12)

	var steps []string
	for i := 0; i < len(rl.filtered); i++ {
		rl.cursor = i
		steps = append(steps, fmt.Sprintf("--- cursor=%d (item: %s %s) ---\n%s", i, rl.filtered[i].Method.Label(), rl.filtered[i].URI, rl.View().Content))
	}

	return strings.Join(steps, "\n")
}
