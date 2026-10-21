//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
)

func TestMutations_createSubnetAndIP(t *testing.T) {
	h := openHarness(t)
	loc := h.createLocation(uniqueName("mut-loc"))
	_, tok := h.createUser(auth.RoleAdmin, uniqueName("mut-admin")+"@test.local")

	cidr := uniqueCIDR()
	code, body := h.do(http.MethodPost, "/api/v1/subnets", tok, map[string]any{
		"cidr": cidr, "name": "Mutation Subnet", "location_id": loc,
		"tags": []string{"mutation"},
	})
	if code != http.StatusCreated {
		t.Fatalf("create subnet status %d: %s", code, body)
	}
	var subnet struct {
		ID   uuid.UUID `json:"id"`
		CIDR string    `json:"cidr"`
	}
	if err := json.Unmarshal(body, &subnet); err != nil {
		t.Fatal(err)
	}
	if subnet.CIDR != cidr {
		t.Fatalf("cidr want %s got %s", cidr, subnet.CIDR)
	}

	addr := strings.TrimSuffix(cidr, ".0/24") + ".10"
	code, body = h.do(http.MethodPost, "/api/v1/subnets/"+subnet.ID.String()+"/ips", tok, map[string]any{
		"address": addr, "hostname": "mut-host", "status": "assigned", "location_id": loc,
	})
	if code != http.StatusCreated {
		t.Fatalf("create ip status %d: %s", code, body)
	}
	var ip struct {
		Address  string `json:"address"`
		Hostname string `json:"hostname"`
	}
	if err := json.Unmarshal(body, &ip); err != nil {
		t.Fatal(err)
	}
	if ip.Address != addr || ip.Hostname != "mut-host" {
		t.Fatalf("unexpected ip: %+v", ip)
	}
}

func TestMutations_sameCIDRAcrossVRFs(t *testing.T) {
	h := openHarness(t)
	loc := h.createLocation(uniqueName("vrf-loc"))
	_, tok := h.createUser(auth.RoleAdmin, uniqueName("vrf-admin")+"@test.local")

	code, body := h.do(http.MethodPost, "/api/v1/vrfs", tok, map[string]any{
		"name": uniqueName("vrf-a"), "rd": fmt.Sprintf("65000:%d", uuid.New().ID()%100000),
	})
	if code != http.StatusCreated {
		t.Fatalf("create vrf a status %d: %s", code, body)
	}
	var vrfA struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(body, &vrfA); err != nil {
		t.Fatal(err)
	}
	code, body = h.do(http.MethodPost, "/api/v1/vrfs", tok, map[string]any{
		"name": uniqueName("vrf-b"), "rd": fmt.Sprintf("65000:%d", uuid.New().ID()%100000),
	})
	if code != http.StatusCreated {
		t.Fatalf("create vrf b status %d: %s", code, body)
	}
	var vrfB struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(body, &vrfB); err != nil {
		t.Fatal(err)
	}

	cidr := uniqueCIDR()
	code, body = h.do(http.MethodPost, "/api/v1/subnets", tok, map[string]any{
		"cidr": cidr, "name": "VRF A net", "location_id": loc, "vrf_id": vrfA.ID,
	})
	if code != http.StatusCreated {
		t.Fatalf("subnet in vrf a status %d: %s", code, body)
	}
	code, body = h.do(http.MethodPost, "/api/v1/subnets", tok, map[string]any{
		"cidr": cidr, "name": "VRF B net", "location_id": loc, "vrf_id": vrfB.ID,
	})
	if code != http.StatusCreated {
		t.Fatalf("same cidr in other VRF should succeed, got %d: %s", code, body)
	}

	// Same VRF must still reject overlap.
	code, body = h.do(http.MethodPost, "/api/v1/subnets", tok, map[string]any{
		"cidr": cidr, "name": "Dup VRF A", "location_id": loc, "vrf_id": vrfA.ID,
	})
	if code == http.StatusCreated {
		t.Fatalf("duplicate cidr in same VRF should fail, body=%s", body)
	}
}

func TestMutations_allocateChild(t *testing.T) {
	h := openHarness(t)
	loc := h.createLocation(uniqueName("child-loc"))
	_, tok := h.createUser(auth.RoleAdmin, uniqueName("child-admin")+"@test.local")

	parentCIDR := uniqueCIDR()
	code, body := h.do(http.MethodPost, "/api/v1/subnets", tok, map[string]any{
		"cidr": parentCIDR, "name": "Parent", "location_id": loc,
	})
	if code != http.StatusCreated {
		t.Fatalf("parent status %d: %s", code, body)
	}
	var parent struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(body, &parent); err != nil {
		t.Fatal(err)
	}

	code, body = h.do(http.MethodPost, "/api/v1/subnets/"+parent.ID.String()+"/allocate-child", tok, map[string]any{
		"prefix_length": 28, "name": "Child /28",
	})
	if code != http.StatusCreated {
		t.Fatalf("allocate-child status %d: %s", code, body)
	}
	var child struct {
		ID       uuid.UUID  `json:"id"`
		CIDR     string     `json:"cidr"`
		ParentID *uuid.UUID `json:"parent_id"`
	}
	if err := json.Unmarshal(body, &child); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(child.CIDR, "/28") {
		t.Fatalf("expected /28 child, got %s", child.CIDR)
	}
	if child.ParentID == nil || *child.ParentID != parent.ID {
		t.Fatalf("expected parent_id=%s, got %v", parent.ID, child.ParentID)
	}
}

