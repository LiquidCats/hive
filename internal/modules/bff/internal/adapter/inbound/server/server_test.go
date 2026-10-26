package server_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/suite"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/inbound/server"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite"
	queries "github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/migrations"
	app "github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/usecase"
	"github.com/LiquidCats/hive/internal/modules/bff/tests/mocks"
	db "github.com/LiquidCats/libraries/db/v2/sqlite"
)

const (
	rpID       = "hive.test"
	origin     = "https://hive.test"
	sessionTTL = time.Hour
)

var b64 = base64.RawURLEncoding

type AuthSuite struct {
	suite.Suite

	router *echo.Echo
	clock  *fakeClock
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func TestAuthSuite(t *testing.T) {
	suite.Run(t, new(AuthSuite))
}

func (s *AuthSuite) SetupTest() {
	ctx := s.T().Context()

	// :memory: is per connection; Connect defaults to a single one.
	conn, err := db.Connect(ctx, db.WithDatabase(":memory:"))
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = conn.Close() })
	s.Require().NoError(db.MigrateUp(ctx, conn, migrations.Migrations))

	wn, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Hive",
		RPID:          rpID,
		RPOrigins:     []string{origin},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true},
			Registration: webauthn.TimeoutConfig{Enforce: true},
		},
	})
	s.Require().NoError(err)

	repo := sqlite.NewRepository(db.NewQueriesTxManager(db.NewTxManager(conn), queries.New(conn)), testCipher(s.T()))
	s.clock = &fakeClock{now: time.Now()}

	srv := server.NewServer(
		app.NewAuth(wn, repo, repo, repo, repo, s.clock, sessionTTL),
		app.NewUsers(repo),
		app.NewRepos(repo, mocks.NewMockGitRemote(s.T())),
		slog.New(slog.DiscardHandler),
		sessionTTL,
	)

	s.router = echo.New()
	s.router.Use(srv.RequireSession)
	server.RegisterHandlers(s.router, srv)
}

func (s *AuthSuite) TestRegisterThenLogin() {
	key, rec := s.register("Alice")
	s.Equal("active", registrationStatus(rec))
	s.NotEmpty(sessionCookie(rec))

	// Usernames are normalized, so a differently cased name is the same user.
	rec = s.post("/webauthn/register/start", map[string]string{"username": " ALICE "})
	s.Equal(http.StatusConflict, rec.Code)

	options := s.startLogin()
	assertion := key.assert(s.T(), options, flagUP|flagUV)

	rec = s.post("/webauthn/login/finish", assertion)
	s.Require().Equal(http.StatusOK, rec.Code, rec.Body.String())
	s.NotEmpty(sessionCookie(rec))

	// A challenge is single use.
	rec = s.post("/webauthn/login/finish", assertion)
	s.Equal(http.StatusBadRequest, rec.Code)

	// Sign counter must keep growing; a stale one means a cloned authenticator.
	key.counter = 0
	rec = s.login(key)
	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *AuthSuite) TestLoginWithUnknownPasskey() {
	stranger := &authenticator{id: []byte("unknown-credential"), userID: make([]byte, 16)}
	stranger.key, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	rec := s.login(stranger)
	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *AuthSuite) TestRegisterRejectsInvalidUsername() {
	// "аdmin" starts with a Cyrillic а.
	for _, name := range []string{"ab", "bob smith", "аdmin", "-bob", "bo\nb"} {
		rec := s.post("/webauthn/register/start", map[string]string{"username": name})
		s.Equal(http.StatusBadRequest, rec.Code, name)
	}
}

func (s *AuthSuite) TestUserVerificationRequired() {
	// A touch-only authenticator (UP without UV) cannot register...
	key, challenge := s.startRegistration("alice")
	rec := s.post("/webauthn/register/finish", key.attest(s.T(), challenge, flagUP|flagAT))
	s.Equal(http.StatusUnauthorized, rec.Code)

	// ...nor log in.
	key, _ = s.register("alice")
	rec = s.post("/webauthn/login/finish", key.assert(s.T(), s.startLogin(), flagUP))
	s.Equal(http.StatusUnauthorized, rec.Code)
	s.Empty(sessionCookie(rec))
}

