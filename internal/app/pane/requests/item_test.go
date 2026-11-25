package requests

import (
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/model"
)

func TestGroupByTags(t *testing.T) {
	items := []RequestItem{
		{URI: "/items", Method: model.GET, FileName: "test.yml", Tags: []string{"items"}},
		{URI: "/items/{id}", Method: model.GET, FileName: "test.yml", Tags: []string{"items"}},
		{URI: "/users", Method: model.POST, FileName: "test.yml", Tags: []string{"users"}},
	}

	groups := GroupByTags(items)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if len(groups[0].Items)+len(groups[1].Items) != 3 {
		t.Fatalf("expected 3 total items across groups, got %d", len(groups[0].Items)+len(groups[1].Items))
	}
}

func TestGroupByTags_Untagged(t *testing.T) {
	items := []RequestItem{
		{URI: "/items", Method: model.GET, FileName: "test.yml"},
	}

	groups := GroupByTags(items)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group (Untagged), got %d", len(groups))
	}
	if groups[0].Tag != "Untagged" {
		t.Fatalf("expected group 'Untagged', got %q", groups[0].Tag)
	}
}

func TestRequestItem_FilterValue(t *testing.T) {
	item := RequestItem{
		URI:    "/items",
		Method: model.GET,
		About:  model.About{Summary: "List items"},
		Tags:   []string{"items"},
	}
	fv := item.FilterValue()
	if !strings.Contains(fv, "/items") || !strings.Contains(fv, "List items") {
		t.Errorf("FilterValue = %q, should contain '/items' and 'List items'", fv)
	}
	if !strings.Contains(fv, "items") {
		t.Errorf("FilterValue = %q, should contain tag 'items'", fv)
	}
}

func TestRequestItem_RenderCompact(t *testing.T) {
	item := RequestItem{
		URI:    "/items",
		Method: model.GET,
	}
	rendered := RenderCompact(item, 80, false, false)
	if !strings.Contains(rendered, "GET") || !strings.Contains(rendered, "/items") {
		t.Errorf("RenderCompact = %q, should contain both 'GET' and '/items'", rendered)
	}
}
