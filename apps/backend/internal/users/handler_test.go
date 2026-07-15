package users

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"phasma/backend/internal/httpx"
	"phasma/backend/internal/pagination"
	"phasma/backend/internal/store"
)

type fakeService struct {
	createID             int
	createErr            error
	createCommand        CreateUserCommand
	user                 User
	users                []User
	nextCursor           *pagination.Cursor
	listQuery            ListQuery
	found                bool
	getErr               error
	profileCommand       UpdateProfileCommand
	profileOutcome       UpdateProfileOutcome
	profileErr           error
	passwordCommand      ChangePasswordCommand
	passwordOutcome      ChangePasswordOutcome
	passwordErr          error
	followCommand        FollowCommand
	followErr            error
	unfollowCommand      FollowCommand
	unfollowErr          error
	getByUsernameCurrent string
	getByID              string
	suggestedUsers       []User
	suggestedErr         error
}

func (s *fakeService) CreateUser(_ context.Context, command CreateUserCommand) (int, error) {
	s.createCommand = command
	return s.createID, s.createErr
}

func (s *fakeService) GetUserByUsername(_ context.Context, _, currentUserID string) (User, bool, error) {
	s.getByUsernameCurrent = currentUserID
	return s.user, s.found, s.getErr
}

func (s *fakeService) GetUserByID(_ context.Context, userID, _ string) (User, bool, error) {
	s.getByID = userID
	return s.user, s.found, s.getErr
}

func (s *fakeService) ListFollowers(_ context.Context, query ListQuery) ([]User, *pagination.Cursor, error) {
	s.listQuery = query
	return s.users, s.nextCursor, s.getErr
}

func (s *fakeService) ListFollowing(_ context.Context, query ListQuery) ([]User, *pagination.Cursor, error) {
	s.listQuery = query
	return s.users, s.nextCursor, s.getErr
}

func (s *fakeService) UpdateProfile(_ context.Context, command UpdateProfileCommand) (UpdateProfileOutcome, error) {
	s.profileCommand = command
	return s.profileOutcome, s.profileErr
}

func (s *fakeService) ChangePassword(_ context.Context, command ChangePasswordCommand) (ChangePasswordOutcome, error) {
	s.passwordCommand = command
	return s.passwordOutcome, s.passwordErr
}

func (s *fakeService) FollowUser(_ context.Context, command FollowCommand) error {
	s.followCommand = command
	return s.followErr
}

func (s *fakeService) UnfollowUser(_ context.Context, command FollowCommand) error {
	s.unfollowCommand = command
	return s.unfollowErr
}

func (s *fakeService) ListSuggestedUsers(_ context.Context, _ string) ([]User, error) {
	return s.suggestedUsers, s.suggestedErr
}

func TestCreateUserValidation(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		message string
	}{
		{"missing field", `{"name":"Test","username":"test","email":"test@example.com"}`, "Name, username, email and password are required."},
		{"invalid username", `{"name":"Test","username":"bad-name","email":"test@example.com","password":"password123"}`, "Username must be 3-30 characters"},
		{"short password", `{"name":"Test","username":"test","email":"test@example.com","password":"short"}`, "Password must be between 8 and 128 characters long."},
		{"long password", `{"name":"Test","username":"test","email":"test@example.com","password":"` + strings.Repeat("a", 129) + `"}`, "Password must be between 8 and 128 characters long."},
		{"invalid email", `{"name":"Test","username":"test","email":"invalid","password":"password123"}`, "Invalid email address."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeService{}
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(test.body))

			NewHandler(service).CreateUser(res, req)

			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
			}
			if !strings.Contains(res.Body.String(), test.message) {
				t.Fatalf("body = %q, want message %q", res.Body.String(), test.message)
			}
			if service.createCommand != (CreateUserCommand{}) {
				t.Fatal("invalid request reached service")
			}
		})
	}
}

