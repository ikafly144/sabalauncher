package msa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ikafly144/sabalauncher/pkg/browser"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type MinecraftAccount struct {
	Username string    `json:"username"`
	UUID     uuid.UUID `json:"uuid"`
	// AccessToken  string    `json:"access_token"`
	// ExpiresIn    time.Time `json:"expires_in"`
	XBLToken string `json:"xbl_token"`
	// XSTSToken string `json:"xsts_token"`
	UserHash string `json:"user_hash"`
}

type xblAuthReqest struct {
	Properties   xblAuthProperties `json:"Properties"`
	RelyingParty string            `json:"RelyingParty"`
	TokenType    string            `json:"TokenType"`
}

type xblAuthProperties struct {
	AuthMethod string `json:"AuthMethod"`
	SiteName   string `json:"SiteName"`
	RpsTicket  string `json:"RpsTicket"`
}

type xblAuthResult struct {
	IssueInstant  time.Time `json:"IssueInstant"`
	NotAfter      time.Time `json:"NotAfter"`
	Token         string    `json:"Token"`
	DisplayClaims struct {
		Xui []struct {
			Uhs string `json:"uhs"`
		} `json:"xui"`
	} `json:"DisplayClaims"`
}

type xstsRequest struct {
	Properties   xstsProperties `json:"Properties"`
	RelyingParty string         `json:"RelyingParty"`
	TokenType    string         `json:"TokenType"`
}

type xstsProperties struct {
	SandboxId  string   `json:"SandboxId"`
	UserTokens []string `json:"UserTokens"`
}

type xstsResult struct {
	IssueInstant  time.Time `json:"IssueInstant"`
	NotAfter      time.Time `json:"NotAfter"`
	Token         string    `json:"Token"`
	DisplayClaims struct {
		Xui []struct {
			Uhs string `json:"uhs"`
		} `json:"xui"`
	} `json:"DisplayClaims"`
	Identity string `json:"Identity,omitempty"`
	XErr     int    `json:"XErr,omitempty"`
	Message  string `json:"Message,omitempty"`
	Redirect string `json:"Redirect,omitempty"`
}

type MinecraftAccountAuthRequest struct {
	IdentityToken string `json:"identityToken"`
}

type MinecraftAccountAuthResult struct {
	UserName    uuid.UUID `json:"username"`
	Roles       []string  `json:"roles"`
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int       `json:"expires_in"`
}

func (m *MinecraftAccountAuthResult) GetMinecraftProfile() (*MinecraftProfile, error) {
	httpClient := http.DefaultClient
	req, err := http.NewRequest(http.MethodGet, "https://api.minecraftservices.com/minecraft/profile", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.AccessToken))
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var mcProfile MinecraftProfile
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to authenticate 1: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&mcProfile); err != nil {
		return nil, err
	}
	slog.Info("mcProfile", "mcProfile", mcProfile)
	if mcProfile.Username == "" {
		return nil, fmt.Errorf("failed to get mcProfile: %s", resp.Status)
	}
	return &mcProfile, nil
}

func NewMinecraftAccount(auth string, expiresIn time.Time) (*MinecraftAccount, error) {
	if auth == "" {
		return nil, fmt.Errorf("auth is empty")
	}
	if expiresIn.IsZero() {
		return nil, fmt.Errorf("expiresIn is zero")
	}
	if expiresIn.Before(time.Now()) {
		return nil, fmt.Errorf("expiresIn is before now")
	}
	httpClient := http.DefaultClient

	req := xblAuthReqest{
		Properties: xblAuthProperties{
			AuthMethod: "RPS",
			SiteName:   "user.auth.xboxlive.com",
			RpsTicket:  fmt.Sprintf("d=%s", auth),
		},
		RelyingParty: "http://auth.xboxlive.com",
		TokenType:    "JWT",
	}
	body1, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	req1, err := http.NewRequest(http.MethodPost, "https://user.auth.xboxlive.com/user/authenticate", bytes.NewReader(body1))
	if err != nil {
		return nil, err
	}
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Accept", "application/json")

	resp1, err := httpClient.Do(req1)
	if err != nil {
		return nil, err
	}
	defer resp1.Body.Close()

	var authResult xblAuthResult
	if resp1.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to authenticate 2: %s", resp1.Status)
	}
	if err := json.NewDecoder(resp1.Body).Decode(&authResult); err != nil {
		return nil, err
	}
	slog.Info("authResult", "authResult", authResult)

	return &MinecraftAccount{
		XBLToken: authResult.Token,
		// XSTSToken: xstsResult.Token,
		UserHash: authResult.DisplayClaims.Xui[0].Uhs,
	}, nil
}

type MCStoreEntitlements struct {
	Items     []MCStoreItem `json:"items"`
	Signature string        `json:"signature"`
	KeyId     string        `json:"keyId"`
}

type MCStoreItem struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
}

type MinecraftProfile struct {
	Username string    `json:"name"`
	UUID     uuid.UUID `json:"id"`
}

