// Copyright (c) 2012-2016 The Chango Framework Authors, All rights reserved.
// Chango Framework source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package cache

import (
	"strings"
	"time"

	"github.com/giovan/chango"
)

var cacheLog = chango.ChangoLog.New("section", "cache")

func init() {
	chango.OnAppStart(func() {
		// Set the default expiration time.
		defaultExpiration := time.Hour // The default for the default is one hour.
		if expireStr, found := chango.Config.String("cache.expires"); found {
			var err error
			if defaultExpiration, err = time.ParseDuration(expireStr); err != nil {
				cacheLog.Panic("Could not parse default cache expiration duration " + expireStr + ": " + err.Error())
			}
		}

		// make sure you aren't trying to use both memcached and redis
		if chango.Config.BoolDefault("cache.memcached", false) && chango.Config.BoolDefault("cache.redis", false) {
			cacheLog.Panic("You've configured both memcached and redis, please only include configuration for one cache!")
		}

		// Use memcached?
		if chango.Config.BoolDefault("cache.memcached", false) {
			hosts := strings.Split(chango.Config.StringDefault("cache.hosts", ""), ",")
			if len(hosts) == 0 {
				cacheLog.Panic("Memcache enabled but no memcached hosts specified!")
			}

			Instance = NewMemcachedCache(hosts, defaultExpiration)
			return
		}

		// Use Redis (share same config as memcached)?
		if chango.Config.BoolDefault("cache.redis", false) {
			hosts := strings.Split(chango.Config.StringDefault("cache.hosts", ""), ",")
			if len(hosts) == 0 {
				cacheLog.Panic("Redis enabled but no Redis hosts specified!")
			}
			if len(hosts) > 1 {
				cacheLog.Panic("Redis currently only supports one host!")
			}
			password := chango.Config.StringDefault("cache.redis.password", "")
			Instance = NewRedisCache(hosts[0], password, defaultExpiration)
			return
		}

		// By default, use the in-memory cache.
		Instance = NewInMemoryCache(defaultExpiration)
	})
}
