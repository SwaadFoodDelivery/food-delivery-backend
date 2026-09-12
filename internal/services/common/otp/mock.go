package otp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"food-delivery-backend/pkg/utils"

	"github.com/rs/zerolog"
)

type MockProvider struct {
	log    zerolog.Logger
	outbox string
}

// No HTTP code-retrieval endpoint or predictable OTP: this records only the
// mock transport's random delivery in a pre-created private local directory.
func NewMockProviderWithOutbox(log zerolog.Logger, dir, env string) (*MockProvider, error) {
	m := NewMockProvider(log)
	if dir == "" {
		return m, nil
	}
	if env != "development" && env != "test" {
		return nil, fmt.Errorf("OTP outbox is development/test only")
	}
	info, err := os.Lstat(dir)
	if err != nil || !filepath.IsAbs(dir) || !info.IsDir() || info.Mode().Perm() != 0700 {
		return nil, fmt.Errorf("OTP outbox requires an existing absolute private 0700 directory")
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve OTP outbox: %w", err)
	}
	m.outbox = resolved
	return m, nil
}

func NewMockProvider(log zerolog.Logger) *MockProvider {
	return &MockProvider{log: log}
}

func (m *MockProvider) Send(ctx context.Context, phone string, code string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.outbox != "" {
		hash := sha256.Sum256([]byte(phone))
		file, err := os.CreateTemp(m.outbox, ".delivery-*")
		if err != nil {
			return fmt.Errorf("open mock OTP delivery: %w", err)
		}
		name := file.Name()
		defer os.Remove(name)
		err = json.NewEncoder(file).Encode(struct {
			PhoneHash string    `json:"phone_hash"`
			Code      string    `json:"code"`
			SentAt    time.Time `json:"sent_at"`
		}{hex.EncodeToString(hash[:]), code, time.Now().UTC()})
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		return os.Rename(name, name+".json")
	}
	ctxLog := zerolog.Ctx(ctx)
	if ctxLog == nil || ctxLog.GetLevel() == zerolog.Disabled {
		ctxLog = &m.log
	}
	ctxLog.Info().Str("phone", utils.MaskPhone(phone)).Str("otp", code).Msg("auth.otp.mock_sent")
	return nil
}