func (m *MinecraftAccount) GetMinecraftAccount() (*MinecraftAccountAuthResult, error) {
	httpClient := http.DefaultClient

	xstsReq := xstsRequest{
		Properties: xstsProperties{
			SandboxId:  "RETAIL",
			UserTokens: []string{m.XBLToken},
		},
		RelyingParty: "rp://api.minecraftservices.com/",
		TokenType:    "JWT",
	}
	body2, err := json.Marshal(xstsReq)
	if err != nil {
		return nil, err
	}
	req2, err := http.NewRequest(http.MethodPost, "https://xsts.auth.xboxlive.com/xsts/authorize", bytes.NewReader(body2))
	if err != nil {
		return nil, err
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json")

	resp2, err := httpClient.Do(req2)
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()
	var xstsResult xstsResult
	if resp2.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to authenticate 3: %s", resp2.Status)
	}
	if err := json.NewDecoder(resp2.Body).Decode(&xstsResult); err != nil {
		return nil, err
	}
	slog.Info("xstsResult", "xstsResult", xstsResult)
	if xstsResult.XErr != 0 {
		_ = browser.Open(xstsResult.Redirect)
		return nil, fmt.Errorf("failed to authenticate 4: %s", xstsResult.Message)
	}

	req := MinecraftAccountAuthRequest{
		IdentityToken: fmt.Sprintf("XBL3.0 x=%s;%s", m.UserHash, xstsResult.Token),
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	req1, err := http.NewRequest(http.MethodPost, "https://api.minecraftservices.com/authentication/login_with_xbox", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Accept", "application/json")

	resp1, err := httpClient.Do(req1)
	if err != nil {
		return nil, err
	}
	defer resp1.Body.Close()

	var authResult MinecraftAccountAuthResult
	if resp1.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to authenticate 5: %s", resp1.Status)
	}
	if err := json.NewDecoder(resp1.Body).Decode(&authResult); err != nil {
		return nil, err
	}
	slog.Info("authResult", "authResult", authResult)

	req2, err = http.NewRequest(http.MethodGet, "https://api.minecraftservices.com/entitlements/mcstore", nil)
	if err != nil {
		return nil, err
	}
	req2.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authResult.AccessToken))
	req2.Header.Set("Accept", "application/json")
	resp2, err = httpClient.Do(req2)
	if err != nil {
		return nil, err
	}
	defer resp2.Body.Close()
	var mcStoreEntitlements MCStoreEntitlements
	if resp2.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to authenticate 6: %s", resp2.Status)
	}
	if err := json.NewDecoder(resp2.Body).Decode(&mcStoreEntitlements); err != nil {
		return nil, err
	}
	slog.Info("mcStoreEntitlements", "mcStoreEntitlements", mcStoreEntitlements)
	if len(mcStoreEntitlements.Items) == 0 {
		return nil, fmt.Errorf("failed to get mcstore entitlements: %s (Must own Minecraft!!)", resp2.Status)
	}
	if _, err := jwt.Parse(mcStoreEntitlements.Items[0].Signature, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwt.ParseRSAPublicKeyFromPEM([]byte(MojangPublicKey))
	}); err != nil {
		return nil, fmt.Errorf("failed to parse mcstore entitlements: %s", err)
	}

	profile, err := authResult.GetMinecraftProfile()
	if err != nil {
		return nil, err
	}

	if m.Username == "" {
		m.Username = profile.Username
	}
	if m.UUID == uuid.Nil {
		m.UUID = profile.UUID
	}

	return &authResult, nil
}

const MojangPublicKey = `-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAtz7jy4jRH3psj5AbVS6W
NHjniqlr/f5JDly2M8OKGK81nPEq765tJuSILOWrC3KQRvHJIhf84+ekMGH7iGlO
4DPGDVb6hBGoMMBhCq2jkBjuJ7fVi3oOxy5EsA/IQqa69e55ugM+GJKUndLyHeNn
X6RzRzDT4tX/i68WJikwL8rR8Jq49aVJlIEFT6F+1rDQdU2qcpfT04CBYLM5gMxE
fWRl6u1PNQixz8vSOv8pA6hB2DU8Y08VvbK7X2ls+BiS3wqqj3nyVWqoxrwVKiXR
kIqIyIAedYDFSaIq5vbmnVtIonWQPeug4/0spLQoWnTUpXRZe2/+uAKN1RY9mmaB
pRFV/Osz3PDOoICGb5AZ0asLFf/qEvGJ+di6Ltt8/aaoBuVw+7fnTw2BhkhSq1S/
va6LxHZGXE9wsLj4CN8mZXHfwVD9QG0VNQTUgEGZ4ngf7+0u30p7mPt5sYy3H+Fm
sWXqFZn55pecmrgNLqtETPWMNpWc2fJu/qqnxE9o2tBGy/MqJiw3iLYxf7U+4le4
jM49AUKrO16bD1rdFwyVuNaTefObKjEMTX9gyVUF6o7oDEItp5NHxFm3CqnQRmch
HsMs+NxEnN4E9a8PDB23b4yjKOQ9VHDxBxuaZJU60GBCIOF9tslb7OAkheSJx5Xy
EYblHbogFGPRFU++NrSQRX0CAwEAAQ==
-----END PUBLIC KEY-----`