func TestMutations_approveIPRequest(t *testing.T) {
	h := openHarness(t)
	loc := h.createLocation(uniqueName("req-loc"))
	cidr := uniqueCIDR()
	subnetID := h.createSubnet(cidr, "Request Net", loc)

	reqID, reqTok := h.createUser(auth.RoleReadOnly, uniqueName("requester")+"@test.local")
	h.setUserLocations(reqID, []uuid.UUID{loc})
	editorID, editorTok := h.createUser(auth.RoleEditor, uniqueName("approver")+"@test.local")
	h.setUserLocations(editorID, []uuid.UUID{loc})

	code, body := h.do(http.MethodPost, "/api/v1/ip-requests", reqTok, map[string]any{
		"subnet_id": subnetID, "hostname": "req-host", "notes": "need an IP",
	})
	if code != http.StatusCreated {
		t.Fatalf("create request status %d: %s", code, body)
	}
	var created struct {
		ID     uuid.UUID `json:"id"`
		Status string    `json:"status"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != "pending" {
		t.Fatalf("status want pending got %s", created.Status)
	}

	code, body = h.do(http.MethodPost, "/api/v1/ip-requests/"+created.ID.String()+"/approve", editorTok, map[string]any{
		"review_notes": "ok",
	})
	if code != http.StatusOK {
		t.Fatalf("approve status %d: %s", code, body)
	}
	var approved struct {
		Status           string `json:"status"`
		AllocatedAddress string `json:"allocated_address"`
	}
	if err := json.Unmarshal(body, &approved); err != nil {
		t.Fatal(err)
	}
	if approved.Status != "approved" {
		t.Fatalf("want approved got %s", approved.Status)
	}
	if approved.AllocatedAddress == "" {
		t.Fatalf("expected allocated address, body=%s", body)
	}
	if !strings.HasPrefix(approved.AllocatedAddress, strings.TrimSuffix(cidr, ".0/24")+".") {
		t.Fatalf("allocated %s not in %s", approved.AllocatedAddress, cidr)
	}
}

func TestMutations_mfaLoginChallenge(t *testing.T) {
	h := openHarness(t)
	email := uniqueName("mfa") + "@test.local"
	_, tok := h.createUser(auth.RoleEditor, email)

	code, body := h.do(http.MethodPost, "/api/v1/auth/mfa/enroll/begin", tok, nil)
	if code != http.StatusOK {
		t.Fatalf("enroll begin status %d: %s", code, body)
	}
	var begin struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(body, &begin); err != nil {
		t.Fatal(err)
	}
	if begin.Secret == "" {
		t.Fatal("expected plaintext TOTP secret with AllowPlainSecrets")
	}
	otpCode, err := totp.GenerateCode(begin.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	code, body = h.do(http.MethodPost, "/api/v1/auth/mfa/enroll/confirm", tok, map[string]any{
		"code": otpCode,
	})
	if code != http.StatusOK {
		t.Fatalf("enroll confirm status %d: %s", code, body)
	}

	code, body = h.do(http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"email": email, "password": "test-pass-1",
	})
	if code != http.StatusOK {
		t.Fatalf("login status %d: %s", code, body)
	}
	var login struct {
		MFARequired bool   `json:"mfa_required"`
		MFAToken    string `json:"mfa_token"`
		Token       string `json:"token"`
	}
	if err := json.Unmarshal(body, &login); err != nil {
		t.Fatal(err)
	}
	if !login.MFARequired || login.MFAToken == "" {
		t.Fatalf("expected MFA challenge, body=%s", body)
	}
	if login.Token != "" {
		t.Fatal("session token must not be issued before MFA verify")
	}

	verifyCode, err := totp.GenerateCode(begin.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	code, body = h.do(http.MethodPost, "/api/v1/auth/mfa/verify", "", map[string]any{
		"mfa_token": login.MFAToken, "code": verifyCode,
	})
	if code != http.StatusOK {
		t.Fatalf("mfa verify status %d: %s", code, body)
	}
	var verified struct {
		Token string `json:"token"`
		User  struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &verified); err != nil {
		t.Fatal(err)
	}
	if verified.Token == "" || verified.User.Email != email {
		t.Fatalf("expected session after MFA, body=%s", body)
	}
}
