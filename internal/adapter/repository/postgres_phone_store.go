package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mobilefarm/af/phone-orchestrator/internal/domain"
)

type PostgresPhoneStore struct {
	pool *pgxpool.Pool
}

func NewPostgresPhoneStore(ctx context.Context, dsn string) (*PostgresPhoneStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("подключение к postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &PostgresPhoneStore{pool: pool}, nil
}

func (p *PostgresPhoneStore) Close() {
	if p.pool != nil {
		p.pool.Close()
	}
}

func (p *PostgresPhoneStore) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *PostgresPhoneStore) ListActive(ctx context.Context) ([]domain.Phone, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT serial, state, current_step, COALESCE(last_error,''), COALESCE(model,''),
		       COALESCE(android_version,''), COALESCE(screen_res_x,0), COALESCE(screen_res_y,0),
		       COALESCE(current_ip,''), proxy_id, COALESCE(wifi_ssid,''), COALESCE(adb_port,5555),
		       last_heartbeat, heartbeat_count, recovery_in_progress, COALESCE(last_error_hash,''),
		       created_at, updated_at, ready_at, retired_at
		FROM phones WHERE state NOT IN ('retired','error')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPhones(rows)
}

func (p *PostgresPhoneStore) ListAll(ctx context.Context) ([]domain.Phone, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT serial, state, current_step, COALESCE(last_error,''), COALESCE(model,''),
		       COALESCE(android_version,''), COALESCE(screen_res_x,0), COALESCE(screen_res_y,0),
		       COALESCE(current_ip,''), proxy_id, COALESCE(wifi_ssid,''), COALESCE(adb_port,5555),
		       last_heartbeat, heartbeat_count, recovery_in_progress, COALESCE(last_error_hash,''),
		       created_at, updated_at, ready_at, retired_at
		FROM phones ORDER BY serial`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPhones(rows)
}

func (p *PostgresPhoneStore) Get(ctx context.Context, serial string) (domain.Phone, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT serial, state, current_step, COALESCE(last_error,''), COALESCE(model,''),
		       COALESCE(android_version,''), COALESCE(screen_res_x,0), COALESCE(screen_res_y,0),
		       COALESCE(current_ip,''), proxy_id, COALESCE(wifi_ssid,''), COALESCE(adb_port,5555),
		       last_heartbeat, heartbeat_count, recovery_in_progress, COALESCE(last_error_hash,''),
		       created_at, updated_at, ready_at, retired_at
		FROM phones WHERE serial = $1`, serial)
	phone, err := scanPhone(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Phone{}, domain.ErrPhoneNotFound
	}
	return phone, err
}

func (p *PostgresPhoneStore) Save(ctx context.Context, phone domain.Phone) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO phones (serial, state, current_step, last_error, model, android_version,
			screen_res_x, screen_res_y, current_ip, proxy_id, wifi_ssid, adb_port,
			last_heartbeat, heartbeat_count, recovery_in_progress, last_error_hash,
			created_at, updated_at, ready_at, retired_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		phone.Serial, string(phone.State), phone.CurrentStep, nullText(phone.LastError),
		nullText(phone.Model), nullText(phone.AndroidVersion), nullInt(phone.ScreenResX), nullInt(phone.ScreenResY),
		nullText(phone.CurrentIP), phone.ProxyID, nullText(phone.WifiSSID), phone.AdbPort,
		phone.LastHeartbeat, phone.HeartbeatCount, phone.RecoveryInProgress, nullText(phone.LastErrorHash),
		phone.CreatedAt, phone.UpdatedAt, phone.ReadyAt, phone.RetiredAt,
	)
	return err
}

