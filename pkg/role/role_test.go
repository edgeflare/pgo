package role_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/edgeflare/pgo/pkg/role"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxtest"
	"github.com/stretchr/testify/require"
)

var (
	defaultConnTestRunner pgxtest.ConnTestRunner
	testRoleNamePrefix    = "test_role"
)

func init() {
	defaultConnTestRunner = pgxtest.DefaultConnTestRunner()
	defaultConnTestRunner.CreateConfig = func(ctx context.Context, t testing.TB) *pgx.ConnConfig {
		config, err := pgx.ParseConfig(os.Getenv("TEST_DATABASE"))
		require.NoError(t, err)
		config.OnNotice = func(_ *pgconn.PgConn, n *pgconn.Notice) {
			t.Logf("PostgreSQL %s: %s", n.Severity, n.Message)
		}
		return config
	}
}

func connectPG(t testing.TB, ctx context.Context) *pgx.Conn {
	config, err := pgx.ParseConfig(fmt.Sprintf(os.Getenv("TEST_DATABASE")))
	require.NoError(t, err)
	config.OnNotice = func(_ *pgconn.PgConn, n *pgconn.Notice) {
		t.Logf("PostgreSQL %s: %s", n.Severity, n.Message)
	}

	conn, err := pgx.ConnectConfig(ctx, config)
	require.NoError(t, err)
	return conn
}

func closeConn(t testing.TB, conn *pgx.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, conn.Close(ctx))
}

func TestList(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		roles, err := role.List(ctx, conn)
		require.NoError(t, err, "Failed to list roles")
		require.NotEmpty(t, roles, "No roles were listed from the database")
	})
}

func TestCreate(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		newRole := role.Role{
			Name:        testRoleNamePrefix + "_create",
			Superuser:   false,
			Inherit:     true,
			CreateRole:  false,
			CreateDB:    false,
			CanLogin:    true,
			Replication: false,
			ConnLimit:   -1,
			Password:    "secure_password",
			Config:      []string{"search_path=public"},
		}
		err := role.Create(ctx, conn, newRole)
		require.NoError(t, err, "Failed to create role")

		// Verify that the role was created
		fetchedRole, err := role.Get(ctx, conn, newRole.Name)
		require.NoError(t, err, "Failed to get the created role")
		require.Equal(t, newRole.Name, fetchedRole.Name, "Fetched role name does not match")
	})
}

func TestUpdate(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		// First create a role to update
		initialRole := role.Role{
			Name:        testRoleNamePrefix + "_update",
			Superuser:   false,
			Inherit:     true,
			CreateRole:  false,
			CreateDB:    false,
			CanLogin:    true,
			Replication: false,
			ConnLimit:   -1,
		}
		err := role.Create(ctx, conn, initialRole)
		require.NoError(t, err, "Failed to create initial role")

		// Update the role
		updatedRole := initialRole
		updatedRole.Superuser = true
		updatedRole.Password = "new_secure_password"

		err = role.Update(ctx, conn, updatedRole)
		require.NoError(t, err, "Failed to update role")

		// Verify that the role was updated
		fetchedRole, err := role.Get(ctx, conn, updatedRole.Name)
		require.NoError(t, err, "Failed to get the updated role")
		require.True(t, fetchedRole.Superuser, "Role Superuser status should be true")
	})
}

func TestGet(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		// Create a role to get
		testRole := role.Role{
			Name:        testRoleNamePrefix + "_get",
			Superuser:   false,
			Inherit:     true,
			CreateRole:  false,
			CreateDB:    false,
			CanLogin:    true,
			Replication: false,
			ConnLimit:   -1,
		}
		err := role.Create(ctx, conn, testRole)
		require.NoError(t, err, "Failed to create role for testing Get")

		// Retrieve the role
		fetchedRole, err := role.Get(ctx, conn, testRole.Name)
		require.NoError(t, err, "Failed to get role")
		require.Equal(t, testRole.Name, fetchedRole.Name, "Fetched role name does not match")
	})
}

func TestDelete(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *pgx.Conn) {
		// Create a role to delete
		roleToDelete := role.Role{
			Name:        testRoleNamePrefix + "_delete",
			Superuser:   false,
			Inherit:     true,
			CreateRole:  false,
			CreateDB:    false,
			CanLogin:    true,
			Replication: false,
			ConnLimit:   -1,
		}
		err := role.Create(ctx, conn, roleToDelete)
		require.NoError(t, err, "Failed to create role for deletion")

		// Delete the role
		err = role.Delete(ctx, conn, roleToDelete.Name)
		require.NoError(t, err, "Failed to delete role")

		// Verify that the role is deleted
		_, err = role.Get(ctx, conn, roleToDelete.Name)
		require.Error(t, err, "Expected error when getting deleted role")
		require.Equal(t, role.ErrRoleNotFound, err, "Error should be ErrRoleNotFound")
	})
}
