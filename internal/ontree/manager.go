package ontree

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ontree-co/treeos/internal/config"
	"github.com/ontree-co/treeos/internal/database"
	"github.com/ontree-co/treeos/internal/logging"
	"github.com/ontree-co/treeos/internal/templates"
	"github.com/ontree-co/treeos/pkg/compose"
	"golang.org/x/crypto/bcrypt"
)

// Manager coordinates core operations for CLI and API consumers.
type Manager struct {
	cfg         *config.Config
	db          *sql.DB
	templateSvc *templates.Service
	composeSvc  *compose.Service
	execCommand execCommandFunc
	timeNow     func() time.Time
}

type execCommandFunc func(ctx context.Context, name string, args ...string) commandRunner

type commandRunner interface {
	Output() ([]byte, error)
	CombinedOutput() ([]byte, error)
	Start() error
	Wait() error
	StdoutPipe() (readCloser, error)
	StderrPipe() (readCloser, error)
}

type readCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}

// NewManager initializes a new Manager.
func NewManager(cfg *config.Config) (*Manager, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		cfg:         cfg,
		db:          db,
		templateSvc: templates.NewService("."),
		timeNow:     time.Now,
	}
	manager.execCommand = manager.defaultExecCommand

	return manager, nil
}

// Close releases database resources.
func (m *Manager) Close() {
	if err := database.Close(); err != nil {
		logging.Warnf("Warning: failed to close database: %v", err)
	}
}

// SetupInit creates the initial admin user and marks setup complete.
func (m *Manager) SetupInit(ctx context.Context, username, password, nodeName, nodeIcon string) error {
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}

	userCount, err := m.countUsers(ctx)
	if err != nil {
		return err
	}
	if userCount > 0 {
		return fmt.Errorf("setup already completed")
	}

	if nodeName == "" {
		nodeName = "TreeOS Node"
	}
	if nodeIcon == "" {
		nodeIcon = "logo.png"
	}

	if _, err := m.createUser(ctx, username, password, "", true, true); err != nil {
		return err
	}

	if err := m.upsertSystemSetup(ctx, nodeName, nodeIcon); err != nil {
		return err
	}

	return nil
}

// SetupStatus reports the current setup state.
func (m *Manager) SetupStatus(ctx context.Context) (SetupStatus, error) {
	row := m.db.QueryRowContext(ctx, `
		SELECT is_setup_complete, node_name, node_icon
		FROM system_setup
		WHERE id = 1
	`)

	var completeInt int
	var nodeName, nodeIcon sql.NullString
	if err := row.Scan(&completeInt, &nodeName, &nodeIcon); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SetupStatus{Complete: false}, nil
		}
		return SetupStatus{}, err
	}

	return SetupStatus{
		Complete: completeInt == 1,
		NodeName: nodeName.String,
		NodeIcon: nodeIcon.String,
	}, nil
}

func (m *Manager) countUsers(ctx context.Context) (int, error) {
	var count int
	if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (m *Manager) createUser(ctx context.Context, username, password, email string, isStaff, isSuperuser bool) (int64, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("failed to hash password: %w", err)
	}

	now := m.timeNow()
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO users (username, password, email, is_staff, is_superuser, is_active, date_joined)
		VALUES (?, ?, ?, ?, ?, 1, ?)
	`, username, string(hashedPassword), email, isStaff, isSuperuser, now)
	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get user ID: %w", err)
	}

	return id, nil
}

func (m *Manager) upsertSystemSetup(ctx context.Context, nodeName, nodeIcon string) error {
	if _, err := m.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO system_setup (id, is_setup_complete)
		VALUES (1, 0)
	`); err != nil {
		return fmt.Errorf("failed to ensure system setup row: %w", err)
	}

	_, err := m.db.ExecContext(ctx, `
		UPDATE system_setup
		SET is_setup_complete = 1, setup_date = ?, node_name = ?, node_icon = ?
		WHERE id = 1
	`, m.timeNow(), nodeName, nodeIcon)
	if err != nil {
		return fmt.Errorf("failed to update system setup: %w", err)
	}

	return nil
}
