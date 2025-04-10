package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/js402/CATE/internal/serverops/store"
	"github.com/js402/CATE/libs/libdb"
	"github.com/stretchr/testify/require"
)

// verifyUserEquality is a helper to check that two User instances match.
func verifyUserEquality(t *testing.T, expected, actual *store.User) {
	t.Helper()
	require.Equal(t, expected.ID, actual.ID)
	require.Equal(t, expected.Email, actual.Email)
	require.Equal(t, expected.Subject, actual.Subject)
	require.Equal(t, expected.FriendlyName, actual.FriendlyName)
	require.Equal(t, expected.HashedPassword, actual.HashedPassword)
	require.Equal(t, expected.RecoveryCodeHash, actual.RecoveryCodeHash)
	require.Equal(t, expected.Salt, actual.Salt)
	require.WithinDuration(t, expected.CreatedAt, actual.CreatedAt, time.Second)
	require.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
}

// TestCreateUserAndRetrieve creates a user and then retrieves it via ID, Email, and Subject.
func TestCreateUserAndRetrieve(t *testing.T) {
	ctx, s := store.SetupStore(t)

	user := &store.User{
		ID:               uuid.NewString(),
		Email:            "test@example.com",
		Subject:          "subj123",
		FriendlyName:     "Test User",
		HashedPassword:   "hash",
		RecoveryCodeHash: "recovery",
		Salt:             uuid.NewString(),
	}

	// Create the user.
	err := s.CreateUser(ctx, user)
	require.NoError(t, err)
	require.NotZero(t, user.CreatedAt)
	require.NotZero(t, user.UpdatedAt)

	// Retrieve by ID.
	retrievedByID, err := s.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	verifyUserEquality(t, user, retrievedByID)

	// Retrieve by Email.
	retrievedByEmail, err := s.GetUserByEmail(ctx, user.Email)
	require.NoError(t, err)
	verifyUserEquality(t, user, retrievedByEmail)

	// Retrieve by Subject.
	retrievedBySubject, err := s.GetUserBySubject(ctx, user.Subject)
	require.NoError(t, err)
	verifyUserEquality(t, user, retrievedBySubject)
}

// TestCreateUserDuplicateEmail ensures that trying to create a user with a duplicate email fails.
func TestCreateUserDuplicateEmail(t *testing.T) {
	ctx, s := store.SetupStore(t)

	user1 := &store.User{
		ID:      uuid.NewString(),
		Email:   "dupe@test.com",
		Subject: "subj1",
		Salt:    uuid.NewString(),
	}
	require.NoError(t, s.CreateUser(ctx, user1))

	// Create a second user with the same email.
	user2 := &store.User{
		ID:      uuid.NewString(),
		Email:   "dupe@test.com",
		Subject: "subj2",
		Salt:    uuid.NewString(),
	}
	err := s.CreateUser(ctx, user2)
	require.Error(t, err)
	// Optionally, you can check for a specific duplicate key error substring.
	require.ErrorIs(t, err, libdb.ErrUniqueViolation, err)
}

// TestCreateUserDuplicateSubject ensures that trying to create a user with a duplicate subject fails.
func TestCreateUserDuplicateSubject(t *testing.T) {
	ctx, s := store.SetupStore(t)

	user1 := &store.User{
		ID:      uuid.NewString(),
		Email:   "test1@test.com",
		Subject: "dupe-subj",
		Salt:    uuid.NewString(),
	}
	require.NoError(t, s.CreateUser(ctx, user1))

	// Create a second user with the same subject.
	user2 := &store.User{
		ID:      uuid.NewString(),
		Email:   "test2@test.com",
		Subject: "dupe-subj",
		Salt:    uuid.NewString(),
	}
	err := s.CreateUser(ctx, user2)
	require.Error(t, err)
}

// TestGetUserNotFound attempts to retrieve non-existent users.
func TestGetUserNotFound(t *testing.T) {
	ctx, s := store.SetupStore(t)

	_, err := s.GetUserByID(ctx, uuid.NewString())
	require.ErrorIs(t, err, libdb.ErrNotFound)

	_, err = s.GetUserByEmail(ctx, "missing@test.com")
	require.ErrorIs(t, err, libdb.ErrNotFound)

	_, err = s.GetUserBySubject(ctx, "missing-subj")
	require.ErrorIs(t, err, libdb.ErrNotFound)
}

