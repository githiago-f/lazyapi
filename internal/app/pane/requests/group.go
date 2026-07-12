package requests

import (
	"sort"
	"strings"
)

type TagGroup struct {
	Tag   string
	Items []RequestItem
}

func GroupByTags(items []RequestItem) []TagGroup {
	groups := map[string][]RequestItem{}
	tagOrder := []string{}

	for _, item := range items {
		tag := "Untagged"
		if len(item.Tags) > 0 && item.Tags[0] != "" {
			tag = item.Tags[0]
		}
		if _, ok := groups[tag]; !ok {
			tagOrder = append(tagOrder, tag)
		}
		groups[tag] = append(groups[tag], item)
	}

	sort.Strings(tagOrder)

	result := make([]TagGroup, 0, len(tagOrder))
	for _, tag := range tagOrder {
		groupItems := groups[tag]
		sort.SliceStable(groupItems, func(i, j int) bool {
			if c := strings.Compare(strings.ToLower(groupItems[i].URI), strings.ToLower(groupItems[j].URI)); c != 0 {
				return c < 0
			}
			return strings.Compare(strings.ToLower(groupItems[i].Method.Label()), strings.ToLower(groupItems[j].Method.Label())) < 0
		})
		result = append(result, TagGroup{Tag: tag, Items: groupItems})
	}

	return result
}