func (s *AuthSuite) TestLaterUsersNeedApproval() {
	_, rec := s.register("alice")
	admin := sessionCookie(rec)

	bob, rec := s.register("bob")
	s.Equal("pending_approval", registrationStatus(rec))
	s.Empty(sessionCookie(rec))

	rec = s.login(bob)
	s.Equal(http.StatusForbidden, rec.Code)
	s.Empty(sessionCookie(rec))

	rec = s.do(http.MethodGet, "/users", admin)
	s.Require().Equal(http.StatusOK, rec.Code, rec.Body.String())

	var page struct {
		Users []struct {
			Username string `json:"username"`
			Disabled bool   `json:"disabled"`
		} `json:"users"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &page))
	users := page.Users
	s.Equal("alice", users[0].Username)
	s.False(users[0].Disabled)
	s.Equal("bob", users[1].Username)
	s.True(users[1].Disabled)

	rec = s.do(http.MethodPost, "/users/"+userID(bob)+"/enable", admin)
	s.Require().Equal(http.StatusNoContent, rec.Code, rec.Body.String())

	rec = s.login(bob)
	s.Require().Equal(http.StatusOK, rec.Code, rec.Body.String())
	s.NotEmpty(sessionCookie(rec))
}

func (s *AuthSuite) TestSession() {
	rec := s.do(http.MethodGet, "/auth/session", "")
	s.Equal(http.StatusUnauthorized, rec.Code)

	rec = s.do(http.MethodGet, "/auth/session", rand.Text())
	s.Equal(http.StatusUnauthorized, rec.Code)

	_, rec = s.register("Alice")
	cookie := sessionCookie(rec)

	rec = s.do(http.MethodGet, "/auth/session", cookie)
	s.Require().Equal(http.StatusOK, rec.Code, rec.Body.String())
	s.JSONEq(`"alice"`, mustJSON(s.T(), rec, "username"))

	s.clock.now = s.clock.now.Add(sessionTTL)
	rec = s.do(http.MethodGet, "/auth/session", cookie)
	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *AuthSuite) TestLogout() {
	_, rec := s.register("alice")
	cookie := sessionCookie(rec)

	rec = s.do(http.MethodPost, "/auth/logout", cookie)
	s.Require().Equal(http.StatusNoContent, rec.Code, rec.Body.String())

	cleared := rec.Result().Cookies()
	s.Require().Len(cleared, 1)
	s.Empty(cleared[0].Value)
	s.Negative(cleared[0].MaxAge)

	rec = s.do(http.MethodGet, "/auth/session", cookie)
	s.Equal(http.StatusUnauthorized, rec.Code)

	// Idempotent, and public so a stale cookie can always be cleared.
	rec = s.do(http.MethodPost, "/auth/logout", cookie)
	s.Equal(http.StatusNoContent, rec.Code)
	rec = s.do(http.MethodPost, "/auth/logout", "")
	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *AuthSuite) TestDisableRevokesSessions() {
	alice, rec := s.register("alice")
	admin := sessionCookie(rec)

	bob, _ := s.register("bob")
	rec = s.do(http.MethodPost, "/users/"+userID(bob)+"/enable", admin)
	s.Require().Equal(http.StatusNoContent, rec.Code, rec.Body.String())

	rec = s.login(bob)
	s.Require().Equal(http.StatusOK, rec.Code, rec.Body.String())
	bobCookie := sessionCookie(rec)

	rec = s.do(http.MethodPost, "/users/"+userID(bob)+"/disable", admin)
	s.Require().Equal(http.StatusNoContent, rec.Code, rec.Body.String())

	rec = s.do(http.MethodGet, "/auth/session", bobCookie)
	s.Equal(http.StatusUnauthorized, rec.Code)

	rec = s.login(bob)
	s.Equal(http.StatusForbidden, rec.Code)

	rec = s.do(http.MethodPost, "/users/"+userID(alice)+"/disable", admin)
	s.Equal(http.StatusBadRequest, rec.Code)

	rec = s.do(http.MethodPost, "/users/"+uuid.NewV7().String()+"/enable", admin)
	s.Equal(http.StatusNotFound, rec.Code)

	rec = s.do(http.MethodPost, "/users/not-a-uuid/disable", admin)
	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *AuthSuite) TestRoutesRequireSession() {
	for _, path := range []string{"/users", "/auth/session"} {
		rec := s.do(http.MethodGet, path, "")
		s.Equal(http.StatusUnauthorized, rec.Code, path)
	}

	rec := s.do(http.MethodPost, "/users/"+uuid.NewV7().String()+"/enable", "")
	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *AuthSuite) TestMalformedBodies() {
	for _, path := range []string{"/webauthn/register/start", "/webauthn/register/finish", "/webauthn/login/finish"} {
		req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, path, bytes.NewBufferString("{"))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		s.router.ServeHTTP(rec, req)

		s.Equal(http.StatusBadRequest, rec.Code, path)
		s.JSONEq(`"bad_request"`, mustJSON(s.T(), rec, "code"), path)
	}
}

// register signs up with a user-verifying passkey; the response says whether the user is active.
func (s *AuthSuite) register(username string) (*authenticator, *httptest.ResponseRecorder) {
	key, challenge := s.startRegistration(username)

	rec := s.post("/webauthn/register/finish", key.attest(s.T(), challenge, flagUP|flagUV|flagAT))
	s.Require().Equal(http.StatusOK, rec.Code, rec.Body.String())

	return key, rec
}

func (s *AuthSuite) login(key *authenticator) *httptest.ResponseRecorder {
	return s.post("/webauthn/login/finish", key.assert(s.T(), s.startLogin(), flagUP|flagUV))
}

// startRegistration returns a new passkey for the user and the challenge it must attest.
func (s *AuthSuite) startRegistration(username string) (*authenticator, string) {
	rec := s.post("/webauthn/register/start", map[string]string{"username": username})
	s.Require().Equal(http.StatusOK, rec.Code, rec.Body.String())

	var options struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
			User      struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"publicKey"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &options))

	key := &authenticator{id: make([]byte, 16)}
	_, _ = rand.Read(key.id)
	key.userID, _ = b64.DecodeString(options.PublicKey.User.ID)
	key.key, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	return key, options.PublicKey.Challenge
}

func (s *AuthSuite) startLogin() string {
	rec := s.post("/webauthn/login/start", nil)
	s.Require().Equal(http.StatusOK, rec.Code, rec.Body.String())

	var options struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
		} `json:"publicKey"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &options))

	return options.PublicKey.Challenge
}

