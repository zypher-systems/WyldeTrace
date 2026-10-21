package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/middleware"
)

const (
	totpIssuer       = "WyldeTrace"
	recoveryCodeCount = 10
	recoveryCodeBytes = 5
)

type mfaBeginResponse struct {
	Secret    string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
}

type mfaConfirmRequest struct {
	Code string `json:"code"`
}

type mfaConfirmResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
	Status        string   `json:"status"`
}

type mfaDisableRequest struct {
	Password     string `json:"password"`
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code"`
}

type mfaVerifyRequest struct {
	MFAToken     string `json:"mfa_token"`
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code"`
}

type mfaStatusDTO struct {
	TOTPEnabled bool `json:"totp_enabled"`
}

func (a *API) MFAStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok || claims.IsAPIToken {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, err := a.Store.Queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	httpx.JSON(w, http.StatusOK, mfaStatusDTO{TOTPEnabled: user.TotpEnabled})
}

func (a *API) MFAEnrollBegin(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok || claims.IsAPIToken {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, err := a.Store.Queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	if user.TotpEnabled {
		httpx.Error(w, http.StatusConflict, "MFA already enabled; disable first to re-enroll")
		return
	}
	if a.Secrets == nil && !a.AllowPlainSecrets {
		httpx.Error(w, http.StatusBadRequest, "SETTINGS_ENCRYPTION_KEY is required to enroll MFA")
		return
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: user.Email,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to generate TOTP secret")
		return
	}
	sealed, err := a.sealSecret(key.Secret())
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = a.Store.Queries.SetUserTOTPBegin(r.Context(), sqlcdb.SetUserTOTPBeginParams{
		ID:         user.ID,
		TotpSecret: sealed,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r.Context(), "user.mfa_enroll_begin", "user", user.ID.String(), map[string]any{})
	httpx.JSON(w, http.StatusOK, mfaBeginResponse{
		Secret:     key.Secret(),
		OTPAuthURL: key.URL(),
	})
}

func (a *API) MFAEnrollConfirm(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok || claims.IsAPIToken {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req mfaConfirmRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		httpx.Error(w, http.StatusBadRequest, "code required")
		return
	}
	user, err := a.Store.Queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	if user.TotpSecret == "" {
		httpx.Error(w, http.StatusBadRequest, "start enrollment first")
		return
	}
	if user.TotpEnabled {
		httpx.Error(w, http.StatusConflict, "MFA already enabled")
		return
	}
	secret, err := a.openSecret(user.TotpSecret)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !totp.Validate(req.Code, secret) {
		httpx.Error(w, http.StatusUnauthorized, "invalid code")
		return
	}
	plain, hashes, err := generateRecoveryCodes()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to generate recovery codes")
		return
	}
	raw, _ := json.Marshal(hashes)
	_, err = a.Store.Queries.SetUserTOTPConfirm(r.Context(), sqlcdb.SetUserTOTPConfirmParams{
		ID:            user.ID,
		TotpSecret:    user.TotpSecret,
		RecoveryCodes: raw,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r.Context(), "user.mfa_enable", "user", user.ID.String(), map[string]any{})
	httpx.JSON(w, http.StatusOK, mfaConfirmResponse{
		RecoveryCodes: plain,
		Status:        "enabled",
	})
}

func (a *API) MFADisable(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok || claims.IsAPIToken {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req mfaDisableRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, err := a.Store.Queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	if !user.TotpEnabled {
		httpx.Error(w, http.StatusBadRequest, "MFA is not enabled")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		httpx.Error(w, http.StatusUnauthorized, "password incorrect")
		return
	}
	code := strings.TrimSpace(req.Code)
	recovery := strings.TrimSpace(req.RecoveryCode)
	secret, serr := a.openSecret(user.TotpSecret)
	if serr != nil {
		httpx.Error(w, http.StatusInternalServerError, serr.Error())
		return
	}
	okCode := code != "" && totp.Validate(code, secret)
	okRecovery := false
	var remaining json.RawMessage
	if !okCode && recovery != "" {
		okRecovery, remaining = consumeRecoveryCode(user.RecoveryCodes, recovery)
	}
	if !okCode && !okRecovery {
		httpx.Error(w, http.StatusUnauthorized, "invalid MFA or recovery code")
		return
	}
	if okRecovery && remaining != nil {
		_, _ = a.Store.Queries.SetUserRecoveryCodes(r.Context(), sqlcdb.SetUserRecoveryCodesParams{
			ID: user.ID, RecoveryCodes: remaining,
		})
	}
	_, err = a.Store.Queries.DisableUserTOTP(r.Context(), user.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r.Context(), "user.mfa_disable", "user", user.ID.String(), map[string]any{})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

// AdminDisableMFA is the break-glass path for admins (no TOTP required).
func (a *API) AdminDisableMFA(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}
	user, err := a.Store.Queries.GetUserByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	_, err = a.Store.Queries.DisableUserTOTP(r.Context(), user.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.audit(r.Context(), "user.mfa_disable_admin", "user", user.ID.String(), map[string]any{})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (a *API) MFAVerify(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !loginAllowed(ip) {
		httpx.Error(w, http.StatusTooManyRequests, "too many failed logins; try again later")
		return
	}
	var req mfaVerifyRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.MFAToken = strings.TrimSpace(req.MFAToken)
	req.Code = strings.TrimSpace(req.Code)
	req.RecoveryCode = strings.TrimSpace(req.RecoveryCode)
	if req.MFAToken == "" {
		httpx.Error(w, http.StatusBadRequest, "mfa_token required")
		return
	}
	claims, err := a.Tokens.Parse(req.MFAToken)
	if err != nil || !claims.MFAPending {
		loginFail(ip)
		httpx.Error(w, http.StatusUnauthorized, "invalid or expired MFA token")
		return
	}
	user, err := a.Store.Queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil || !user.TotpEnabled {
		loginFail(ip)
		httpx.Error(w, http.StatusUnauthorized, "MFA not available")
		return
	}
	secret, serr := a.openSecret(user.TotpSecret)
	if serr != nil {
		loginFail(ip)
		httpx.Error(w, http.StatusInternalServerError, serr.Error())
		return
	}
	okCode := req.Code != "" && totp.Validate(req.Code, secret)
	okRecovery := false
	var remaining json.RawMessage
	if !okCode && req.RecoveryCode != "" {
		okRecovery, remaining = consumeRecoveryCode(user.RecoveryCodes, req.RecoveryCode)
	}
	if !okCode && !okRecovery {
		loginFail(ip)
		httpx.Error(w, http.StatusUnauthorized, "invalid MFA or recovery code")
		return
	}
	if okRecovery && remaining != nil {
		_, _ = a.Store.Queries.SetUserRecoveryCodes(r.Context(), sqlcdb.SetUserRecoveryCodesParams{
			ID: user.ID, RecoveryCodes: remaining,
		})
	}
	loginSuccess(ip)
	token, err := a.Tokens.Issue(user.ID, user.Email, claims.Role)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "token issue failed")
		return
	}
	locs, _ := a.Store.Queries.ListLocationIDsByUser(r.Context(), user.ID)
	dto := toUserDTO(user, locs)
	dto.UsingDefaultPassword = a.usingBootstrapPassword(user.PasswordHash)
	httpx.JSON(w, http.StatusOK, loginResponse{Token: token, User: dto})
}

func generateRecoveryCodes() (plain []string, hashes []string, err error) {
	plain = make([]string, 0, recoveryCodeCount)
	hashes = make([]string, 0, recoveryCodeCount)
	for i := 0; i < recoveryCodeCount; i++ {
		buf := make([]byte, recoveryCodeBytes)
		if _, err = rand.Read(buf); err != nil {
			return nil, nil, err
		}
		code := strings.ToUpper(hex.EncodeToString(buf))
		// Format as XXXX-XXXX-XX for readability when 10 hex chars.
		if len(code) >= 10 {
			code = fmt.Sprintf("%s-%s-%s", code[:4], code[4:8], code[8:])
		}
		plain = append(plain, code)
		hashes = append(hashes, hashRecoveryCode(code))
	}
	return plain, hashes, nil
}

func hashRecoveryCode(code string) string {
	norm := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

func consumeRecoveryCode(raw json.RawMessage, code string) (bool, json.RawMessage) {
	var hashes []string
	if len(raw) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(raw, &hashes); err != nil {
		return false, nil
	}
	want := hashRecoveryCode(code)
	idx := -1
	for i, h := range hashes {
		if subtle.ConstantTimeCompare([]byte(h), []byte(want)) == 1 {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false, nil
	}
	remaining := append(hashes[:idx:idx], hashes[idx+1:]...)
	out, _ := json.Marshal(remaining)
	return true, out
}
