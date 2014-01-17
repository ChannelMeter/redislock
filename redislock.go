/*
Package redislock implements a pessimistic lock using Redis.

For example, lock and unlock a user using its ID as a resource identifier:
	lock, ok, err := TryLock(conn, "user:123")
	
	if !ok {
		// User is in use - return to avoid duplicate work, race conditions, etc.
		return
	}
	
	if err != nil {
		log.Fatal("Error while attempting lock")
	}
	
	// Do something with the user.
	
	// When you're done, unlock the user.
	lock.Unlock()
*/
package redislock

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"time"
)

const lockTimeout = 10 * time.Minute

var unlockScript = redis.NewScript(1, `
	if redis.call("get", KEYS[1]) == ARGV[1]
	then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
`)

// Lock represents a held lock.
type Lock struct {
	resource string
	token    string
	conn     redis.Conn
}

func (lock *Lock) tryLock() (ok bool, err error) {
	status, err := redis.String(lock.conn.Do("SET", lock.key(), lock.token, "EX", int64(lockTimeout/time.Second), "NX"))

	if err == redis.ErrNil {
		// The lock was not successful, it already exists.
		err = nil
	} else if err == nil && status == "OK" {
		// The lock was successful.
		ok = true
	}

	return
}

// Unlock releases the lock. If the lock has timed out, it silently fails without error.
func (lock *Lock) Unlock() (err error) {
	_, err = unlockScript.Do(lock.conn, lock.key(), lock.token)
	return
}

func (lock *Lock) key() string {
	return fmt.Sprintf("redislock:%s", lock.resource)
}

// TryLock attempts to acquire a lock on the given resource in a non-blocking manner.
func TryLock(conn redis.Conn, resource string) (lock *Lock, ok bool, err error) {
	lock = &Lock{resource, uuid.New(), conn}

	ok, err = lock.tryLock()

	if !ok || err != nil {
		lock = nil
	}

	return
}