func TestCreateUserNormalizesInput(t *testing.T) {
	service := &fakeService{createID: 12}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{
		"name":" Test User ",
		"username":" Test ",
		"email":" Test@Example.COM ",
		"password":" password123 "
	}`))

	NewHandler(service).CreateUser(res, req)

	if res.Code != http.StatusCreated || strings.TrimSpace(res.Body.String()) != `{"username":"test"}` {
		t.Fatalf("response = %d %q", res.Code, res.Body.String())
	}
	want := CreateUserCommand{
		Name: "Test User", Username: "test", Email: "test@example.com", Password: " password123 ",
	}
	if service.createCommand != want {
		t.Fatalf("command = %#v, want %#v", service.createCommand, want)
	}
}

func TestCreateUserErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"conflict", store.ErrConflict, http.StatusConflict, "User with this username or email already exists."},
		{"internal", errors.New("database unavailable"), http.StatusInternalServerError, "Could not create user. Please try again."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeService{createErr: test.err}
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(
				`{"name":"Test","username":"test","email":"test@example.com","password":"password123"}`,
			))

			NewHandler(service).CreateUser(res, req)

			if res.Code != test.status || !strings.Contains(res.Body.String(), test.message) {
				t.Fatalf("response = %d %q", res.Code, res.Body.String())
			}
		})
	}
}

func TestGetUserHidesEmailFromOtherUsers(t *testing.T) {
	service := &fakeService{
		found: true,
		user: User{
			ID: 2, Username: "test", Email: "private@example.com",
			Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	res := httptest.NewRecorder()
	req := httpx.WithUserID(httptest.NewRequest(http.MethodGet, "/users/test", nil), "1")
	req.SetPathValue("username", "TEST")

	NewHandler(service).GetUser(res, req)

	if res.Code != http.StatusOK || strings.Contains(res.Body.String(), "private@example.com") {
		t.Fatalf("response = %d %q", res.Code, res.Body.String())
	}
	if service.getByUsernameCurrent != "1" {
		t.Fatalf("current user ID = %q, want 1", service.getByUsernameCurrent)
	}
}

func TestGetUserNotFound(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/missing", nil)
	req.SetPathValue("username", "missing")

	NewHandler(&fakeService{}).GetUser(res, req)

	if res.Code != http.StatusNotFound || !strings.Contains(res.Body.String(), "Not Found") {
		t.Fatalf("response = %d %q", res.Code, res.Body.String())
	}
}

func TestGetCurrentUserUsesSessionIdentity(t *testing.T) {
	service := &fakeService{found: true, user: User{ID: 7, Email: "private@example.com"}}
	res := httptest.NewRecorder()
	req := httpx.WithUserID(httptest.NewRequest(http.MethodGet, "/users/me", nil), "7")

	NewHandler(service).GetCurrentUser(res, req)

	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"email":"private@example.com"`) {
		t.Fatalf("response = %d %q", res.Code, res.Body.String())
	}
	if service.getByID != "7" {
		t.Fatalf("user ID = %q, want 7", service.getByID)
	}
}

