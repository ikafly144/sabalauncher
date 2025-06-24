package msa

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
	"github.com/ikafly144/sabalauncher/secret"
)

var msaClientID = secret.GetSecret("MSA_CLIENT_ID")

type Session interface {
	StartLogin() error
	DeviceCode() *public.DeviceCode
	AuthResult() (*public.AuthResult, error)
	impl()
}

var _ cache.ExportReplace = (*CacheAccessor)(nil)

func NewCacheAccessor(path string) (*CacheAccessor, error) {
	return &CacheAccessor{path: path}, nil
}

type CacheAccessor struct {
	path string
}

// Export writes the binary representation of the cache (cache.Marshal()) to external storage.
// This is considered opaque. Context cancellations should be honored as in Replace.
func (c *CacheAccessor) Export(ctx context.Context, cache cache.Marshaler, hints cache.ExportHints) error {
	if c.path == "" {
		return fmt.Errorf("cache path is not set")
	}
	slog.Info("Exporting cache", "path", c.path, "partitionKey", hints.PartitionKey)
	data, err := cache.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	_ = os.MkdirAll(filepath.Dir(c.path), 0755)
	file, err := os.Create(c.path)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write cache data: %w", err)
	}
	slog.Info("Cache exported", "path", c.path, "partitionKey", hints.PartitionKey)

	return nil
}

// Replace replaces the cache with what is in external storage. Implementors should honor
// Context cancellations and return context.Canceled or context.DeadlineExceeded in those cases.
func (c *CacheAccessor) Replace(ctx context.Context, cache cache.Unmarshaler, hints cache.ReplaceHints) error {
	if c.path == "" {
		return fmt.Errorf("cache path is not set")
	}

	_ = os.MkdirAll(filepath.Dir(c.path), 0755)
	file, err := os.Open(c.path)
	if err != nil && os.IsNotExist(err) {
		slog.Warn("Cache file does not exist, creating new one", "path", c.path, "partitionKey", hints.PartitionKey)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to open cache file: %w", err)
	}
	defer file.Close()
	slog.Info("Replacing cache", "path", c.path, "partitionKey", hints.PartitionKey)
	bytes, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read cache file: %w", err)
	}
	if err := cache.Unmarshal(bytes); err != nil {
		return fmt.Errorf("failed to unmarshal cache data: %w", err)
	}
	return nil
}

func NewSession(c *CacheAccessor) (Session, error) {
	client, err := public.New(msaClientID, public.WithAuthority("https://login.microsoftonline.com/consumers"), public.WithCache(c))
	if err != nil {
		return nil, err
	}
	return &session{
		client: client,
	}, nil
}

var _ Session = (*session)(nil)

type session struct {
	client      public.Client
	result      *public.AuthResult
	resultError error
	deviceCode  *public.DeviceCode

	done sync.WaitGroup
}

func (s *session) impl() {}

func (s *session) StartLogin() error {
	deviceCode, err := s.client.AcquireTokenByDeviceCode(context.Background(), []string{"XboxLive.signin", "XboxLive.offline_access"})
	if err != nil {
		return err
	}
	s.deviceCode = &deviceCode

	s.done.Add(1)
	go func() {
		defer s.done.Done()
		result, err := deviceCode.AuthenticationResult(context.Background())
		if err != nil {
			s.result = nil
			s.resultError = err
			return
		}
		accounts, err := s.client.Accounts(context.Background())
		if err != nil {
			s.result = nil
			s.resultError = fmt.Errorf("failed to get accounts: %w", err)
		} else if len(accounts) == 0 {
			s.result = nil
			s.resultError = fmt.Errorf("no accounts found after login")
		}
		if len(accounts) > 1 {
			for _, account := range accounts {
				if account.Key() != result.Account.Key() {
					slog.Info("Multiple accounts found", "account", account)
					if err := s.client.RemoveAccount(context.Background(), account); err != nil {
						slog.Error("Failed to remove account", "account", account, "error", err)
					}
					break
				}
			}
		}
		slog.Info("Login successful", "result", result)
		if len(result.DeclinedScopes) > 0 {
			slog.Warn("Login with declined scopes", "declinedScopes", result.DeclinedScopes)
		}
		if !slices.Contains(result.GrantedScopes, "XboxLive.signin") || !slices.Contains(result.GrantedScopes, "XboxLive.offline_access") {
			s.result = nil
			s.resultError = fmt.Errorf("missing scopes: %v", result.GrantedScopes)
		}
		s.result = &result
		s.resultError = nil
	}()
	return nil
}

func (s *session) DeviceCode() *public.DeviceCode {
	return s.deviceCode
}

func (s *session) AuthResult() (*public.AuthResult, error) {
	s.done.Wait()
	return s.result, s.resultError
}
