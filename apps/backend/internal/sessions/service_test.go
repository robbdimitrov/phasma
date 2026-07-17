package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	"phasma/backend/internal/auth"
)

type fakeRepository struct {
	credentials          *UserCredentials
	err                  error
	createdSession       CreatedSession
	createdSessionID     string
	createdSessionUser   int
	createdSessionExpiry time.Time
	deletedSessionID     string
	listedSessions       []Session
	listUserID           string
	listCurrentToken     string
	deleteByIDPublicID   string
	deleteByIDUserID     string
	deleteByIDToken      string
	deleteByIDOutcome    DeleteSessionOutcome
	deleteByIDErr        error
	updatedPasswordHash  string
	updatePasswordErr    error
	expiredSessionSweeps int
}

func (r *fakeRepository) DeleteExpiredSessions(context.Context) error {
	r.expiredSessionSweeps++
	return r.err
}

func (r *fakeRepository) FindLoginCredentialsByEmail(context.Context, string) (*UserCredentials, error) {
	return r.credentials, r.err
}

func (r *fakeRepository) CreateSession(
	_ context.Context,
	sessionID string,
	userID int,
	expiresAt time.Time,
) (CreatedSession, error) {
	r.createdSessionID = sessionID
	r.createdSessionUser = userID
	r.createdSessionExpiry = expiresAt
	if r.createdSession.UserID == 0 {
		return CreatedSession{UserID: userID}, r.err
	}
	return r.createdSession, r.err
}

func (r *fakeRepository) DeleteSession(_ context.Context, sessionID string) error {
	r.deletedSessionID = sessionID
	return r.err
}

func (r *fakeRepository) ListActiveSessions(_ context.Context, userID, currentSessionToken string) ([]Session, error) {
	r.listUserID = userID
	r.listCurrentToken = currentSessionToken
	return r.listedSessions, r.err
}

func (r *fakeRepository) DeleteSessionByID(
	_ context.Context,
	publicID, userID, currentSessionToken string,
) (DeleteSessionOutcome, error) {
	r.deleteByIDPublicID = publicID
	r.deleteByIDUserID = userID
	r.deleteByIDToken = currentSessionToken
	return r.deleteByIDOutcome, r.deleteByIDErr
}

func (r *fakeRepository) UpdatePasswordHash(_ context.Context, _ int, hash string) error {
	r.updatedPasswordHash = hash
	return r.updatePasswordErr
}

type fakeThrottle struct {
	failures     []LoginFailure
	recordedKeys []string
	clearedKeys  []string
	err          error
}

func (t *fakeThrottle) GetFailures(_ context.Context, _ []string) ([]LoginFailure, error) {
	return t.failures, t.err
}

func (t *fakeThrottle) RecordFailure(_ context.Context, key string) error {
	t.recordedKeys = append(t.recordedKeys, key)
	return nil
}

func (t *fakeThrottle) Clear(_ context.Context, keys []string) error {
	t.clearedKeys = append([]string(nil), keys...)
	return nil
}

func TestServiceLoginRateLimitedByIP(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	throttle := &fakeThrottle{failures: []LoginFailure{{
		Key:     "ip:192.0.2.1",
		Count:   ipLoginFailures,
		ResetAt: now.Add(time.Minute),
	}}}
	service := newTestService(&fakeRepository{}, throttle, now)

	_, err := service.Login(context.Background(), LoginInput{
		Email:    "test@example.com",
		Password: "password123",
		ClientIP: "192.0.2.1",
	})

	if !errors.Is(err, ErrLoginRateLimited) {
		t.Fatalf("Login() error = %v, want ErrLoginRateLimited", err)
	}
	if len(throttle.recordedKeys) != 0 {
		t.Fatal("rate-limited login should not record another failure")
	}
}

