package testutils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	InvoicePaymentSucceeded = "invoice.payment_succeeded"
	CustomerUpdated         = "customer.updated"
)

func HasStripe() bool {
	_, err := exec.LookPath("stripe")
	return err == nil
}

type StripeListener struct {
	ApiKey string
}

// MustLaunchStripe starts the stripe command it returns a function the must be called to stop
// the process
func (s *StripeListener) MustLaunchStripe(address string) func() error {
	cmd := exec.Command("stripe", "listen", "--forward-to", address, "--api-key", s.ApiKey)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		panic(fmt.Sprintf("Failed to start stripe: %s", err))
	}
	return func() error {
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			return fmt.Errorf("Failed to send signal: %w", err)
		}
		return cmd.Wait()
	}
}

func (s *StripeListener) TriggerEvent(event string, args ...string) error {
	allArgs := append([]string{"trigger", event, "--api-key", s.ApiKey}, args...)
	cmd := exec.Command("stripe", allArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("Failed to trigger event %s: %w", event, err)
	}
	return cmd.Wait()
}

func (s *StripeListener) SignSecret() (string, error) {
	var out bytes.Buffer
	cmd := exec.Command("stripe", "listen", "--api-key", s.ApiKey, "--print-secret")
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("Failed to start sign secret command: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("Failed during sign secret command: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}
