package middleware

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/gomodule/redigo/redis"
	"github.com/rerost/playground/infra/cache"
	"github.com/rerost/playground/infra/store"
)

type Middleware struct {
	DB    store.Store
	Cache cache.GobCache
}

func MiddlewareForGAE(ctx context.Context, pid string) (Middleware, error) {
	c, err := datastore.NewClient(ctx, pid)
	if err != nil {
		return Middleware{}, fmt.Errorf("could not create cloud datastore client: %v", err)
	}

	var memcacheClient *memcache.Client
	if caddr := os.Getenv("MEMCACHED_ADDR"); caddr != "" {
		memcacheClient = memcache.New(caddr)
	}

	if memcacheClient != nil {
		log.Printf("App (project ID: %q) is caching results", pid)
	} else {
		log.Printf("App (project ID: %q) is NOT caching results", pid)
	}

	return Middleware{
		DB:    store.NewClienG(c),
		Cache: cache.NewGobCacheM(memcacheClient),
	}, nil
}

func MiddlewareForDevelopment(_ context.Context) (Middleware, error) {
	var memcacheClient *memcache.Client
	if caddr := os.Getenv("MEMCACHED_ADDR"); caddr != "" {
		memcacheClient = memcache.New(caddr)
	}

	return Middleware{
		DB:    store.NewClientInMem(),
		Cache: cache.NewGobCacheM(memcacheClient),
	}, nil
}

// MiddlewareForRedis
// url Like "redis://localhost:6379"
func MiddlewareForRedis(ctx context.Context, url string) (Middleware, error) {
	pool := &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.DialURL(url) },
	}

	return Middleware{
		DB:    store.NewClientRedis(pool),
		Cache: cache.NewGobCacheR(pool),
	}, nil
}