func TestServiceLoginEmailKeyUsesHigherThreshold(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	throttle := &fakeThrottle{failures: []LoginFailure{{
		Key:     "email:test@example.com",
		Count:   ipLoginFailures,
		ResetAt: now.Add(time.Minute),
	}}}
	service := newTestService(&fakeRepository{}, throttle, now)
	service.verifyPassword = func(context.Context, string, string) (bool, error) { return false, nil }

	_, err := service.Login(context.Background(), LoginInput{
		Email:    "test@example.com",
		Password: "password123",
		ClientIP: "192.0.2.1",
	})

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestServiceLoginInvalidCredentialsRecordsBothKeys(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	throttle := &fakeThrottle{}
	service := newTestService(&fakeRepository{}, throttle, now)

	_, err := service.Login(context.Background(), LoginInput{
		Email:    "test@example.com",
		Password: "password123",
		ClientIP: "192.0.2.1",
	})

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
	if len(throttle.recordedKeys) != 2 {
		t.Fatalf("recorded keys = %v, want IP and email keys", throttle.recordedKeys)
	}
	if throttle.recordedKeys[0] != "ip:192.0.2.1" ||
		throttle.recordedKeys[1] != "email:test@example.com" {
		t.Fatalf("recorded keys = %v", throttle.recordedKeys)
	}
}

func TestServiceLoginVerifiesDecoyHashForUnknownAccount(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	service := newTestService(&fakeRepository{}, &fakeThrottle{}, now)
	called := false
	var gotHash string
	service.verifyPassword = func(_ context.Context, _, hash string) (bool, error) {
		called = true
		gotHash = hash
		return false, nil
	}

	_, err := service.Login(context.Background(), LoginInput{
		Email:    "nobody@example.com",
		Password: "password123",
		ClientIP: "192.0.2.1",
	})

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
	if !called {
		t.Fatal("password verification must run even for a missing account (timing oracle)")
	}
	if gotHash != service.decoyHash {
		t.Fatalf("verified hash = %q, want decoy hash", gotHash)
	}
}

func TestServiceLoginSuccessCreatesSessionAndClearsFailures(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		credentials: &UserCredentials{ID: 7, PasswordHash: "encoded-hash"},
	}
	throttle := &fakeThrottle{}
	service := newTestService(repository, throttle, now)
	service.verifyPassword = func(_ context.Context, password, hash string) (bool, error) {
		return password == "password123" && hash == "encoded-hash", nil
	}

	output, err := service.Login(context.Background(), LoginInput{
		Email:    "test@example.com",
		Password: "password123",
		ClientIP: "192.0.2.1",
	})

	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if output != (LoginOutput{SessionID: "AAAAAAAAAAAAAAAAAAAAAAAAAAAA", UserID: 7}) {
		t.Fatalf("Login() output = %+v", output)
	}
	if repository.createdSessionID != output.SessionID {
		t.Fatalf("created session ID = %q", repository.createdSessionID)
	}
	if repository.createdSessionUser != 7 {
		t.Fatalf("created session user ID = %d, want 7", repository.createdSessionUser)
	}
	if want := now.Add(sessionDuration); !repository.createdSessionExpiry.Equal(want) {
		t.Fatalf("session expiry = %v, want %v", repository.createdSessionExpiry, want)
	}
	if len(throttle.clearedKeys) != 2 {
		t.Fatalf("cleared keys = %v, want IP and email keys", throttle.clearedKeys)
	}
	if repository.expiredSessionSweeps != 0 {
		t.Fatalf("expired session sweeps = %d, want 0", repository.expiredSessionSweeps)
	}
}

func TestServiceLoginPasswordVerificationErrorIsInvalidCredentials(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		credentials: &UserCredentials{ID: 7, PasswordHash: "malformed"},
	}
	throttle := &fakeThrottle{}
	service := newTestService(repository, throttle, now)
	service.verifyPassword = func(context.Context, string, string) (bool, error) {
		return false, errors.New("invalid password hash")
	}

	_, err := service.Login(context.Background(), LoginInput{
		Email:    "test@example.com",
		Password: "password123",
		ClientIP: "192.0.2.1",
	})

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
	if len(throttle.recordedKeys) != 2 {
		t.Fatalf("recorded keys = %v, want IP and email keys", throttle.recordedKeys)
	}
}

func TestServiceLoginThrottleDownFailsOpen(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		credentials: &UserCredentials{ID: 5, PasswordHash: "encoded-hash"},
	}
	throttle := &fakeThrottle{err: errors.New("cache down")}
	service := newTestService(repository, throttle, now)
	service.verifyPassword = func(context.Context, string, string) (bool, error) { return true, nil }

	_, err := service.Login(context.Background(), LoginInput{
		Email:    "test@example.com",
		Password: "password123",
		ClientIP: "192.0.2.1",
	})

	if err != nil {
		t.Fatalf("Login() error = %v, want nil (throttle down must fail open)", err)
	}
}

func TestServiceLogoutDeletesSession(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository, NoopLoginThrottle{})

	err := service.Logout(context.Background(), "AAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if repository.deletedSessionID != "AAAAAAAAAAAAAAAAAAAAAAAAAAAA" {
		t.Fatalf("deleted session ID = %q", repository.deletedSessionID)
	}
}

func TestServiceListActiveSessions(t *testing.T) {
	want := []Session{{ID: "01904d2e-7f4d-7c33-ae21-2f94737eaa10", Current: true}}
	repository := &fakeRepository{listedSessions: want}
	service := NewService(repository, NoopLoginThrottle{})

	got, err := service.ListActive(context.Background(), "7", "current-token")

	if err != nil {
		t.Fatalf("ListActive() error = %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("ListActive() = %+v, want %+v", got, want)
	}
	if repository.listUserID != "7" || repository.listCurrentToken != "current-token" {
		t.Fatalf("repository list args = %q, %q", repository.listUserID, repository.listCurrentToken)
	}
}

func TestServiceListActiveSessionsRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	service := NewService(&fakeRepository{err: wantErr}, NoopLoginThrottle{})

	_, err := service.ListActive(context.Background(), "7", "current-token")

	if !errors.Is(err, wantErr) {
		t.Fatalf("ListActive() error = %v, want %v", err, wantErr)
	}
}

