package msa

import (
	"context"
	_ "embed"
	"log/slog"
	"sync"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
)

//go:embed __msa_client.txt
var msaClientID string

type Session interface {
	StartLogin() error
	DeviceCode() *public.DeviceCode
	AuthResult() (*public.AuthResult, error)
	impl()
}

func NewSession() (Session, error) {
	client, err := public.New(msaClientID, public.WithAuthority("https://login.microsoftonline.com/consumers"))
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
	deviceCode, err := s.client.AcquireTokenByDeviceCode(context.Background(), []string{"XboxLive.signin"})
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
		slog.Info("Login successful", "result", result)
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
