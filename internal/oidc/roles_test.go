package oidcauth

import (
	"testing"

	"github.com/wyldetechnologies/wyldetrace/internal/auth"
)

func TestParseRoleMap(t *testing.T) {
	m := ParseRoleMap("admin:wt-admins|net-admins,editor:wt-editors,readonly:users")
	if len(m[auth.RoleAdmin]) != 2 || m[auth.RoleAdmin][0] != "wt-admins" {
		t.Fatalf("admin map: %#v", m[auth.RoleAdmin])
	}
	if len(m[auth.RoleEditor]) != 1 {
		t.Fatalf("editor map: %#v", m[auth.RoleEditor])
	}
}

func TestResolveRoleGroups(t *testing.T) {
	m := ParseRoleMap("admin:wt-admins,editor:wt-editors,readonly:users")
	got := ResolveRole(auth.RoleReadOnly, m, "", []string{"wt-editors", "users"})
	if got != auth.RoleEditor {
		t.Fatalf("got %s want editor", got)
	}
	got = ResolveRole(auth.RoleReadOnly, m, "", []string{"wt-admins"})
	if got != auth.RoleAdmin {
		t.Fatalf("got %s want admin", got)
	}
}

func TestResolveRoleDirect(t *testing.T) {
	got := ResolveRole(auth.RoleReadOnly, nil, "reportview", nil)
	if got != auth.RoleReportView {
		t.Fatalf("got %s", got)
	}
}

func TestResolveRoleDefaultWhenUnmapped(t *testing.T) {
	m := ParseRoleMap("admin:wt-admins")
	got := ResolveRole(auth.RoleReadOnly, m, "", []string{"other"})
	if got != auth.RoleReadOnly {
		t.Fatalf("got %s want readonly", got)
	}
}

func TestPreviewRoleSources(t *testing.T) {
	m := ParseRoleMap("admin:wt-admins,editor:wt-editors")
	p := PreviewRole(auth.RoleReadOnly, m, "", []string{"wt-editors"})
	if p.Role != auth.RoleEditor || p.Source != "groups" {
		t.Fatalf("groups preview: %#v", p)
	}
	p = PreviewRole(auth.RoleReadOnly, m, "admin", []string{"wt-editors"})
	if p.Role != auth.RoleAdmin || p.Source != "direct" {
		t.Fatalf("direct preview: %#v", p)
	}
	p = PreviewRole(auth.RoleReadOnly, m, "readonly", []string{"wt-admins"})
	if p.Role != auth.RoleAdmin || p.Source != "groups" {
		t.Fatalf("groups beat lower direct: %#v", p)
	}
	p = PreviewRole(auth.RoleReadOnly, m, "", []string{"other"})
	if p.Role != auth.RoleReadOnly || p.Source != "default" {
		t.Fatalf("default preview: %#v", p)
	}
}
