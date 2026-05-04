package integration

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	postgresmig "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	dbmigrations "marbl/db/migrations"
	"marbl/internal/consumer"
	"marbl/internal/persistence"
	"marbl/internal/persistence/db"
)

func skipWithoutIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("MARBL_INTEGRATION") != "1" {
		t.Skip("set MARBL_INTEGRATION=1 and Docker to run integration tests")
	}
}

func applyMigrations(t *testing.T, connStr string) {
	t.Helper()
	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	driver, err := postgresmig.WithInstance(sqlDB, &postgresmig.Config{})
	if err != nil {
		t.Fatal(err)
	}
	src, err := iofs.New(dbmigrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
}

func startPostgres(t *testing.T) (context.Context, string, func()) {
	t.Helper()
	skipWithoutIntegration(t)
	ctx := context.Background()
	c, err := postgres.Run(ctx, "docker.io/postgres:16-alpine",
		postgres.WithDatabase("marbl"),
		postgres.WithUsername("marbl"),
		postgres.WithPassword("marbl"),
	)
	if err != nil {
		t.Fatalf("postgres container: %v", err)
	}
	connStr, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = c.Terminate(ctx)
		t.Fatal(err)
	}
	applyMigrations(t, connStr)
	cleanup := func() {
		_ = c.Terminate(context.Background())
	}
	return ctx, connStr, cleanup
}

func TestIntegrationTaskLifecycle(t *testing.T) {
	ctx, connStr, cleanup := startPostgres(t)
	defer cleanup()

	pool, err := persistence.NewPool(ctx, connStr)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	q := db.New(pool)
	task, err := q.CreateTask(ctx, db.CreateTaskParams{Type: 2, Value: 7})
	if err != nil {
		t.Fatal(err)
	}
	n, err := q.TryMarkProcessing(ctx, task.ID)
	if err != nil || n != 1 {
		t.Fatalf("try mark: n=%d err=%v", n, err)
	}
	dn, err := q.MarkDone(ctx, task.ID)
	if err != nil || dn != 1 {
		t.Fatalf("mark done: n=%d err=%v", dn, err)
	}
	sum, err := q.SumValueDoneByType(ctx, 2)
	if err != nil || sum != 7 {
		t.Fatalf("sum: %d err=%v", sum, err)
	}
}

func TestIntegrationReaperResetsStaleProcessing(t *testing.T) {
	ctx, connStr, cleanup := startPostgres(t)
	defer cleanup()

	pool, err := persistence.NewPool(ctx, connStr)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	q := db.New(pool)
	task, err := q.CreateTask(ctx, db.CreateTaskParams{Type: 1, Value: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.TryMarkProcessing(ctx, task.ID); err != nil {
		t.Fatal(err)
	}
	old := float64(time.Now().Add(-2 * time.Hour).Unix())
	if _, err := pool.Exec(ctx, `UPDATE tasks SET last_update_time = $1 WHERE id = $2`, old, task.ID); err != nil {
		t.Fatal(err)
	}
	cutoff := consumer.StaleCutoff(time.Now(), 5)
	n, err := q.ReapStaleProcessing(ctx, cutoff)
	if err != nil || n != 1 {
		t.Fatalf("reap: n=%d err=%v", n, err)
	}
	got, err := q.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != db.TaskStateReceived {
		t.Fatalf("state=%s want received", got.State)
	}
}

func TestIntegrationMigrateDownRemovesComment(t *testing.T) {
	ctx, connStr, cleanup := startPostgres(t)
	defer cleanup()

	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	driver, err := postgresmig.WithInstance(sqlDB, &postgresmig.Config{})
	if err != nil {
		t.Fatal(err)
	}
	src, err := iofs.New(dbmigrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = m.Close() }()
	// One step down from v2: remove comment migration only (matches `migrate down` CLI).
	if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate down: %v", err)
	}
	var colExists bool
	if err := sqlDB.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM information_schema.columns
  WHERE table_name = 'tasks' AND column_name = 'comment'
)`).Scan(&colExists); err != nil {
		t.Fatal(err)
	}
	if colExists {
		t.Fatal("expected comment column removed after down migration")
	}
}
