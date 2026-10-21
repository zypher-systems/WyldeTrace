package oidcauth

import (
	"strings"

	"github.com/wyldetechnologies/wyldetrace/internal/auth"
)

// RolePreview explains how a role would be resolved from claims.
type RolePreview struct {
	Role          auth.Role `json:"role"`
	Source        string    `json:"source"` // direct | groups | default
	DirectRole    string    `json:"direct_role,omitempty"`
	MatchedGroups []string  `json:"matched_groups"`
	Groups        []string  `json:"groups"`
	DefaultRole   auth.Role `json:"default_role"`
	WouldSync     bool      `json:"would_sync,omitempty"`
	CurrentRole   string    `json:"current_role,omitempty"`
}

// ParseRoleMap parses "admin:g1|g2,editor:g3,readonly:users" into role -> group set.
func ParseRoleMap(raw string) map[auth.Role][]string {
	out := map[auth.Role][]string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		roleStr, groupsCSV, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		role, err := auth.ParseRole(strings.TrimSpace(roleStr))
		if err != nil {
			continue
		}
		groups := []string{}
		for _, g := range strings.Split(groupsCSV, "|") {
			g = strings.TrimSpace(g)
			if g != "" {
				groups = append(groups, strings.ToLower(g))
			}
		}
		if len(groups) > 0 {
			out[role] = groups
		}
	}
	return out
}

// ResolveRole picks the highest-ranked role from a direct claim and/or group membership.
// If neither yields a role, defaultRole is used (readonly when empty).
func ResolveRole(defaultRole auth.Role, roleMap map[auth.Role][]string, directRole string, groups []string) auth.Role {
	var best auth.Role
	found := false
	if r, err := auth.ParseRole(strings.TrimSpace(strings.ToLower(directRole))); err == nil {
		best = r
		found = true
	}
	groupSet := map[string]bool{}
	for _, g := range groups {
		groupSet[strings.ToLower(strings.TrimSpace(g))] = true
	}
	for role, mapped := range roleMap {
		for _, g := range mapped {
			if groupSet[g] {
				if !found || role.Rank() > best.Rank() {
					best = role
					found = true
				}
				break
			}
		}
	}
	if !found {
		if defaultRole == "" {
			return auth.RoleReadOnly
		}
		return defaultRole
	}
	return best
}

// PreviewRole returns the resolved role plus which claim/group inputs drove it.
func PreviewRole(defaultRole auth.Role, roleMap map[auth.Role][]string, directRole string, groups []string) RolePreview {
	direct := strings.TrimSpace(strings.ToLower(directRole))
	cleanGroups := make([]string, 0, len(groups))
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g != "" {
			cleanGroups = append(cleanGroups, g)
		}
	}
	if defaultRole == "" {
		defaultRole = auth.RoleReadOnly
	}

	matched := []string{}
	groupSet := map[string]bool{}
	for _, g := range cleanGroups {
		groupSet[strings.ToLower(g)] = true
	}
	bestGroupRank := -1
	for role, mapped := range roleMap {
		for _, g := range mapped {
			if !groupSet[g] {
				continue
			}
			matched = append(matched, g)
			if role.Rank() > bestGroupRank {
				bestGroupRank = role.Rank()
			}
		}
	}
	matched = uniqueLower(matched)

	resolved := ResolveRole(defaultRole, roleMap, direct, cleanGroups)
	source := "default"
	if directRoleParsed, err := auth.ParseRole(direct); err == nil {
		if bestGroupRank > directRoleParsed.Rank() {
			source = "groups"
		} else {
			source = "direct"
		}
	} else if bestGroupRank >= 0 {
		source = "groups"
	}

	return RolePreview{
		Role:          resolved,
		Source:        source,
		DirectRole:    direct,
		MatchedGroups: matched,
		Groups:        cleanGroups,
		DefaultRole:   defaultRole,
	}
}

func uniqueLower(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