func (s *AuthSuite) post(path string, body any) *httptest.ResponseRecorder {
	payload, err := json.Marshal(body)
	s.Require().NoError(err)

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	return rec
}

// do sends a bodyless request, with the session cookie if set.
func (s *AuthSuite) do(method, path, cookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(s.T().Context(), method, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "session", Value: cookie})
	}

	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	return rec
}

func registrationStatus(rec *httptest.ResponseRecorder) string {
	var body struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)

	return body.Status
}

func mustJSON(t *testing.T, rec *httptest.ResponseRecorder, field string) string {
	t.Helper()

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	return string(body[field])
}

func userID(key *authenticator) string {
	return uuid.UUID(key.userID).String()
}

func sessionCookie(rec *httptest.ResponseRecorder) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			return c.Value
		}
	}

	return ""
}

// authenticator is a software passkey: "none" attestation, ES256 key.
type authenticator struct {
	id      []byte
	userID  []byte
	key     *ecdsa.PrivateKey
	counter uint32
}

const (
	flagUP = 0x01
	flagUV = 0x04
	flagAT = 0x40
)

func (a *authenticator) authData(flags byte) []byte {
	rpHash := sha256.Sum256([]byte(rpID))
	data := append(rpHash[:], flags)

	return binary.BigEndian.AppendUint32(data, a.counter)
}

func clientData(t *testing.T, typ, challenge string) []byte {
	t.Helper()

	data, err := json.Marshal(map[string]string{"type": typ, "challenge": challenge, "origin": origin})
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func (a *authenticator) attest(t *testing.T, challenge string, flags byte) map[string]any {
	t.Helper()

	point, err := a.key.PublicKey.Bytes() // 0x04 || X || Y
	if err != nil {
		t.Fatal(err)
	}

	cose, err := cbor.Marshal(map[int]any{
		1:  2,  // kty: EC2
		3:  -7, // alg: ES256
		-1: 1,  // crv: P-256
		-2: point[1:33],
		-3: point[33:],
	})
	if err != nil {
		t.Fatal(err)
	}

	data := a.authData(flags)
	data = append(data, make([]byte, 16)...) // AAGUID
	data = binary.BigEndian.AppendUint16(data, uint16(len(a.id)))
	data = append(data, a.id...)
	data = append(data, cose...)

	attestation, err := cbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": data})
	if err != nil {
		t.Fatal(err)
	}

	return map[string]any{
		"id":    b64.EncodeToString(a.id),
		"rawId": b64.EncodeToString(a.id),
		"type":  "public-key",
		"response": map[string]string{
			"clientDataJSON":    b64.EncodeToString(clientData(t, "webauthn.create", challenge)),
			"attestationObject": b64.EncodeToString(attestation),
		},
	}
}

func (a *authenticator) assert(t *testing.T, challenge string, flags byte) map[string]any {
	t.Helper()

	a.counter++
	data := a.authData(flags)
	client := clientData(t, "webauthn.get", challenge)
	clientHash := sha256.Sum256(client)
	digest := sha256.Sum256(append(bytes.Clone(data), clientHash[:]...))

	sig, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}

	return map[string]any{
		"id":    b64.EncodeToString(a.id),
		"rawId": b64.EncodeToString(a.id),
		"type":  "public-key",
		"response": map[string]string{
			"clientDataJSON":    b64.EncodeToString(client),
			"authenticatorData": b64.EncodeToString(data),
			"signature":         b64.EncodeToString(sig),
			"userHandle":        b64.EncodeToString(a.userID),
		},
	}
}