func (p *PostgresPhoneStore) Update(ctx context.Context, phone domain.Phone) error {
	phone.UpdatedAt = time.Now()
	ct, err := p.pool.Exec(ctx, `
		UPDATE phones SET state=$2, current_step=$3, last_error=$4, model=$5, android_version=$6,
			screen_res_x=$7, screen_res_y=$8, current_ip=$9, proxy_id=$10, wifi_ssid=$11, adb_port=$12,
			last_heartbeat=$13, heartbeat_count=$14, recovery_in_progress=$15, last_error_hash=$16,
			updated_at=$17, ready_at=$18, retired_at=$19
		WHERE serial=$1`,
		phone.Serial, string(phone.State), phone.CurrentStep, nullText(phone.LastError),
		nullText(phone.Model), nullText(phone.AndroidVersion), nullInt(phone.ScreenResX), nullInt(phone.ScreenResY),
		nullText(phone.CurrentIP), phone.ProxyID, nullText(phone.WifiSSID), phone.AdbPort,
		phone.LastHeartbeat, phone.HeartbeatCount, phone.RecoveryInProgress, nullText(phone.LastErrorHash),
		phone.UpdatedAt, phone.ReadyAt, phone.RetiredAt,
	)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrPhoneNotFound
	}
	return nil
}

func (p *PostgresPhoneStore) Delete(ctx context.Context, serial string) error {
	ct, err := p.pool.Exec(ctx, `DELETE FROM phones WHERE serial=$1`, serial)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrPhoneNotFound
	}
	return nil
}

func (p *PostgresPhoneStore) LogTransition(ctx context.Context, log domain.PhoneStateLog) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO phone_state_log (serial, from_state, to_state, step, error, duration_ms)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		log.Serial, string(log.FromState), string(log.ToState), log.Step, nullText(log.Error), nullInt(log.DurationMS),
	)
	return err
}

func (p *PostgresPhoneStore) Stats(ctx context.Context) (domain.PhoneStats, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT COUNT(*),
			COUNT(*) FILTER (WHERE state='working'),
			COUNT(*) FILTER (WHERE state='paused'),
			COUNT(*) FILTER (WHERE state='error'),
			COUNT(*) FILTER (WHERE state IN ('new','wifi_setup','proxy_setup','apps_install','auth','ready'))
		FROM phones WHERE state != 'retired'`)
	var s domain.PhoneStats
	if err := row.Scan(&s.Total, &s.Working, &s.Paused, &s.Error, &s.SettingUp); err != nil {
		return domain.PhoneStats{}, err
	}
	return s, nil
}

type pgxRow interface {
	Scan(dest ...any) error
}

func scanPhone(row pgxRow) (domain.Phone, error) {
	var phone domain.Phone
	var state string
	var lastError, model, android, ip, ssid, hash string
	var proxyID *int
	var lastHB, readyAt, retiredAt *time.Time
	err := row.Scan(
		&phone.Serial, &state, &phone.CurrentStep, &lastError, &model, &android,
		&phone.ScreenResX, &phone.ScreenResY, &ip, &proxyID, &ssid, &phone.AdbPort,
		&lastHB, &phone.HeartbeatCount, &phone.RecoveryInProgress, &hash,
		&phone.CreatedAt, &phone.UpdatedAt, &readyAt, &retiredAt,
	)
	if err != nil {
		return domain.Phone{}, err
	}
	phone.State = domain.PhoneState(state)
	phone.LastError = lastError
	phone.Model = model
	phone.AndroidVersion = android
	phone.CurrentIP = ip
	phone.ProxyID = proxyID
	phone.WifiSSID = ssid
	phone.LastErrorHash = hash
	phone.LastHeartbeat = lastHB
	phone.ReadyAt = readyAt
	phone.RetiredAt = retiredAt
	return phone, nil
}

func scanPhones(rows pgx.Rows) ([]domain.Phone, error) {
	var phones []domain.Phone
	for rows.Next() {
		phone, err := scanPhone(rows)
		if err != nil {
			return nil, err
		}
		phones = append(phones, phone)
	}
	return phones, rows.Err()
}

func nullText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}
