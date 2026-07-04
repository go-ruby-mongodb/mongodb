// Copyright (c) the go-ruby-mongodb/mongodb authors
//
// SPDX-License-Identifier: BSD-3-Clause

package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/xoptions"
)

// Client is Mongo::Client: a connection to a MongoDB deployment. Like the gem it
// is created once and shared; the underlying driver pools connections lazily, so
// NewClient does not dial and needs no live server.
type Client struct {
	inner     *mongo.Client
	defaultDB string
}

// clientConfig accumulates the keyword options of Mongo::Client.new.
type clientConfig struct {
	database   string
	appName    string
	deployment driver.Deployment // test seam; nil in production
}

// Option configures a Client, mirroring a Mongo::Client.new keyword argument.
type Option func(*clientConfig)

// WithDatabase sets the default database, the gem's `database:` option.
// Client.Database with no name (the empty string) then resolves to it.
func WithDatabase(name string) Option {
	return func(c *clientConfig) { c.database = name }
}

// AppName sets the application name reported to the server, the gem's `app_name:`.
func AppName(name string) Option {
	return func(c *clientConfig) { c.appName = name }
}

// withDeployment injects a driver.Deployment (e.g. a mock) so the deterministic
// suite can exercise real operations in-process. It is unexported: production
// callers cannot reach it, and the same-package tests use it to stay
// server-free.
func withDeployment(d driver.Deployment) Option {
	return func(c *clientConfig) { c.deployment = d }
}

// NewClient connects to MongoDB, mirroring Mongo::Client.new(uri, options).
// The uri is a standard mongodb:// or mongodb+srv:// connection string. It
// returns a *mongodb.Error (class Mongo::Error::*) on failure.
func NewClient(uri string, opts ...Option) (*Client, error) {
	cfg := &clientConfig{}
	for _, o := range opts {
		o(cfg)
	}

	co := options.Client().ApplyURI(uri)
	if cfg.appName != "" {
		co.SetAppName(cfg.appName)
	}
	if cfg.deployment != nil {
		// The only error SetInternalClientOptions can return for the "deployment"
		// key is a type mismatch, which cannot happen here (cfg.deployment is a
		// driver.Deployment), so the error is intentionally discarded.
		_ = xoptions.SetInternalClientOptions(co, "deployment", cfg.deployment)
	}

	inner, err := mongo.Connect(co)
	if err != nil {
		return nil, mapError(err)
	}
	return &Client{inner: inner, defaultDB: cfg.database}, nil
}

// Database returns a handle to a database, the gem's Mongo::Client#database /
// #use. An empty name resolves to the client's default database (the `database:`
// option).
func (c *Client) Database(name string) *Database {
	if name == "" {
		name = c.defaultDB
	}
	return &Database{inner: c.inner.Database(name)}
}

// Collection returns a collection in the client's default database, the gem's
// Mongo::Client#[].
func (c *Client) Collection(name string) *Collection {
	return c.Database("").Collection(name)
}

// Close disconnects the client and releases pooled connections, the gem's
// Mongo::Client#close.
func (c *Client) Close() error {
	return mapError(c.inner.Disconnect(context.Background()))
}

// Ping round-trips a ping command to confirm the deployment is reachable, the
// gem's Mongo::Client health check. It requires a live server.
func (c *Client) Ping() error {
	return mapError(c.inner.Ping(context.Background(), nil))
}

// Database is Mongo::Database: a handle to a named database on a Client.
type Database struct {
	inner *mongo.Database
}

// Name returns the database name, the gem's Mongo::Database#name.
func (d *Database) Name() string { return d.inner.Name() }

// Collection returns a collection handle, the gem's Mongo::Database#collection /
// #[].
func (d *Database) Collection(name string) *Collection {
	return &Collection{inner: d.inner.Collection(name)}
}

// Drop drops the database, the gem's Mongo::Database#drop. It requires a live
// server.
func (d *Database) Drop() error {
	return mapError(d.inner.Drop(context.Background()))
}