func TestServiceRevokeSessionOutcomes(t *testing.T) {
	tests := []struct {
		name    string
		outcome DeleteSessionOutcome
		repoErr error
		wantErr error
	}{
		{name: "deleted", outcome: DeleteSessionDeleted},
		{name: "missing", outcome: DeleteSessionNotFound, wantErr: ErrSessionNotFound},
		{name: "current", outcome: DeleteSessionCurrent, wantErr: ErrCurrentSession},
		{name: "repository error", repoErr: errors.New("database unavailable"), wantErr: errors.New("database unavailable")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeRepository{
				deleteByIDOutcome: test.outcome,
				deleteByIDErr:     test.repoErr,
			}
			service := NewService(repository, NoopLoginThrottle{})

			err := service.Revoke(
				context.Background(),
				"01904d2e-7f4d-7c33-ae21-2f94737eaa10",
				"7",
				"current-token",
			)

			if test.wantErr == nil && err != nil {
				t.Fatalf("Revoke() error = %v", err)
			}
			if test.wantErr != nil && (err == nil || err.Error() != test.wantErr.Error()) {
				t.Fatalf("Revoke() error = %v, want %v", err, test.wantErr)
			}
			if repository.deleteByIDPublicID != "01904d2e-7f4d-7c33-ae21-2f94737eaa10" ||
				repository.deleteByIDUserID != "7" ||
				repository.deleteByIDToken != "current-token" {
				t.Fatalf("repository revoke args = %q, %q, %q",
					repository.deleteByIDPublicID,
					repository.deleteByIDUserID,
					repository.deleteByIDToken,
				)
			}
		})
	}
}

func TestServiceLoginRehashesWeakHash(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	// A hash with params below DefaultPasswordParams triggers a rehash.
	weakHash := "$argon2id$v=19$m=4096,t=1,p=1$c29tZXNhbHQ$dGVzdGhhc2g"
	repository := &fakeRepository{
		credentials: &UserCredentials{ID: 3, PasswordHash: weakHash},
	}
	service := newTestService(repository, &fakeThrottle{}, now)
	service.verifyPassword = func(context.Context, string, string) (bool, error) { return true, nil }
	service.hashPassword = func(_ context.Context, password string, _ auth.PasswordParams) (string, error) {
		return "new-strong-hash", nil
	}

	_, err := service.Login(context.Background(), LoginInput{
		Email: "test@example.com", Password: "p", ClientIP: "192.0.2.1",
	})

	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if repository.updatedPasswordHash != "new-strong-hash" {
		t.Fatalf("UpdatePasswordHash not called or wrong hash: %q", repository.updatedPasswordHash)
	}
}

func TestServiceLoginSkipsRehashForCurrentParams(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		credentials: &UserCredentials{ID: 3, PasswordHash: currentParamsHash},
	}
	service := newTestService(repository, &fakeThrottle{}, now)
	service.verifyPassword = func(context.Context, string, string) (bool, error) { return true, nil }
	rehashCalled := false
	service.hashPassword = func(context.Context, string, auth.PasswordParams) (string, error) {
		rehashCalled = true
		return "", nil
	}

	_, err := service.Login(context.Background(), LoginInput{
		Email: "test@example.com", Password: "p", ClientIP: "192.0.2.1",
	})

	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if rehashCalled {
		t.Fatal("hashPassword should not be called for an at-target hash")
	}
}

func TestServiceLoginPersistErrorDoesNotFailLogin(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		credentials:       &UserCredentials{ID: 3, PasswordHash: "invalid-hash"},
		updatePasswordErr: errors.New("db down"),
	}
	service := newTestService(repository, &fakeThrottle{}, now)
	service.verifyPassword = func(context.Context, string, string) (bool, error) { return true, nil }
	service.hashPassword = func(context.Context, string, auth.PasswordParams) (string, error) {
		return "new-hash", nil
	}

	_, err := service.Login(context.Background(), LoginInput{
		Email: "test@example.com", Password: "p", ClientIP: "192.0.2.1",
	})

	if err != nil {
		t.Fatalf("Login() error = %v, want nil (persist failure must not fail login)", err)
	}
}

// currentParamsHash is a valid Argon2id PHC hash at DefaultPasswordParams.
// Used to verify that an at-target hash is not rehashed.
const currentParamsHash = "$argon2id$v=19$m=19456,t=2,p=1$c29tZXNhbHQxMjM0NTY3OA$hbwqGankDMxbQ5fN303ASdxPeikPPqvxOYodhOKwtOY"

func newTestService(repository Repository, throttle LoginThrottle, now time.Time) *Service {
	service := NewService(repository, throttle)
	service.now = func() time.Time { return now }
	service.generateSessionID = func() (string, error) {
		return "AAAAAAAAAAAAAAAAAAAAAAAAAAAA", nil
	}
	// Use a no-op hasher by default so existing tests don't trigger slow rehashes.
	service.hashPassword = func(context.Context, string, auth.PasswordParams) (string, error) {
		return "rehashed", nil
	}
	return service
}

var (
	_ Repository    = (*fakeRepository)(nil)
	_ LoginThrottle = (*fakeThrottle)(nil)
)
