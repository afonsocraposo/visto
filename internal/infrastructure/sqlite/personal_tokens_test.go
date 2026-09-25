package sqlite_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/afonsocosta/visto/internal/application/auth"
	"github.com/afonsocosta/visto/internal/infrastructure/sqlite"
)

func TestPersonalAPITokens_GivenTwoUsers_WhenTokensAreUsedAndRevoked_ThenSecretsAndOwnershipAreProtected(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "visto.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	accounts := auth.NewService(store)
	owner, err := accounts.Bootstrap(ctx, "owner@example.com", "Owner", "a-long-admin-password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := accounts.CreateUser(ctx, "other@example.com", "Other", "a-long-user-password")
	if err != nil {
		t.Fatal(err)
	}

	issued, err := accounts.CreatePersonalToken(ctx, owner.ID, "home dashboard", nil)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := accounts.PersonalTokens(ctx, owner.ID)
	if err != nil || len(listed) != 1 || listed[0].ID != issued.ID {
		t.Fatalf("owner tokens=%+v error=%v", listed, err)
	}
	otherTokens, err := accounts.PersonalTokens(ctx, other.ID)
	if err != nil || len(otherTokens) != 0 {
		t.Fatalf("other user's tokens=%+v error=%v", otherTokens, err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(issued.Token)))
	user, err := store.FindUserByPersonalTokenHash(ctx, hash, time.Now().UTC())
	if err != nil || user.ID != owner.ID {
		t.Fatalf("token owner=%+v error=%v", user, err)
	}
	if err := accounts.RevokePersonalToken(ctx, other.ID, issued.ID); err != auth.ErrPersonalTokenMissing {
		t.Fatalf("cross-user revoke error=%v, want not found", err)
	}
	if _, err := accounts.AuthenticatePersonalToken(ctx, issued.Token); err != nil {
		t.Fatalf("other user revoked the token: %v", err)
	}
	if err := accounts.RevokePersonalToken(ctx, owner.ID, issued.ID); err != nil {
		t.Fatalf("owner revoke: %v", err)
	}
	if _, err := accounts.AuthenticatePersonalToken(ctx, issued.Token); err != auth.ErrInvalidCredentials {
		t.Fatalf("authentication after revoke error=%v, want invalid credentials", err)
	}
}
