package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/domain"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestOnboarding_GivenFreshInstance_WhenAdminAndUserSignUp_ThenSignupIsGatedUntilAdminExists(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := auth.NewService(store)
	available, err := service.BootstrapAvailable(ctx)
	if err != nil || !available {
		t.Fatalf("bootstrap available = %v, %v", available, err)
	}
	if _, _, _, err := service.SignUp(ctx, "family@example.com", "Family", "a-long-family-password"); err != auth.ErrBootstrapIncomplete {
		t.Fatalf("signup before admin = %v, want ErrBootstrapIncomplete", err)
	}
	admin, err := service.Bootstrap(ctx, "owner@example.com", "Owner", "a-long-admin-password")
	if err != nil || admin.Role != domain.AdminRole {
		t.Fatalf("bootstrap = %+v, %v", admin, err)
	}
	available, err = service.BootstrapAvailable(ctx)
	if err != nil || available {
		t.Fatalf("bootstrap available after admin = %v, %v", available, err)
	}
	user, token, _, err := service.SignUp(ctx, "family@example.com", "Family", "a-long-family-password")
	if err != nil || user.Role != domain.UserRole || token == "" {
		t.Fatalf("signup = %+v, token present=%v, %v", user, token != "", err)
	}
}

func TestAdminUserManagement_GivenAdministrator_WhenManagingAccounts_ThenPasswordsAndDeletionAreSafe(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := auth.NewService(store)
	admin, err := service.Bootstrap(ctx, "owner@example.com", "Owner", "a-long-admin-password")
	if err != nil {
		t.Fatal(err)
	}
	_, currentSession, _, err := service.Login(ctx, "owner@example.com", "a-long-admin-password")
	if err != nil {
		t.Fatal(err)
	}
	_, oldSession, _, err := service.Login(ctx, "owner@example.com", "a-long-admin-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteUser(ctx, admin.ID); err != auth.ErrLastAdministrator {
		t.Fatalf("delete last admin = %v", err)
	}
	if err := service.UpdateUser(ctx, admin.ID, "Owner", "a-new-admin-password", currentSession); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, currentSession); err != nil {
		t.Fatalf("admin's active session was not preserved: %v", err)
	}
	if _, err := service.Authenticate(ctx, oldSession); err == nil {
		t.Fatal("password update did not revoke the other session")
	}
	if _, err := service.AuthenticateCredentials(ctx, "owner@example.com", "a-new-admin-password"); err != nil {
		t.Fatalf("authenticate with updated admin password: %v", err)
	}
	user, err := service.CreateUser(ctx, "family@example.com", "Family", "a-long-family-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateUser(ctx, user.ID, "Family Member", "a-new-family-password", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthenticateCredentials(ctx, "family@example.com", "a-new-family-password"); err != nil {
		t.Fatalf("authenticate with updated password: %v", err)
	}
	users, err := service.Users(ctx)
	if err != nil || len(users) != 2 {
		t.Fatalf("users = %+v, %v", users, err)
	}
	var updated bool
	for _, listed := range users {
		if listed.ID == user.ID && listed.DisplayName == "Family Member" {
			updated = true
		}
	}
	if !updated {
		t.Fatalf("updated account not present in user list: %+v", users)
	}
	if err := service.DeleteUser(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	users, err = service.Users(ctx)
	if err != nil || len(users) != 1 || users[0].ID != admin.ID {
		t.Fatalf("users after delete = %+v, %v", users, err)
	}
}

func TestGoogleAccounts_GivenConfiguredProvider_WhenUserSignsIn_ThenItCreatesAndReusesAnEmailAccount(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := auth.NewService(store, auth.Config{AllowSignups: true, GoogleEnabled: true})
	if _, err := service.Bootstrap(ctx, "owner@example.com", "Owner", "a-long-admin-password"); err != nil {
		t.Fatal(err)
	}
	localUser, err := service.CreateUser(ctx, "person@example.com", "Local Person", "a-long-local-password")
	if err != nil {
		t.Fatal(err)
	}
	first, firstToken, _, err := service.LoginWithGoogle(ctx, "google-sub-1", " Person@Example.com ", "Google Person")
	if err != nil {
		t.Fatalf("first Google sign-in: %v", err)
	}
	if first.ID != localUser.ID || first.Email != "person@example.com" || first.DisplayName != "Local Person" || first.Role != domain.UserRole || firstToken == "" {
		t.Fatalf("linked Google user = %+v", first)
	}
	second, secondToken, _, err := service.LoginWithGoogle(ctx, "google-sub-1", "person@example.com", "Changed Google Name")
	if err != nil {
		t.Fatalf("repeat Google sign-in: %v", err)
	}
	if second.ID != first.ID || second.DisplayName != "Local Person" || secondToken == "" {
		t.Fatalf("repeat sign-in user = %+v", second)
	}
	if _, _, _, err := service.LoginWithGoogle(ctx, "google-sub-2", "person@example.com", "Different Google Account"); err == nil {
		t.Fatal("a different Google account must not attach to an existing email automatically")
	}
	if _, err := service.AuthenticateCredentials(ctx, "person@example.com", "a-long-local-password"); err != nil {
		t.Fatalf("local password should still work after linking: %v", err)
	}
	created, _, _, err := service.LoginWithGoogle(ctx, "google-sub-3", "another@example.com", "Another Person")
	if err != nil || created.Email != "another@example.com" {
		t.Fatalf("new Google account = %+v, %v", created, err)
	}
}

func TestGoogleAccounts_GivenDisabledSignups_WhenNewGoogleAccountSignsIn_ThenItIsRejected(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	localService := auth.NewService(store)
	if _, err := localService.Bootstrap(ctx, "owner@example.com", "Owner", "a-long-admin-password"); err != nil {
		t.Fatal(err)
	}
	localUser, err := localService.CreateUser(ctx, "existing@example.com", "Existing User", "a-long-local-password")
	if err != nil {
		t.Fatal(err)
	}
	service := auth.NewService(store, auth.Config{AllowSignups: false, GoogleEnabled: true})
	if _, _, _, err := service.LoginWithGoogle(ctx, "google-sub-new", "new@example.com", "New User"); err != auth.ErrSignupsDisabled {
		t.Fatalf("new Google account error = %v, want ErrSignupsDisabled", err)
	}
	linked, _, _, err := service.LoginWithGoogle(ctx, "google-sub-existing", localUser.Email, localUser.DisplayName)
	if err != nil || linked.ID != localUser.ID {
		t.Fatalf("existing local account should link even when signups are disabled: %+v, %v", linked, err)
	}
}
