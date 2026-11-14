package middleware

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Exercise the production SDK with a signed JWT and stub only its network
// boundaries. Emulator verification also checks revocation without opting in.
func TestProductionRetainedToken(t *testing.T) {
	t.Setenv("SAILOR_ENV", "production")
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	certTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	cert, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}))
	privateKey := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	credentials, err := json.Marshal(map[string]string{
		"type": "service_account", "project_id": "sailor-club", "private_key": privateKey,
		"client_email": "runtime@sailor-club.iam.gserviceaccount.com", "token_uri": "https://oauth2.googleapis.com/token",
	})
	if err != nil {
		t.Fatal(err)
	}
	credentialsPath := filepath.Join(t.TempDir(), "synthetic-adc.json")
	if err := os.WriteFile(credentialsPath, credentials, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credentialsPath)
	encode := func(value any) string {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	unsigned := encode(map[string]string{"alg": "RS256", "kid": "local-test"}) + "." + encode(map[string]any{
		"aud": "sailor-club", "iss": "https://securetoken.google.com/sailor-club", "sub": "test-user",
		"iat": now, "exp": now + 3600, "auth_time": now,
		"email": "sailor@example.com", "name": "Sailor", "email_verified": true,
	})
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	token := unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
	var disabled, revoked, unavailable, timeout atomic.Bool
	var lookups atomic.Int32
	oldTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("read RPC: %v", err)
			return
		}
		var payload any
		switch {
		case r.Host == "www.googleapis.com" && strings.Contains(r.URL.Path, "securetoken"):
			payload = map[string]string{"local-test": certPEM}
		case r.Host == "oauth2.googleapis.com":
			payload = map[string]any{"access_token": "synthetic-access-token", "expires_in": 3600, "token_type": "Bearer"}
		case r.Host == "identitytoolkit.googleapis.com" && strings.HasSuffix(r.URL.Path, "/accounts:lookup"):
			lookups.Add(1)
			if timeout.Load() {
				<-r.Context().Done()
				return
			}
			if unavailable.Load() {
				w.WriteHeader(http.StatusForbidden)
				_, _ = io.WriteString(w, `{"error":{"message":"PERMISSION_DENIED"}}`)
				return
			}
			validSince := now - 60
			if revoked.Load() {
				validSince = now + 1
			}
			payload = map[string]any{"users": []any{map[string]any{"localId": "test-user", "disabled": disabled.Load(), "validSince": strconv.FormatInt(validSince, 10)}}}
		default:
			t.Errorf("unexpected outbound request: %s %s", r.Host, r.URL)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=3600")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Errorf("encode RPC: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.ServerName = server.Certificate().DNSNames[0]
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	http.DefaultTransport = transport
	t.Cleanup(transport.CloseIdleConnections)
	client, err := auth.NewFirebaseAuth(context.Background(), "sailor-club")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.VerifyIDTokenAndCheckRevoked(context.Background(), token); err != nil {
		t.Fatalf("initial signed verification: %v", err)
	}
	for _, role := range []string{"member", "admin"} {
		t.Run(role, func(t *testing.T) {
			q := lookupOnlyQuerier{row: sqlcdb.GetAuthUserByFirebaseUIDRow{User: sqlcdb.User{ID: 7, Email: "sailor@example.com", Name: "Sailor", Role: role}, HasPerson: true, AdminExists: true}}
			handler := Auth(client, q)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
			request := func(want int) {
				t.Helper()
				r := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody)
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				if w.Code != want {
					t.Fatalf("status=%d body=%s want %d", w.Code, w.Body, want)
				}
			}
			disabled.Store(false)
			revoked.Store(false)
			unavailable.Store(false)
			request(http.StatusNoContent)
			disabled.Store(true)
			request(http.StatusUnauthorized)
			disabled.Store(false)
			revoked.Store(true)
			request(http.StatusUnauthorized)
			revoked.Store(false)
			unavailable.Store(true)
			request(http.StatusServiceUnavailable)
			unavailable.Store(false)
			request(http.StatusNoContent)
		})
	}
	if lookups.Load() < 11 {
		t.Fatalf("lookups=%d; every verification must query Firebase", lookups.Load())
	}
	timeout.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if got, err := client.VerifyIDTokenAndCheckRevoked(ctx, token); got != nil || err == nil || fbauth.IsIDTokenInvalid(err) {
		t.Fatalf("provider timeout = (%v, %v), want service failure", got, err)
	}
	if got, err := client.VerifyIDTokenAndCheckRevoked(context.Background(), "malformed"); got != nil || !fbauth.IsIDTokenInvalid(err) {
		t.Fatalf("malformed token = (%v, %v)", got, err)
	}
}

type failingVerifier struct{ t *testing.T }

func (f failingVerifier) VerifyIDTokenAndCheckRevoked(ctx context.Context, _ string) (*fbauth.Token, error) {
	f.t.Helper()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 5*time.Second {
		f.t.Fatal("missing verification deadline")
	}
	return nil, context.DeadlineExceeded
}

func TestAuthProviderFailure(t *testing.T) {
	for _, path := range []string{"/api/auth/me", "/api/enroll/secret"} {
		t.Run(path, func(t *testing.T) {
			called := false
			handler := Auth(failingVerifier{t}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if GetUser(r.Context()) != nil {
					t.Error("provider outage granted authenticated claims")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			r := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			r.Header.Set("Authorization", "Bearer retained-token")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if path == "/api/auth/me" && (w.Code != http.StatusServiceUnavailable || called) {
				t.Fatalf("private request = %d called=%v", w.Code, called)
			}
			if path == "/api/enroll/secret" && (w.Code != http.StatusNoContent || !called) {
				t.Fatalf("anonymous preview = %d called=%v", w.Code, called)
			}
		})
	}
}

func TestFirebaseProductionEmulatorRejected(t *testing.T) {
	t.Setenv("SAILOR_ENV", "production")
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "localhost:9099")
	if client, err := auth.NewFirebaseAuth(context.Background(), "sailor-club"); client != nil || err == nil {
		t.Fatal("production accepted Firebase emulator")
	}
}
