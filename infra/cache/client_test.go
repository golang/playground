package cache_test

import (
	"testing"

	"github.com/rerost/playground/infra/cache"
)

func TestNilClient(t *testing.T) {
	client := cache.NewGobCacheM(nil)
	err := client.Set("test:nil", nil)
	if err != nil {
		t.Error(err)
		return
	}

	err = client.Get("test:nil", nil)
	if err != client.ErrCacheMiss() {
		t.Error(err)
		return
	}
}
