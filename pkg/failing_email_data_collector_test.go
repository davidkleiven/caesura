package pkg

import (
	"context"
	"errors"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
)

func TestFailingEmailDataCollector(t *testing.T) {
	collector := FailingEmailDataCollector{}

	ctx := context.Background()
	for _, fn := range []func() error{
		func() error {
			_, err := collector.GetUsersInOrg(ctx, "orgId")
			return err
		},
		func() error {
			_, err := collector.Item(ctx, "path")
			return err
		},
		func() error {
			iter := collector.Resource(ctx, "org", "path")
			num := 0
			for _ = range iter {
				num++
			}
			if num != 0 {
				return errors.New("sould be empty")
			}
			return nil
		},
		func() error {
			_, err := collector.ResourceItemNames(ctx, "resource-id")
			return err
		},
		func() error {
			_, err := collector.MetaById(ctx, "orgId", "meta")
			return err
		},
	} {
		testutils.AssertNil(t, fn())
	}
}