func TestListFollowersUsesPaginationAndHidesOtherUserEmail(t *testing.T) {
	cursor := pagination.Cursor{Created: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC), ID: 42}
	nextCursor := pagination.Cursor{Created: time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC), ID: 21}
	service := &fakeService{
		users: []User{
			{ID: 1, Username: "viewer", Email: "viewer@example.com", Created: cursor.Created},
			{ID: 2, Username: "other", Email: "other@example.com", Created: cursor.Created},
		},
		nextCursor: &nextCursor,
	}
	res := httptest.NewRecorder()
	req := httpx.WithUserID(httptest.NewRequest(http.MethodGet, "/users/test/followers?cursor="+pagination.EncodeCursor(cursor)+"&limit=500", nil), "1")
	req.SetPathValue("username", "TEST")

	NewHandler(service).ListFollowers(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", res.Code, res.Body.String())
	}
	if service.listQuery.Username != "test" || service.listQuery.CurrentUserID != "1" ||
		service.listQuery.Cursor == nil || *service.listQuery.Cursor != cursor || service.listQuery.Limit != 50 {
		t.Fatalf("query = %#v", service.listQuery)
	}
	if !strings.Contains(res.Body.String(), `"email":"viewer@example.com"`) ||
		strings.Contains(res.Body.String(), "other@example.com") {
		t.Fatalf("body = %q", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"nextCursor":"`+pagination.EncodeCursor(nextCursor)+`"`) {
		t.Fatalf("body missing next cursor: %q", res.Body.String())
	}
}

func TestListUsersErrors(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		err     error
		status  int
		message string
	}{
		{"invalid cursor", "/users/test/following?cursor=invalid", nil, http.StatusBadRequest, "Invalid pagination parameters."},
		{"missing user", "/users/test/following", store.ErrNotFound, http.StatusNotFound, "Not Found"},
		{"internal", "/users/test/following", errors.New("failed"), http.StatusInternalServerError, "Internal Server Error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeService{getErr: test.err}
			res := httptest.NewRecorder()
			req := httpx.WithUserID(httptest.NewRequest(http.MethodGet, test.url, nil), "1")
			req.SetPathValue("username", "test")

			NewHandler(service).ListFollowing(res, req)

			if res.Code != test.status || !strings.Contains(res.Body.String(), test.message) {
				t.Fatalf("response = %d %q", res.Code, res.Body.String())
			}
		})
	}
}

func TestUpdateUserRequiresSession(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/users/me", strings.NewReader(`{}`))

	NewHandler(&fakeService{}).UpdateUser(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestUpdateUserProfile(t *testing.T) {
	service := &fakeService{}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/users/me", strings.NewReader(`{
		"name":" Test User ","username":" Test ","email":" Test@Example.COM ","avatar":"avatar.jpg"
	}`))
	req = httpx.WithUserID(req, "1")

	NewHandler(service).UpdateUser(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("response = %d %q", res.Code, res.Body.String())
	}
	if service.profileCommand.Name != "Test User" ||
		service.profileCommand.Username != "test" ||
		service.profileCommand.Email != "test@example.com" ||
		service.profileCommand.Avatar != "avatar.jpg" {
		t.Fatalf("command = %#v", service.profileCommand)
	}
}

func TestUpdateUserProfileTrimsBio(t *testing.T) {
	service := &fakeService{}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/users/me", strings.NewReader(`{
		"name":"Test User","username":"test","email":"test@example.com","bio":"  hello there  "
	}`))
	req = httpx.WithUserID(req, "1")

	NewHandler(service).UpdateUser(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("response = %d %q", res.Code, res.Body.String())
	}
	if service.profileCommand.Bio == nil || *service.profileCommand.Bio != "hello there" {
		t.Fatalf("bio = %v, want \"hello there\"", service.profileCommand.Bio)
	}
}

func TestUpdateUserProfileOutcomes(t *testing.T) {
	tests := []struct {
		name    string
		outcome UpdateProfileOutcome
		err     error
		status  int
		message string
	}{
		{"invalid avatar", UpdateProfileInvalidAvatar, nil, http.StatusBadRequest, "Avatar upload is invalid or expired."},
		{"conflict", UpdateProfileUpdated, store.ErrConflict, http.StatusConflict, "This username or email is already in use."},
		{"internal", UpdateProfileUpdated, errors.New("failed"), http.StatusInternalServerError, "Internal Server Error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeService{profileOutcome: test.outcome, profileErr: test.err}
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/users/me", strings.NewReader(
				`{"name":"Test","username":"test","email":"test@example.com"}`,
			))
			req = httpx.WithUserID(req, "1")

			NewHandler(service).UpdateUser(res, req)

			if res.Code != test.status || !strings.Contains(res.Body.String(), test.message) {
				t.Fatalf("response = %d %q", res.Code, res.Body.String())
			}
		})
	}
}

func TestUpdatePasswordOutcomes(t *testing.T) {
	tests := []struct {
		name    string
		outcome ChangePasswordOutcome
		err     error
		status  int
		message string
	}{
		{"changed", ChangePasswordChanged, nil, http.StatusNoContent, ""},
		{"missing user", ChangePasswordUserNotFound, nil, http.StatusNotFound, "Not Found"},
		{"wrong password", ChangePasswordWrongPassword, nil, http.StatusBadRequest, "Wrong password"},
		{"internal", ChangePasswordChanged, errors.New("failed"), http.StatusInternalServerError, "Internal Server Error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeService{passwordOutcome: test.outcome, passwordErr: test.err}
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/users/me", strings.NewReader(
				`{"oldPassword":"old-password","password":"new-password"}`,
			))
			req.AddCookie(&http.Cookie{Name: "session", Value: "AAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
			req = httpx.WithUserID(req, "1")

			NewHandler(service).UpdateUser(res, req)

			if res.Code != test.status || !strings.Contains(res.Body.String(), test.message) {
				t.Fatalf("response = %d %q", res.Code, res.Body.String())
			}
			if test.err == nil && service.passwordCommand.CurrentSessionID != "AAAAAAAAAAAAAAAAAAAAAAAAAAAA" {
				t.Fatalf("session ID = %q", service.passwordCommand.CurrentSessionID)
			}
		})
	}
}

func TestFollowRoutes(t *testing.T) {
	service := &fakeService{}
	handler := NewHandler(service)

	followRes := httptest.NewRecorder()
	followReq := httpx.WithUserID(httptest.NewRequest(http.MethodPost, "/users/alice/follow", nil), "1")
	followReq.SetPathValue("username", "alice")
	handler.FollowUser(followRes, followReq)

	unfollowRes := httptest.NewRecorder()
	unfollowReq := httpx.WithUserID(httptest.NewRequest(http.MethodDelete, "/users/alice/follow", nil), "1")
	unfollowReq.SetPathValue("username", "alice")
	handler.UnfollowUser(unfollowRes, unfollowReq)

	want := FollowCommand{FollowerID: "1", FolloweeUsername: "alice"}
	if followRes.Code != http.StatusNoContent || unfollowRes.Code != http.StatusNoContent {
		t.Fatalf("statuses = %d, %d", followRes.Code, unfollowRes.Code)
	}
	if service.followCommand != want || service.unfollowCommand != want {
		t.Fatalf("commands = %#v, %#v", service.followCommand, service.unfollowCommand)
	}
}

func TestFollowUserErrors(t *testing.T) {
	tests := []struct {
		name    string
		current string
		target  string
		err     error
		status  int
		message string
	}{
		{"unauthorized", "", "alice", nil, http.StatusUnauthorized, "Unauthorized"},
		{"invalid username", "1", "bad-name!", nil, http.StatusBadRequest, "Invalid username."},
		{"self", "1", "alice", ErrSelfFollow, http.StatusBadRequest, "Cannot follow yourself."},
		{"not found", "1", "alice", store.ErrNotFound, http.StatusNotFound, "User Not Found"},
		{"internal", "1", "alice", errors.New("failed"), http.StatusInternalServerError, "Internal Server Error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeService{followErr: test.err}
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/users/"+test.target+"/follow", nil)
			req.SetPathValue("username", test.target)
			if test.current != "" {
				req = httpx.WithUserID(req, test.current)
			}

			NewHandler(service).FollowUser(res, req)

			if res.Code != test.status || !strings.Contains(res.Body.String(), test.message) {
				t.Fatalf("response = %d %q", res.Code, res.Body.String())
			}
		})
	}
}

func TestListSuggestedUsersReturnsItems(t *testing.T) {
	service := &fakeService{
		suggestedUsers: []User{
			{ID: 5, Username: "alice", Email: "alice@example.com"},
			{ID: 6, Username: "bob", Email: "bob@example.com"},
		},
	}
	res := httptest.NewRecorder()
	req := httpx.WithUserID(httptest.NewRequest(http.MethodGet, "/users/suggested", nil), "1")

	NewHandler(service).ListSuggestedUsers(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if !strings.Contains(res.Body.String(), `"items"`) {
		t.Fatalf("body = %q, want items key", res.Body.String())
	}
	// Emails must be stripped from suggested users.
	if strings.Contains(res.Body.String(), "alice@example.com") || strings.Contains(res.Body.String(), "bob@example.com") {
		t.Fatalf("body = %q, email must be hidden for suggested users", res.Body.String())
	}
}

func TestListSuggestedUsersRequiresSession(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/suggested", nil)

	NewHandler(&fakeService{}).ListSuggestedUsers(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestRegisterRoutes(t *testing.T) {
	service := &fakeService{createID: 1}
	handler := NewHandler(service)
	public := http.NewServeMux()
	protected := http.NewServeMux()
	RegisterPublicRoutes(public, handler)
	RegisterProtectedRoutes(protected, handler)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(
		`{"name":"Test","username":"test","email":"test@example.com","password":"password123"}`,
	))
	public.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("public route status = %d", res.Code)
	}

	res = httptest.NewRecorder()
	req = httpx.WithUserID(httptest.NewRequest(http.MethodPut, "/users/me", strings.NewReader(`{}`)), "1")
	protected.ServeHTTP(res, req)
	if res.Code == http.StatusNotFound {
		t.Fatalf("protected route status = %d, want route to be registered", res.Code)
	}
}
