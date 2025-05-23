// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

package ipnauth

import (
	"context"
	"encoding/json"
	"fmt"

	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn"
	"tailscale.com/tailcfg"
)

// AuditLogFunc is any function that can be used to log audit actions performed by an [Actor].
type AuditLogFunc func(action tailcfg.ClientAuditAction, details string) error

// Actor is any actor using the [ipnlocal.LocalBackend].
//
// It typically represents a specific OS user, indicating that an operation
// is performed on behalf of this user, should be evaluated against their
// access rights, and performed in their security context when applicable.
type Actor interface {
	// UserID returns an OS-specific UID of the user represented by the receiver,
	// or "" if the actor does not represent a specific user on a multi-user system.
	// As of 2024-08-27, it is only used on Windows.
	UserID() ipn.WindowsUserID
	// Username returns the user name associated with the receiver,
	// or "" if the actor does not represent a specific user.
	Username() (string, error)
	// ClientID returns a non-zero ClientID and true if the actor represents
	// a connected LocalAPI client. Otherwise, it returns a zero value and false.
	ClientID() (_ ClientID, ok bool)

	// Context returns the context associated with the actor.
	// It carries additional information about the actor
	// and is canceled when the actor is done.
	Context() context.Context

	// CheckProfileAccess checks whether the actor has the necessary access rights
	// to perform a given action on the specified Tailscale profile.
	// It returns an error if access is denied.
	//
	// If the auditLogger is non-nil, it is used to write details about the action
	// to the audit log when required by the policy.
	CheckProfileAccess(profile ipn.LoginProfileView, requestedAccess ProfileAccess, auditLogFn AuditLogFunc) error

	// IsLocalSystem reports whether the actor is the Windows' Local System account.
	//
	// Deprecated: this method exists for compatibility with the current (as of 2024-08-27)
	// permission model and will be removed as we progress on tailscale/corp#18342.
	IsLocalSystem() bool

	// IsLocalAdmin reports whether the actor has administrative access to the
	// local machine, for whatever that means with respect to the current OS.
	//
	// The operatorUID is only used on Unix-like platforms and specifies the ID
	// of a local user (in the os/user.User.Uid string form) who is allowed to
	// operate tailscaled without being root or using sudo.
	//
	// Deprecated: this method exists for compatibility with the current (as of 2024-08-27)
	// permission model and will be removed as we progress on tailscale/corp#18342.
	IsLocalAdmin(operatorUID string) bool
}

// ActorCloser is an optional interface that might be implemented by an [Actor]
// that must be closed when done to release the resources.
type ActorCloser interface {
	// Close releases resources associated with the receiver.
	Close() error
}

// ClientID is an opaque, comparable value used to identify a connected LocalAPI
// client, such as a connected Tailscale GUI or CLI. It does not necessarily
// correspond to the same [net.Conn] or any physical session.
//
// Its zero value is valid, but does not represent a specific connected client.
type ClientID struct {
	v any
}

// NoClientID is the zero value of [ClientID].
var NoClientID ClientID

// ClientIDFrom returns a new [ClientID] derived from the specified value.
// ClientIDs derived from equal values are equal.
func ClientIDFrom[T comparable](v T) ClientID {
	return ClientID{v}
}

// String implements [fmt.Stringer].
func (id ClientID) String() string {
	if id.v == nil {
		return "(none)"
	}
	return fmt.Sprint(id.v)
}

// MarshalJSON implements [json.Marshaler].
// It is primarily used for testing.
func (id ClientID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.v)
}

// UnmarshalJSON implements [json.Unmarshaler].
// It is primarily used for testing.
func (id *ClientID) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &id.v)
}

type actorWithRequestReason struct {
	Actor
	ctx context.Context
}

// WithRequestReason returns an [Actor] that wraps the given actor and
// carries the specified request reason in its context.
func WithRequestReason(actor Actor, requestReason string) Actor {
	ctx := apitype.RequestReasonKey.WithValue(actor.Context(), requestReason)
	return &actorWithRequestReason{Actor: actor, ctx: ctx}
}

// Context implements [Actor].
func (a *actorWithRequestReason) Context() context.Context { return a.ctx }

type withoutCloseActor struct{ Actor }

// WithoutClose returns an [Actor] that does not expose the [ActorCloser] interface.
// In other words, _, ok := WithoutClose(actor).(ActorCloser) will always be false,
// even if the original actor implements [ActorCloser].
func WithoutClose(actor Actor) Actor {
	return withoutCloseActor{actor}
}
