package bmc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/openchami/magellan/pkg/secrets"
	"github.com/stretchr/testify/require"
)

type rotateTestStore struct {
	mu          sync.Mutex
	values      map[string]string
	writeErr    error
	writeCount  int
	failWriteAt int
}

func (s *rotateTestStore) GetSecretByID(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[id]
	if !ok {
		return "", fmt.Errorf("secret %q not found", id)
	}
	return value, nil
}

func (s *rotateTestStore) StoreSecretByID(id, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeCount++
	if s.writeErr != nil && (s.failWriteAt == 0 || s.writeCount == s.failWriteAt) {
		return s.writeErr
	}
	s.values[id] = value
	return nil
}

func (s *rotateTestStore) ListSecrets() (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := make(map[string]string, len(s.values))
	for key, value := range s.values {
		values[key] = value
	}
	return values, nil
}

func (s *rotateTestStore) RemoveSecretByID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, id)
	return nil
}

func (s *rotateTestStore) value(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[id]
}

type rotateRedfishFixture struct {
	mu                 sync.Mutex
	username           string
	password           string
	accountUsername    string
	patchStatus        int
	patchApplyThenFail bool
	rejectReplacement  bool
	passwordChanged    bool
	patchCount         int
}

func (f *rotateRedfishFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	f.mu.Lock()
	requiresAuth := r.URL.Path != "/redfish/v1/"
	validCredential := !requiresAuth || (ok && username == f.username && password == f.password)
	if requiresAuth && f.rejectReplacement && f.passwordChanged && password == f.password {
		validCredential = false
	}
	f.mu.Unlock()
	if !validCredential {
		w.Header().Set("WWW-Authenticate", `Basic realm="Redfish"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/redfish/v1/":
		_, _ = fmt.Fprint(w, `{"@odata.id":"/redfish/v1/","AccountService":{"@odata.id":"/redfish/v1/AccountService"}}`)
	case r.Method == http.MethodGet && r.URL.Path == "/redfish/v1/AccountService":
		_, _ = fmt.Fprint(w, `{"@odata.id":"/redfish/v1/AccountService","Accounts":{"@odata.id":"/redfish/v1/AccountService/Accounts"}}`)
	case r.Method == http.MethodGet && r.URL.Path == "/redfish/v1/AccountService/Accounts":
		_, _ = fmt.Fprint(w, `{"@odata.id":"/redfish/v1/AccountService/Accounts","Members":[{"@odata.id":"/redfish/v1/AccountService/Accounts/2"}]}`)
	case r.URL.Path == "/redfish/v1/AccountService/Accounts/2" && r.Method == http.MethodGet:
		f.mu.Lock()
		accountUsername := f.accountUsername
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"@odata.id": "/redfish/v1/AccountService/Accounts/2",
			"Id":        "2",
			"UserName":  accountUsername,
			"Enabled":   true,
		})
	case r.URL.Path == "/redfish/v1/AccountService/Accounts/2" && r.Method == http.MethodPatch:
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.patchCount++
		if f.patchStatus != 0 {
			if f.patchApplyThenFail {
				f.password = payload["Password"]
				f.passwordChanged = true
			}
			http.Error(w, "injected PATCH failure", f.patchStatus)
			return
		}
		f.password = payload["Password"]
		f.passwordChanged = true
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func newRotateRedfishFixture(t *testing.T) (*httptest.Server, *rotateRedfishFixture) {
	t.Helper()
	fixture := &rotateRedfishFixture{
		username:        "test_user",
		password:        "old-password",
		accountUsername: "test_user",
	}
	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)
	return server, fixture
}

func newRotateTestStore() *rotateTestStore {
	return &rotateTestStore{values: map[string]string{
		"":                 `{"username":"test_user","password":"old-password"}`,
		"http://127.0.0.1": `{"username":"test_user","password":"old-password"}`,
	}}
}

func TestRotateStoredAccountUpdatesBMCAndStore(t *testing.T) {
	server, fixture := newRotateRedfishFixture(t)
	store := newRotateTestStore()
	endpoint := server.URL
	store.values[endpoint] = store.values["http://127.0.0.1"]

	err := RotateStoredAccount(endpoint, "2", "new-password", store, false, "")
	require.NoError(t, err)
	require.Equal(t, "new-password", fixture.password)
	require.JSONEq(t, `{"username":"test_user","password":"new-password"}`, store.value(endpoint))
	require.Equal(t, 1, fixture.patchCount)
}

func TestRotateStoredAccountRejectsUsernameMismatchBeforePatch(t *testing.T) {
	server, fixture := newRotateRedfishFixture(t)
	fixture.accountUsername = "somebody-else"
	store := newRotateTestStore()
	store.values[server.URL] = `{"username":"test_user","password":"old-password"}`

	err := RotateStoredAccount(server.URL, "2", "new-password", store, false, "")
	require.ErrorContains(t, err, "account username does not match stored credential")
	require.Zero(t, fixture.patchCount)
	require.JSONEq(t, `{"username":"test_user","password":"old-password"}`, store.value(server.URL))
}

func TestRotateStoredAccountReportsPATCHFailureWithoutChangingStore(t *testing.T) {
	server, fixture := newRotateRedfishFixture(t)
	fixture.patchStatus = http.StatusInternalServerError
	store := newRotateTestStore()
	store.values[server.URL] = `{"username":"test_user","password":"old-password"}`

	err := RotateStoredAccount(server.URL, "2", "new-password", store, false, "")
	require.ErrorContains(t, err, "BMC may still have applied the change")
	require.ErrorContains(t, err, "Do not retry blindly")
	require.Equal(t, "old-password", fixture.password)
	require.JSONEq(t, `{"username":"test_user","password":"old-password","pending_rotation":{"account_id":"2","new_password":"new-password"}}`, store.value(server.URL))
}

func TestRotateStoredAccountDoesNotPatchWhenIntentWriteFails(t *testing.T) {
	server, fixture := newRotateRedfishFixture(t)
	store := newRotateTestStore()
	store.values[server.URL] = `{"username":"test_user","password":"old-password"}`
	store.writeErr = errors.New("injected intent write failure")

	err := RotateStoredAccount(server.URL, "2", "new-password", store, false, "")
	require.ErrorContains(t, err, "could not persist rotation intent")
	require.Equal(t, "old-password", fixture.password)
	require.Zero(t, fixture.patchCount)
	require.JSONEq(t, `{"username":"test_user","password":"old-password"}`, store.value(server.URL))
}

func TestRotateStoredAccountReportsVerificationFailureAndLeavesStoreUnchanged(t *testing.T) {
	server, fixture := newRotateRedfishFixture(t)
	fixture.rejectReplacement = true
	store := newRotateTestStore()
	store.values[server.URL] = `{"username":"test_user","password":"old-password"}`

	err := RotateStoredAccount(server.URL, "2", "new-password", store, false, "")
	require.ErrorContains(t, err, "new credential verification failed")
	require.Equal(t, "new-password", fixture.password)
	require.JSONEq(t, `{"username":"test_user","password":"old-password","pending_rotation":{"account_id":"2","new_password":"new-password"}}`, store.value(server.URL))
}

func TestRotateStoredAccountReportsStoreFailureAfterBMCChange(t *testing.T) {
	server, fixture := newRotateRedfishFixture(t)
	store := newRotateTestStore()
	store.values[server.URL] = `{"username":"test_user","password":"old-password"}`
	store.writeErr = errors.New("injected store failure")
	store.failWriteAt = 2

	err := RotateStoredAccount(server.URL, "2", "new-password", store, false, "")
	require.ErrorContains(t, err, "credential-store finalization failed")
	require.Equal(t, "new-password", fixture.password)
	require.JSONEq(t, `{"username":"test_user","password":"old-password","pending_rotation":{"account_id":"2","new_password":"new-password"}}`, store.value(server.URL))
	store.writeErr = nil
	require.NoError(t, RecoverStoredAccountRotation(server.URL, store, false, ""))
	require.JSONEq(t, `{"username":"test_user","password":"new-password"}`, store.value(server.URL))
}

func TestRecoverStoredAccountRotationCommitsWhenNewPasswordIsActive(t *testing.T) {
	server, fixture := newRotateRedfishFixture(t)
	fixture.password = "new-password"
	store := newRotateTestStore()
	store.values[server.URL] = `{"username":"test_user","password":"old-password","pending_rotation":{"account_id":"2","new_password":"new-password"}}`

	require.NoError(t, RecoverStoredAccountRotation(server.URL, store, false, ""))
	require.JSONEq(t, `{"username":"test_user","password":"new-password"}`, store.value(server.URL))
	require.Zero(t, fixture.patchCount)
}

func TestRotateStoredAccountRefusesToReplacePendingRotation(t *testing.T) {
	server, fixture := newRotateRedfishFixture(t)
	store := newRotateTestStore()
	store.values[server.URL] = `{"username":"test_user","password":"old-password","pending_rotation":{"account_id":"2","new_password":"pending-password"}}`

	err := RotateStoredAccount(server.URL, "2", "another-password", store, false, "")
	require.ErrorContains(t, err, "a rotation is already pending")
	require.Zero(t, fixture.patchCount)
	require.JSONEq(t, `{"username":"test_user","password":"old-password","pending_rotation":{"account_id":"2","new_password":"pending-password"}}`, store.value(server.URL))
}

func TestRecoverStoredAccountRotationRetriesWhenOldPasswordIsActive(t *testing.T) {
	server, fixture := newRotateRedfishFixture(t)
	store := newRotateTestStore()
	store.values[server.URL] = `{"username":"test_user","password":"old-password","pending_rotation":{"account_id":"2","new_password":"new-password"}}`

	require.NoError(t, RecoverStoredAccountRotation(server.URL, store, false, ""))
	require.Equal(t, "new-password", fixture.password)
	require.JSONEq(t, `{"username":"test_user","password":"new-password"}`, store.value(server.URL))
	require.Equal(t, 1, fixture.patchCount)
}

func TestRecoverStoredAccountRotationHandlesAppliedPatchTimeout(t *testing.T) {
	server, fixture := newRotateRedfishFixture(t)
	fixture.patchStatus = http.StatusInternalServerError
	fixture.patchApplyThenFail = true
	store := newRotateTestStore()
	store.values[server.URL] = `{"username":"test_user","password":"old-password"}`

	err := RotateStoredAccount(server.URL, "2", "new-password", store, false, "")
	require.ErrorContains(t, err, "BMC may still have applied the change")
	require.Equal(t, "new-password", fixture.password)
	require.NoError(t, RecoverStoredAccountRotation(server.URL, store, false, ""))
	require.JSONEq(t, `{"username":"test_user","password":"new-password"}`, store.value(server.URL))
	require.Equal(t, 1, fixture.patchCount)
}

func TestRecoverStoredAccountRotationRetainsPendingWhenNeitherPasswordWorks(t *testing.T) {
	server, fixture := newRotateRedfishFixture(t)
	fixture.password = "admin-reset-password"
	store := newRotateTestStore()
	store.values[server.URL] = `{"username":"test_user","password":"old-password","pending_rotation":{"account_id":"2","new_password":"new-password"}}`

	err := RecoverStoredAccountRotation(server.URL, store, false, "")
	require.ErrorContains(t, err, "neither previous nor pending password authenticates")
	require.JSONEq(t, `{"username":"test_user","password":"old-password","pending_rotation":{"account_id":"2","new_password":"new-password"}}`, store.value(server.URL))
	require.Zero(t, fixture.patchCount)
}

func TestRotateStoredAccountRequiresExactPersistentCredential(t *testing.T) {
	server, fixture := newRotateRedfishFixture(t)
	store := newRotateTestStore()
	delete(store.values, server.URL)
	store.values[secrets.DEFAULT_KEY] = `{"username":"test_user","password":"old-password"}`

	err := RotateStoredAccount(server.URL, "2", "new-password", store, false, "")
	require.ErrorContains(t, err, "no default fallback")
	require.Zero(t, fixture.patchCount)

	err = RotateStoredAccount(server.URL, "2", "new-password", secrets.NewStaticStore("test_user", "old-password"), false, "")
	require.ErrorContains(t, err, "persistent credential store")
}