// TestUpdateUser updates an existing user and verifies the changes.
func TestUpdateUser(t *testing.T) {
	ctx, s := store.SetupStore(t)

	original := &store.User{
		ID:      uuid.NewString(),
		Email:   "original@test.com",
		Subject: "original-subj",
		Salt:    uuid.NewString(),
	}
	require.NoError(t, s.CreateUser(ctx, original))

	// Update user fields.
	original.FriendlyName = "Updated Name"
	original.Email = "updated@test.com"
	original.Subject = "updated-subj"
	original.HashedPassword = "new-hash"
	original.RecoveryCodeHash = "new-recovery"

	require.NoError(t, s.UpdateUser(ctx, original))

	// Retrieve the user and verify the update.
	updated, err := s.GetUserByID(ctx, original.ID)
	require.NoError(t, err)
	require.Equal(t, original.FriendlyName, updated.FriendlyName)
	require.Equal(t, original.Email, updated.Email)
	require.Equal(t, original.Subject, updated.Subject)
	require.Equal(t, original.HashedPassword, updated.HashedPassword)
	require.Equal(t, original.RecoveryCodeHash, updated.RecoveryCodeHash)
	// Verify UpdatedAt timestamp is newer.
	require.True(t, updated.UpdatedAt.After(updated.CreatedAt))
}

// TestUpdateUserConflict tests that updates conflicting with existing unique constraints are rejected.
func TestUpdateUserConflict(t *testing.T) {
	ctx, s := store.SetupStore(t)

	user1 := &store.User{
		ID:      uuid.NewString(),
		Email:   "user1@test.com",
		Subject: "subj1",
		Salt:    uuid.NewString(),
	}
	user2 := &store.User{
		ID:      uuid.NewString(),
		Email:   "user2@test.com",
		Subject: "subj2",
		Salt:    uuid.NewString(),
	}
	require.NoError(t, s.CreateUser(ctx, user1))
	require.NoError(t, s.CreateUser(ctx, user2))

	// Attempt to update user2 to have the same email as user1.
	user2.Email = user1.Email
	err := s.UpdateUser(ctx, user2)
	require.Error(t, err)

	// Reset email and then try to conflict on subject.
	user2.Email = "unique@test.com"
	user2.Subject = user1.Subject
	err = s.UpdateUser(ctx, user2)
	require.Error(t, err)
}

// TestDeleteUser deletes a user and then confirms that retrieval fails.
func TestDeleteUser(t *testing.T) {
	ctx, s := store.SetupStore(t)

	user := &store.User{
		ID:      uuid.NewString(),
		Email:   "delete@test.com",
		Subject: "delete-subj",
		Salt:    uuid.NewString(),
	}
	require.NoError(t, s.CreateUser(ctx, user))

	// Delete the user.
	require.NoError(t, s.DeleteUser(ctx, user.ID))

	// Verify the user cannot be retrieved.
	_, err := s.GetUserByID(ctx, user.ID)
	require.ErrorIs(t, err, libdb.ErrNotFound)
}

// TestDeleteUserNotFound ensures that trying to delete a non-existent user returns ErrNotFound.
func TestDeleteUserNotFound(t *testing.T) {
	ctx, s := store.SetupStore(t)
	err := s.DeleteUser(ctx, uuid.NewString())
	require.ErrorIs(t, err, libdb.ErrNotFound)
}

