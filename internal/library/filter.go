package library

import (
	"slices"
	"strings"
)

func Filter(profiles []Profile, query, group, scope string) []Profile {
	query = strings.ToLower(strings.TrimSpace(query))
	var result []Profile
	for _, p := range profiles {
		if (scope == "favorites" && !p.Favorite) || (scope == "recent" && p.LastUsed.IsZero()) || (group != "" && p.Group != group) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.Host+" "+p.Username+" "+p.Group), query) {
			continue
		}
		result = append(result, p)
	}
	slices.SortStableFunc(result, func(a, b Profile) int {
		if scope == "recent" {
			if n := b.LastUsed.Compare(a.LastUsed); n != 0 {
				return n
			}
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return result
}

func Groups(profiles []Profile) []string {
	var groups []string
	for _, p := range profiles {
		if p.Group != "" && !slices.Contains(groups, p.Group) {
			groups = append(groups, p.Group)
		}
	}
	slices.Sort(groups)
	return groups
}
