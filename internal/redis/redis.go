package redis

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/carlmjohnson/errutil"
	"github.com/go-redsync/redsync"
	"github.com/gomodule/redigo/redis"
)

var ErrNil = redis.ErrNil

type Store struct {
	rp *redis.Pool
	l  Logger
}

// New creates a Store and pings it.
func New(d Dialer, l Logger) (*Store, error) {
	c := Store{
		rp: &redis.Pool{
			MaxIdle:     3,
			IdleTimeout: 4 * time.Minute,
			Dial:        d,
		},
		l: l,
	}

	if err := c.Ping(); err != nil {
		c.printf("connecting to Redis: %v", err)
		return nil, err
	}
	return &c, nil
}

func (rs *Store) printf(format string, v ...interface{}) {
	if rs.l != nil {
		rs.l.Printf(format, v...)
	}
}

// Ping Redis
func (rs *Store) Ping() (err error) {
	rs.printf("do Redis PING")
	conn := rs.rp.Get()
	defer errutil.Defer(&err, conn.Close)

	_, err = conn.Do("PING")
	return err
}

// Get calls GET in Redis and converts values from JSON bytes
func (rs *Store) Get(key string, getv interface{}) (err error) {
	rs.printf("do Redis GET %q", key)
	conn := rs.rp.Get()
	defer errutil.Defer(&err, conn.Close)

	getb, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		return err
	}
	return json.Unmarshal(getb, getv)
}

// Set converts values to JSON bytes and calls SET in Redis
func (rs *Store) Set(key string, setv interface{}) (err error) {
	rs.printf("do Redis SET %q", key)
	conn := rs.rp.Get()
	defer errutil.Defer(&err, conn.Close)

	setb, err := json.Marshal(setv)
	if err != nil {
		return err
	}

	_, err = conn.Do("SET", key, setb)
	return err
}

// GetSet converts values to JSON bytes and calls GETSET in Redis
func (rs *Store) GetSet(key string, getv, setv interface{}) (err error) {
	rs.printf("do Redis GETSET %q", key)
	conn := rs.rp.Get()
	defer errutil.Defer(&err, conn.Close)

	setb, err := json.Marshal(setv)
	if err != nil {
		return err
	}
	getb, err := redis.Bytes(conn.Do("GETSET", key, setb))
	if err != nil {
		return err
	}
	return json.Unmarshal(getb, getv)
}

func (rs *Store) GetLock(key string) (unlock func(), err error) {
	rs.printf("get Redis lock %q", key)
	lock := redsync.
		New([]redsync.Pool{rs.rp}).
		NewMutex(key)
	if err := lock.Lock(); err != nil {
		return nil, err
	}
	return func() {
		rs.printf("return Redis lock %q", key)
		lock.Unlock()
	}, nil
}

// Parse creates a Dialer by parsing the connection URL.
//
// URLs should have the format redis://user:password@host:port/db
// where db is an integer and username is ignored.
func Parse(connURL string) (Dialer, error) {
	u, err := url.Parse(connURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "redis" {
		return nil, fmt.Errorf("invalid redis URL scheme: %q", u.Scheme)
	}
	password, _ := u.User.Password()
	db := 0
	if path := strings.TrimPrefix(u.Path, "/"); path != "" {
		if db, err = strconv.Atoi(path); err != nil {
			return nil, err
		}
	}
	return func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", u.Host)
		if err != nil {
			return nil, err
		}
		if password != "" {
			if _, err := c.Do("AUTH", password); err != nil {
				c.Close()
				return nil, err
			}
		}
		if db != 0 {
			if _, err := c.Do("SELECT", db); err != nil {
				c.Close()
				return nil, err
			}
		}
		return c, nil
	}, nil
}