// TestListUsersOrder creates multiple users and checks that ListUsers returns them in descending creation order.
func TestListUsersOrder(t *testing.T) {
	ctx, s := store.SetupStore(t)
	beforeCreation := time.Now().UTC()

	// Create several users with a slight delay between each.
	users := []*store.User{
		{ID: uuid.NewString(), Email: "first@test.com", Subject: "subj1", Salt: uuid.NewString()},
		{ID: uuid.NewString(), Email: "second@test.com", Subject: "subj2", Salt: uuid.NewString()},
		{ID: uuid.NewString(), Email: "third@test.com", Subject: "subj3", Salt: uuid.NewString()},
	}

	for _, u := range users {
		require.NoError(t, s.CreateUser(ctx, u))
		time.Sleep(10 * time.Millisecond)
	}

	retrieved, err := s.ListUsers(ctx, time.Now().UTC())
	require.NoError(t, err)
	require.Len(t, retrieved, len(users))
	// Verify that the most recently created user appears first.
	require.Equal(t, users[2].Email, retrieved[0].Email)
	require.Equal(t, users[1].Email, retrieved[1].Email)
	require.Equal(t, users[0].Email, retrieved[2].Email)

	retrieved, err = s.ListUsers(ctx, beforeCreation)
	require.NoError(t, err)
	require.Len(t, retrieved, 0)
}

func createTestUser(t *testing.T, ctx context.Context, s store.Store, subject, friendlyName string) *store.User {
	t.Helper()
	user := &store.User{
		ID:           uuid.NewString(),
		Email:        uuid.NewString() + "@test.com", // Ensure unique email
		Subject:      subject,
		FriendlyName: friendlyName,
		Salt:         uuid.NewString(),
	}
	err := s.CreateUser(ctx, user)
	require.NoError(t, err)
	createdUser, err := s.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	return createdUser
}

func TestListUsersBySubjects(t *testing.T) {
	ctx, s := store.SetupStore(t)

	// --- Setup: Create test users ---
	userA := createTestUser(t, ctx, s, "subj-a", "User A") // Oldest
	userB := createTestUser(t, ctx, s, "subj-b", "User B")
	userD := createTestUser(t, ctx, s, "subj-d", "User D") // Newest

	// --- Test Cases ---

	t.Run("Fetch specific existing subjects", func(t *testing.T) {
		subjectsToFetch := []string{"subj-b", "subj-d"}
		retrieved, err := s.ListUsersBySubjects(ctx, subjectsToFetch...)

		require.NoError(t, err)
		require.Len(t, retrieved, 2)

		// Results should be ordered by CreatedAt DESC (D then B)
		// Check IDs to confirm correct users and order
		require.Equal(t, userD.ID, retrieved[0].ID, "First item should be user D")
		require.Equal(t, userB.ID, retrieved[1].ID, "Second item should be user B")
	})

	t.Run("Fetch subject with multiple users", func(t *testing.T) {
		subjectsToFetch := []string{"subj-a"}
		retrieved, err := s.ListUsersBySubjects(ctx, subjectsToFetch...)

		require.NoError(t, err)
		require.Len(t, retrieved, 1)

		// Results should be ordered by CreatedAt DESC (C then A)
		require.Equal(t, userA.ID, retrieved[0].ID, "First item should be user A")
	})

	t.Run("Fetch mixture of subjects", func(t *testing.T) {
		subjectsToFetch := []string{"subj-a", "subj-d"} // A, C, D match
		retrieved, err := s.ListUsersBySubjects(ctx, subjectsToFetch...)

		require.NoError(t, err)
		require.Len(t, retrieved, 2)

		// Results ordered by CreatedAt DESC: D, C, A
		require.Equal(t, userD.ID, retrieved[0].ID, "First item should be user D")
		require.Equal(t, userA.ID, retrieved[1].ID, "Second item should be user A")
	})

	t.Run("Fetch non-existent subject", func(t *testing.T) {
		subjectsToFetch := []string{"non-existent"}
		retrieved, err := s.ListUsersBySubjects(ctx, subjectsToFetch...)

		require.NoError(t, err)
		require.Empty(t, retrieved, "Result should be empty for non-existent subject")
	})

	t.Run("Fetch with empty subject list", func(t *testing.T) {
		// subjects... parameter becomes an empty slice if no args passed
		retrieved, err := s.ListUsersBySubjects(ctx)

		require.NoError(t, err)
		require.Empty(t, retrieved, "Result should be empty when no subjects are passed")
	})
}
